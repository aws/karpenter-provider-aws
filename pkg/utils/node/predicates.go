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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
	"time"
)

const (
	// TODO: Decide if this can be configured in the spec, and implement.
	// Only consider a node underutilizd if no non-daemon pods are scheduled to it
	UnderutilizedPercentage = 0
)

func IsReadyAndSchedulable(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue && !node.Spec.Unschedulable
		}
	}
	return false
}

func IsPastTTL(node *v1.Node) bool {
	ttl, ok := node.Annotations[v1alpha1.ProvisionerTTLKey]
	if !ok {
		return false
	}
	ttlTime, err := time.Parse(time.RFC3339, ttl)
	if err != nil {
		return false
	}
	return time.Now().After(ttlTime)
}

// IsUnderutilized returns true if all of a node's pod's resource requests sum to less than the given thresholds
func IsUnderutilized(node *v1.Node, pods []*v1.Pod) bool {
	threshold := v1.ResourceList{
		v1.ResourceCPU: *resource.NewMilliQuantity(
			roundQuantity(node.Status.Capacity.Cpu().MilliValue(), UnderutilizedPercentage),
			resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(
			roundQuantity(node.Status.Capacity.Memory().Value(), UnderutilizedPercentage),
			resource.DecimalSI),
		v1.ResourcePods: *resource.NewQuantity(
			roundQuantity(node.Status.Capacity.Pods().Value(), UnderutilizedPercentage),
			resource.BinarySI),
	}
	cpuTotal := &resource.Quantity{}
	memoryTotal := &resource.Quantity{}
	podTotal := &resource.Quantity{}

	for _, pod := range pods {
		if !scheduling.IsOwnedByDaemonSet(pod) {
			zap.S().Debugf("This is the pod name %s", pod.Name)
			resources := scheduling.GetResources(&pod.Spec)
			cpuTotal.Add(*resources.Cpu())
			memoryTotal.Add(*resources.Memory())
			podTotal.Add(*resource.NewQuantity(1, resource.BinarySI))
		}
	}

	return (cpuTotal.Cmp(*threshold.Cpu()) == -1 &&
		memoryTotal.Cmp(*threshold.Memory()) == -1 &&
		podTotal.Cmp(*threshold.Pods()) == -1) ||
		(cpuTotal.IsZero() && memoryTotal.IsZero() && podTotal.IsZero())
}

// roundQuantity calculates the rounded int value of a percentage of val
func roundQuantity(val int64, percentage float64) int64 {
	return int64(math.Round(float64(val) * percentage))
}
