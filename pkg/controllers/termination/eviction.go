/*
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

package termination

import (
	"context"
	"time"

	set "github.com/deckarep/golang-set"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/utils/injection"
)

const (
	evictionQueueBaseDelay = 100 * time.Millisecond
	evictionQueueMaxDelay  = 10 * time.Second
)

type EvictionQueue struct {
	workqueue.RateLimitingInterface
	set.Set

	coreV1Client corev1.CoreV1Interface
}

func NewEvictionQueue(ctx context.Context) *EvictionQueue {
	queue := &EvictionQueue{
		RateLimitingInterface: workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(evictionQueueBaseDelay, evictionQueueMaxDelay)),
		Set:                   set.NewSet(),

		coreV1Client: injection.GetKubernetesInterface(ctx).CoreV1(),
	}
	go queue.Start(logging.WithLogger(ctx, logging.FromContext(ctx).Named("eviction")))
	return queue
}

// Add adds pods to the EvictionQueue
func (e *EvictionQueue) Add(pods []*v1.Pod) {
	for _, pod := range pods {
		if nn := client.ObjectKeyFromObject(pod); !e.Set.Contains(nn) {
			e.Set.Add(nn)
			e.RateLimitingInterface.Add(nn)
		}
	}
}

func (e *EvictionQueue) Start(ctx context.Context) {
	for {
		// Get pod from queue. This waits until queue is non-empty.
		item, shutdown := e.RateLimitingInterface.Get()
		if shutdown {
			break
		}
		nn := item.(types.NamespacedName)
		// Evict pod
		if e.evict(ctx, nn) {
			e.RateLimitingInterface.Forget(nn)
			e.Set.Remove(nn)
			e.RateLimitingInterface.Done(nn)
			continue
		}
		e.RateLimitingInterface.Done(nn)
		// Requeue pod if eviction failed
		e.RateLimitingInterface.AddRateLimited(nn)
	}
	logging.FromContext(ctx).Errorf("EvictionQueue is broken and has shutdown")
}

// evict returns true if successful eviction call, error is returned if not eviction-related error
func (e *EvictionQueue) evict(ctx context.Context, nn types.NamespacedName) bool {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("pod", nn))
	err := e.coreV1Client.Pods(nn.Namespace).Evict(ctx, &v1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
	})
	if errors.IsNotFound(err) { // 404
		return true
	}
	if errors.IsTooManyRequests(err) { // 429
		logging.FromContext(ctx).Debug(err)
		return false
	}
	if err != nil {
		logging.FromContext(ctx).Error(err)
		return false
	}
	logging.FromContext(ctx).Debug("Evicted pod")
	return true
}
