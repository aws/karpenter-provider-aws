package termination

import (
	"context"
	"sync"
	"time"

	set "github.com/deckarep/golang-set"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
)

type EvictionQueue struct {
	Queue        workqueue.DelayingInterface
	RateLimiter  workqueue.RateLimiter
	coreV1Client corev1.CoreV1Interface

	enqueued set.Set
	once     sync.Once
}

// Evict adds a pod to the EvictionQueue
func (e *EvictionQueue) Add(pods []*v1.Pod) {
	e.once.Do(func() { go e.run() })

	for _, pod := range pods {
		if !e.enqueued.Contains(pod) {
			zap.S().Debugf("DEBUGGING: Enqueued pod %s", pod.Name)
			e.enqueued.Add(pod)
			e.Queue.Add(pod)
		}
	}
}

func (e *EvictionQueue) run() {
	for {
		if e.Queue.Len() == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		// Get pod from queue
		item, shutdown := e.Queue.Get()
		if shutdown {
			zap.S().Infof("Shutdown queue is broken")
		}
		pod, ok := item.(*v1.Pod)
		if !ok {
			panic("Item is not pod")
		}
		// Evict pod
		success := e.evict(pod)
		if success {
			zap.S().Debugf("Successfully evicted pod %s", pod.Name)
			e.Queue.Done(item)
			e.RateLimiter.Forget(item)
			e.enqueued.Remove(item)
			continue
		}
		// Requeue pod if failed
		zap.S().Debugf("DEBUGGING: this is the num of failures for pod %s: %d", pod.Name, e.RateLimiter.NumRequeues(item))
		e.Queue.AddAfter(item, e.RateLimiter.When(item))
	}
}

// returns true if successful eviction call, error is returned if not eviction-related error
func (e *EvictionQueue) evict(pod *v1.Pod) bool {
	err := e.coreV1Client.Pods(pod.Namespace).Evict(context.Background(), &v1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
	})
	// 500
	if errors.IsInternalError(err) {
		zap.S().Debugf("Failed to evict pod %s due to misconfiguration error.", pod.Name)
		return false
	}
	// 429
	if errors.IsTooManyRequests(err) {
		zap.S().Debugf("Failed to evict pod %s due to PDB violation", pod.Name)
		return false
	}
	// 404
	if errors.IsNotFound(err) {
		return true
	}
	if err != nil {
		return false
	}
	return true
}
