package termination

import (
	"context"
	"sync"

	set "github.com/deckarep/golang-set"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
)

type EvictionQueue struct {
	queue        workqueue.RateLimitingInterface
	coreV1Client corev1.CoreV1Interface

	enqueued set.Set
	once     sync.Once
}

// Add adds pods to the EvictionQueue
func (e *EvictionQueue) Add(pods []*v1.Pod) {
	// Start processing eviction queue if it hasn't started already
	e.once.Do(func() { go e.run() })

	for _, pod := range pods {
		if nn := (types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}); !e.enqueued.Contains(nn) {
			e.enqueued.Add(nn)
			e.queue.Add(nn)
		}
	}
}

func (e *EvictionQueue) run() {
	for {
		// Get pod from queue. This waits until queue is non-empty.
		item, shutdown := e.queue.Get()
		if shutdown {
			break
		}
		nn := item.(types.NamespacedName)
		// Evict pod
		if e.evict(nn) {
			zap.S().Debugf("Evicted pod %s", nn.String())
			e.queue.Forget(nn)
			e.enqueued.Remove(nn)
			e.queue.Done(nn)
			continue
		}
		e.queue.Done(nn)
		// Requeue pod if eviction failed
		e.queue.AddRateLimited(nn)
	}
	zap.S().Errorf("EvictionQueue is broken and has shutdown.")
}

// evict returns true if successful eviction call, error is returned if not eviction-related error
func (e *EvictionQueue) evict(nn types.NamespacedName) bool {
	err := e.coreV1Client.Pods(nn.Namespace).Evict(context.Background(), &v1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
	})
	if errors.IsInternalError(err) { // 500
		zap.S().Debugf("Failed to evict pod %s due to PDB misconfiguration error.", nn.String())
		return false
	}
	if errors.IsTooManyRequests(err) { // 429
		zap.S().Debugf("Failed to evict pod %s due to PDB violation.", nn.String())
		return false
	}
	if errors.IsNotFound(err) { // 404
		return true
	}
	if err != nil {
		return false
	}
	return true
}
