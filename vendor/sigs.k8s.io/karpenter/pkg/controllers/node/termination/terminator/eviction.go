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

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	terminatorevents "sigs.k8s.io/karpenter/pkg/controllers/node/termination/terminator/events"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/utils/node"
)

const (
	evictionQueueBaseDelay = 100 * time.Millisecond
	evictionQueueMaxDelay  = 10 * time.Second
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
	UID        types.UID
	providerID string
}

func NewQueueKey(pod *corev1.Pod, providerID string) QueueKey {
	return QueueKey{
		NamespacedName: client.ObjectKeyFromObject(pod),
		UID:            pod.UID,
		providerID:     providerID,
	}
}

type Queue struct {
	workqueue.TypedRateLimitingInterface[QueueKey]

	mu  sync.Mutex
	set sets.Set[QueueKey]

	kubeClient client.Client
	recorder   events.Recorder
}

func NewQueue(kubeClient client.Client, recorder events.Recorder) *Queue {
	return &Queue{
		TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig[QueueKey](
			workqueue.NewTypedItemExponentialFailureRateLimiter[QueueKey](evictionQueueBaseDelay, evictionQueueMaxDelay),
			workqueue.TypedRateLimitingQueueConfig[QueueKey]{
				Name: "eviction.workqueue",
			}),
		set:        sets.New[QueueKey](),
		kubeClient: kubeClient,
		recorder:   recorder,
	}
}

func NewTestingQueue(kubeClient client.Client, recorder events.Recorder) *Queue {
	return &Queue{
		TypedRateLimitingInterface: &controllertest.TypedQueue[QueueKey]{TypedInterface: workqueue.NewTypedWithConfig(workqueue.TypedQueueConfig[QueueKey]{Name: "eviction.workqueue"})},
		set:                        sets.New[QueueKey](),
		kubeClient:                 kubeClient,
		recorder:                   recorder,
	}
}

func (q *Queue) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("eviction-queue").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(q))
}

// Add adds pods to the Queue
func (q *Queue) Add(node *corev1.Node, pods ...*corev1.Pod) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, pod := range pods {
		qk := NewQueueKey(pod, node.Spec.ProviderID)
		if !q.set.Has(qk) {
			q.set.Insert(qk)
			q.TypedRateLimitingInterface.Add(qk)
		}
	}
}

func (q *Queue) Has(node *corev1.Node, pod *corev1.Pod) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.set.Has(NewQueueKey(pod, node.Spec.ProviderID))
}

func (q *Queue) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "eviction-queue")
	// Check if the queue is empty. client-go recommends not using this function to gate the subsequent
	// get call, but since we're popping items off the queue synchronously, there should be no synchonization
	// issues.
	if q.TypedRateLimitingInterface.Len() == 0 {
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	// Get pod from queue. This waits until queue is non-empty.
	item, shutdown := q.TypedRateLimitingInterface.Get()
	if shutdown {
		return reconcile.Result{}, fmt.Errorf("EvictionQueue is broken and has shutdown")
	}

	defer q.TypedRateLimitingInterface.Done(item)

	// Evict the pod
	if q.Evict(ctx, item) {
		q.TypedRateLimitingInterface.Forget(item)
		q.mu.Lock()
		q.set.Delete(item)
		q.mu.Unlock()
		return reconcile.Result{RequeueAfter: singleton.RequeueImmediately}, nil
	}

	// Requeue pod if eviction failed
	q.TypedRateLimitingInterface.AddRateLimited(item)
	return reconcile.Result{RequeueAfter: singleton.RequeueImmediately}, nil
}

// Evict returns true if successful eviction call, and false if there was an eviction-related error
func (q *Queue) Evict(ctx context.Context, key QueueKey) bool {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Pod", klog.KRef(key.Namespace, key.Name)))
	evictionMessage, err := evictionReason(ctx, key, q.kubeClient)
	if err != nil {
		// XXX(cmcavoy): this should be unreachable, but we log it if it happens
		log.FromContext(ctx).V(1).Error(err, "failed looking up pod eviction reason")
	}
	if err := q.kubeClient.SubResource("eviction").Create(ctx,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}},
		&policyv1.Eviction{
			DeleteOptions: &metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: lo.ToPtr(key.UID),
				},
			},
		}); err != nil {
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			code := apiStatus.Status().Code
			NodesEvictionRequestsTotal.Inc(map[string]string{CodeLabel: fmt.Sprint(code)})
		}
		// status codes for the eviction API are defined here:
		// https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works
		if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
			// 404 - The pod no longer exists
			// https://github.com/kubernetes/kubernetes/blob/ad19beaa83363de89a7772f4d5af393b85ce5e61/pkg/registry/core/pod/storage/eviction.go#L160
			// 409 - The pod exists, but it is not the same pod that we initiated the eviction on
			// https://github.com/kubernetes/kubernetes/blob/ad19beaa83363de89a7772f4d5af393b85ce5e61/pkg/registry/core/pod/storage/eviction.go#L318
			return true
		}
		if apierrors.IsTooManyRequests(err) { // 429 - PDB violation
			q.recorder.Publish(terminatorevents.NodeFailedToDrain(&corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			}}, fmt.Errorf("evicting pod %s/%s violates a PDB", key.Namespace, key.Name)))
			return false
		}
		log.FromContext(ctx).Error(err, "failed evicting pod")
		return false
	}
	NodesEvictionRequestsTotal.Inc(map[string]string{CodeLabel: "200"})
	q.recorder.Publish(terminatorevents.EvictPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}, evictionMessage))
	return true
}

func evictionReason(ctx context.Context, key QueueKey, kubeClient client.Client) (string, error) {
	nodeClaim, err := node.NodeClaimForNode(ctx, kubeClient, &corev1.Node{Spec: corev1.NodeSpec{ProviderID: key.providerID}})
	if err != nil {
		return "", err
	}
	terminationCondition := nodeClaim.StatusConditions().Get(v1.ConditionTypeDisruptionReason)
	if terminationCondition.IsTrue() {
		return terminationCondition.Message, nil
	}
	return "Forceful Termination", nil
}
