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

package node

import (
	"context"
	"math"
	"time"

	set "github.com/deckarep/golang-set"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
)

const (
	schedulateTerminationOfBatchAfter = 60
	terminationPercentage             = 5
)

type TerminationQueue struct {
	workqueue.DelayingInterface
	set.Set

	ctx          context.Context
	coreV1Client corev1.CoreV1Interface
}

func NewTerminationQueue(ctx context.Context, coreV1Client corev1.CoreV1Interface) *TerminationQueue {
	queue := &TerminationQueue{
		DelayingInterface: workqueue.NewDelayingQueue(),
		Set:               set.NewSet(),

		ctx:          ctx,
		coreV1Client: coreV1Client,
	}
	go queue.Start(logging.WithLogger(ctx, logging.FromContext(ctx).Named("terminatorQueue")))
	return queue
}

// Add adds pods to the EvictionQueue
func (e *TerminationQueue) Add(node *v1.Node, totalNodesProvisioned int) {
	if totalNodesProvisioned == 0 {
		//Let the status of the provisioner be reconciled first
		return
	}
	currentTerminationQueueLength := float64(e.Set.Cardinality())
	//compute batch depending upon the length of the queue and total number of provisioned
	batchSize := math.Ceil(float64(terminationPercentage*totalNodesProvisioned) / 100)
	var scheduleInBatch = int64(0)
	if batchSize > 0 {
		scheduleInBatch = int64(math.Ceil(currentTerminationQueueLength / batchSize))
	}
	logging.FromContext(e.ctx).With("name", node.GetName()).Debugf("Batchsize is %v , current lenght of queue is %v", batchSize, currentTerminationQueueLength)
	if !e.Set.Contains(node.GetName()) {
		e.Set.Add(node.GetName())
		logging.FromContext(e.ctx).Infof("Added node for termination in DelayQueue in batch %v", scheduleInBatch)
		//Adding length of queue + 5sec in duration to counter the scenario where the reconciles are not concurrent enough, the length of queue will always be 0, because the item will
		//be processed by the queue almost instantaneously, which may result in all nodes going down together, rendering this queue as useless.
		e.DelayingInterface.AddAfter(node.GetName(), (time.Duration(scheduleInBatch)*schedulateTerminationOfBatchAfter*time.Second)+5)
	}
}

func (e *TerminationQueue) Start(ctx context.Context) {
	for {
		// Get pod from queue. This waits until queue is non-empty.
		item, shutdown := e.DelayingInterface.Get()
		if shutdown {
			break
		}
		nn := item.(string)
		logging.FromContext(ctx).With("name", nn).Debug("Popped from Delayed TerminationQueue, making Node for Deletion")
		// Evict pod
		if e.terminate(ctx, nn) {
			e.Set.Remove(nn)
			e.DelayingInterface.Done(nn)
			logging.FromContext(ctx).With("name", nn).Info("Marked node for deletion from termination queue")
			continue
		}
		e.DelayingInterface.Done(nn)
		//Todo Just log that it can't be termintated with the reason, or do we again put in queue for some later duration ?
	}
	logging.FromContext(ctx).Errorf("TerminationQueue is broken and has shutdown")
}

func (e *TerminationQueue) terminate(ctx context.Context, name string) bool {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("node", name))
	err := e.coreV1Client.Nodes().Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) { // 404
		return true
	}
	if err != nil {
		logging.FromContext(ctx).Error(err)
		return false
	}
	logging.FromContext(ctx).Debug("Terminated node in TerminationQueue")
	return true
}
