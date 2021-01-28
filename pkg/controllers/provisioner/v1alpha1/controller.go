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

package v1alpha1

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/controllers"
	"github.com/awslabs/karpenter/pkg/controllers/provisioner/v1alpha1/allocation"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller for the resource
type Controller struct {
	Client    client.Client
	Allocator allocation.Allocator
	sync.RWMutex
	ProcessedPods map[string]bool
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Object {
	return &v1alpha1.Provisioner{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Object {
	return []controllers.Object{}
}

func (c *Controller) Interval() time.Duration {
	return 60 * time.Second
}

// Reconcile executes a control loop for the resource
//
// SKIP FOR NOW, Attempt to schedule pods on existing capacity
// SKIP FOR NOW, Attempt to schedule remaining pods by preempting existing pods
func (c *Controller) Reconcile(object controllers.Object) error {
	_ = object.(*v1alpha1.Provisioner)

	// 1. List Pods where pod.spec.nodeName = ''
	pods := &v1.PodList{}
	if err := c.Client.List(context.Background(), pods, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		return fmt.Errorf("Listing unscheduled pods, %w", err)
	}

	unschedulable := []*v1.Pod{}
	for _, pod := range pods.Items {
		tempPod := pod
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodScheduled && condition.Reason == v1.PodReasonUnschedulable {
				unschedulable = append(unschedulable, &tempPod)
			}
		}
	}
	pendingPods := c.removeProcessedPods(unschedulable)
	if len(pendingPods) == 0 {
		return nil
	}
	// Attempt to schedule remaining pods by creating a set of nodes
	if err := c.Allocator.Allocate(pendingPods); err != nil {
		return fmt.Errorf("failed to allocate %d pods, %w", len(unschedulable), err)
	}
	return nil
}

func (c *Controller) removeProcessedPods(unschedulable []*v1.Pod) []*v1.Pod {
	needsProcessing := make([]*v1.Pod, 0)
	c.Lock()
	defer c.Unlock()
	for _, pod := range unschedulable {
		key := podKeyCreate(pod)
		// if a pod is never seen before, add to the list needsProcessing
		if _, ok := c.ProcessedPods[key]; !ok {
			needsProcessing = append(needsProcessing, pod)
		}
		// unschedulable pods are set as false
		c.ProcessedPods[key] = false
	}
	// remove the pods which are not pending anymore.
	for podKey, done := range c.ProcessedPods {
		if done {
			delete(c.ProcessedPods, podKey)
			continue
		}
		c.ProcessedPods[podKey] = true
	}
	return needsProcessing
}

func podKeyCreate(pod *v1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}
