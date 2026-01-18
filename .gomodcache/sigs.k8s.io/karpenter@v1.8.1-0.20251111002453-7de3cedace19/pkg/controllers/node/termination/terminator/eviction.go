/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package terminator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	terminatorevents "sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator/events"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	utilscontroller "sigs.k8s.io/karpenter/pkg/utils/controller"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	podutils "sigs.k8s.io/karpenter/pkg/utils/pod"
)

const (
	evictionQueueBaseDelay = 100 * time.Millisecond
	evictionQueueMaxDelay  = 10 * time.Second
	minReconciles          = 100
	maxReconciles          = 5000

	multiplePodDisruptionBudgetsError = "This pod has more than one PodDisruptionBudget, which the eviction subresource does not support."
)

type NodeDrainError struct {
	error
}

func NewNodeDrainError(err error) *NodeDrainError {
	return &NodeDrainError{error: err}
}

func IsNodeDrainError(err error) bool {
	if err == nil {
		return false
	}
	var nodeDrainErr *NodeDrainError
	return errors.As(err, &nodeDrainErr)
}

type QueueKey struct {
	types.NamespacedName
	UID types.UID
}

func NewQueueKey(pod *corev1.Pod) QueueKey {
	return QueueKey{
		NamespacedName: client.ObjectKeyFromObject(pod),
		UID:            pod.UID,
	}
}

type Queue struct {
	sync.Mutex

	source chan event.TypedGenericEvent[*corev1.Pod]
	set    sets.Set[QueueKey]

	kubeClient client.Client
	recorder   events.Recorder
}

func NewQueue(kubeClient client.Client, recorder events.Recorder) *Queue {
	return &Queue{
		source:     make(chan event.TypedGenericEvent[*corev1.Pod], 10000),
		set:        sets.New[QueueKey](),
		kubeClient: kubeClient,
		recorder:   recorder,
	}
}

func (q *Queue) Name() string {
	return "eviction-queue"
}

func (q *Queue) Register(ctx context.Context, m manager.Manager) error {
	maxConcurrentReconciles := utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), minReconciles, maxReconciles)
	qps, bucketSize := utilscontroller.GetTypedBucketConfigs(100, minReconciles, maxConcurrentReconciles)
	return controllerruntime.NewControllerManagedBy(m).
		Named(q.Name()).
		WatchesRawSource(source.Channel(q.source, handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
			GenericFunc: func(_ context.Context, e event.TypedGenericEvent[*corev1.Pod], queue workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				queue.Add(reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(e.Object),
				})
			},
		})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter[reconcile.Request](
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](evictionQueueBaseDelay, evictionQueueMaxDelay),
				// qps scales linearly with concurrentReconciles, bucket size is 10 * qps
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(qps), bucketSize)},
			),
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), q))
}

// Add adds pods to the Queue
func (q *Queue) Add(pods ...*corev1.Pod) {
	q.Lock()
	defer q.Unlock()

	for _, pod := range pods {
		qk := NewQueueKey(pod)
		if !q.set.Has(qk) {
			q.set.Insert(qk)
			q.source <- event.TypedGenericEvent[*corev1.Pod]{Object: pod}
		}
	}
}

func (q *Queue) Has(pod *corev1.Pod) bool {
	q.Lock()
	defer q.Unlock()

	return q.set.Has(NewQueueKey(pod))
}

func (q *Queue) Reconcile(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, q.Name())

	if !q.Has(pod) {
		//This is a different pod than the one the queue, we should exit without evicting
		//This race happens when a pod is replaced with one that has the same namespace and name
		//but a different UID after the original pod is added to the queue but before the
		//controller can reconcile on it
		return reconcile.Result{}, nil
	}
	// Evict the pod
	if err := q.kubeClient.SubResource("eviction").Create(ctx,
		pod,
		&policyv1.Eviction{
			DeleteOptions: &metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: lo.ToPtr(pod.UID),
				},
			},
		}); err != nil {
		var apiStatus apierrors.APIStatus
		var message string
		if errors.As(err, &apiStatus) {
			code := apiStatus.Status().Code
			message = apiStatus.Status().Message
			NodesEvictionRequestsTotal.Inc(map[string]string{CodeLabel: fmt.Sprint(code)})
		}
		// status codes for the eviction API are defined here:
		// https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works
		if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
			// 404 - The pod no longer exists
			// https://github.com/kubernetes/kubernetes/blob/ad19beaa83363de89a7772f4d5af393b85ce5e61/pkg/registry/core/pod/storage/eviction.go#L160
			// 409 - The pod exists, but it is not the same pod that we initiated the eviction on
			// https://github.com/kubernetes/kubernetes/blob/ad19beaa83363de89a7772f4d5af393b85ce5e61/pkg/registry/core/pod/storage/eviction.go#L318
			return reconcile.Result{}, nil
		}
		// The pod exists and is the same pod, we need to continue
		// 429 - PDB violation
		// Regardless of whether the PDBs allow disruptions, Kubernetes doesn't support multiple PDBs on a single pod:
		// https://github.com/kubernetes/kubernetes/blob/84cacae7046df93c1f6f8ea97c912d948e1ad06a/pkg/registry/core/pod/storage/eviction.go#L226
		if apierrors.IsTooManyRequests(err) || message == multiplePodDisruptionBudgetsError {
			node, err2 := podutils.NodeForPod(ctx, q.kubeClient, pod)
			if err2 != nil {
				return reconcile.Result{}, err2
			}
			errorMessage := lo.Ternary(message == multiplePodDisruptionBudgetsError, "eviction does not support multiple PDBs", "evicting pod violates a PDB")
			q.recorder.Publish(terminatorevents.NodeFailedToDrain(node, serrors.Wrap(errors.New(errorMessage), "Pod", klog.KRef(pod.Namespace, pod.Name))))
			return reconcile.Result{Requeue: true}, nil
		}
		// Its not a PDB, we should requeue
		return reconcile.Result{}, err
	}
	NodesEvictionRequestsTotal.Inc(map[string]string{CodeLabel: "200"})
	reason := evictionReason(ctx, pod, q.kubeClient)
	q.recorder.Publish(terminatorevents.EvictPod(pod, reason))
	PodsDrainedTotal.Inc(map[string]string{ReasonLabel: reason})

	q.Lock()
	defer q.Unlock()
	q.set.Delete(NewQueueKey(pod))
	return reconcile.Result{}, nil
}

func evictionReason(ctx context.Context, pod *corev1.Pod, kubeClient client.Client) string {
	node, err := podutils.NodeForPod(ctx, kubeClient, pod)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "pod has no node, failed looking up pod eviction reason")
		return ""
	}
	nodeClaim, err := nodeutils.NodeClaimForNode(ctx, kubeClient, node)
	if err != nil {
		log.FromContext(ctx).V(1).Error(err, "node has no nodeclaim, failed looking up pod eviction reason")
		return ""
	}
	if cond := nodeClaim.StatusConditions().Get(v1.ConditionTypeDisruptionReason); cond.IsTrue() {
		return cond.Reason
	}
	return "Forceful Termination"
}
