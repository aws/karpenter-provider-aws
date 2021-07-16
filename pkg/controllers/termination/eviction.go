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

// Evict adds a pod to the EvictionQueue
func (e *EvictionQueue) Add(pods []*v1.Pod) {
	// Start processing eviction queue if it hasn't started already
	e.once.Do(func() { go e.run() })

	for _, pod := range pods {
		nn := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
		if !e.enqueued.Contains(nn) {
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
			zap.S().Infof("Shutdown queue is broken")
		}
		// Panic if not a NamespacedName
		nn := item.(types.NamespacedName)
		// Evict pod
		evicted := e.evict(nn)
		e.queue.Done(nn)
		if evicted {
			zap.S().Debugf("Successfully evicted pod %s", nn.String())
			e.queue.Forget(nn)
			e.enqueued.Remove(nn)
			continue
		}
		// Requeue pod if eviction failed
		e.queue.AddRateLimited(nn)
	}
}

// returns true if successful eviction call, error is returned if not eviction-related error
func (e *EvictionQueue) evict(nn types.NamespacedName) bool {
	err := e.coreV1Client.Pods(nn.Namespace).Evict(context.Background(), &v1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
	})
	// 500
	if errors.IsInternalError(err) {
		zap.S().Debugf("Failed to evict pod %s due to PDB misconfiguration error. Backing off then trying again.", nn.String())
		return false
	}
	// 429
	if errors.IsTooManyRequests(err) {
		zap.S().Debugf("Failed to evict pod %s due to PDB violation. Backing off then trying again.", nn.String())
		return false
	}
	// 404
	if errors.IsNotFound(err) {
		zap.S().Debugf("Continuing after failing to evict pod %s since it was not found.", nn.String())
		return true
	}
	if err != nil {
		return false
	}
	return true
}
