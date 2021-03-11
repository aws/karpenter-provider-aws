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

package binpacking

import (
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type SortablePods []*v1.Pod

func (pods SortablePods) Len() int {
	return len(pods)
}

func (pods SortablePods) Swap(i, j int) {
	pods[i], pods[j] = pods[j], pods[i]
}

type ByResourcesRequested struct{ SortablePods }

func (r ByResourcesRequested) Less(a, b int) bool {
	resourcePodA := scheduling.GetResources(&r.SortablePods[a].Spec)
	resourcePodB := scheduling.GetResources(&r.SortablePods[b].Spec)
	if resourcePodA.Cpu().Equal(*resourcePodB.Cpu()) {
		// check for memory
		return resourcePodA.Memory().Cmp(*resourcePodB.Memory()) == -1
	}
	return resourcePodA.Cpu().Cmp(*resourcePodB.Cpu()) == -1
}

func MergeResources(resources ...v1.ResourceList) v1.ResourceList {
	result := v1.ResourceList{}
	cpu := result.Cpu()
	memory := result.Memory()
	pods := result.Pods()
	for _, resource := range resources {
		cpu.Add(resource.Cpu().DeepCopy())
		memory.Add(resource.Memory().DeepCopy())
		pods.Add(resource.Pods().DeepCopy())
	}
	result[v1.ResourceCPU] = *cpu
	result[v1.ResourceMemory] = *memory
	result[v1.ResourcePods] = *pods
	return result
}

var (
	cpuPercentRanges = []struct {
		start      int64
		end        int64
		percentage float64
	}{
		{
			start:      0,
			end:        1000,
			percentage: 0.06,
		},
		{
			start:      1000,
			end:        2000,
			percentage: 0.01,
		},
		{
			start:      2000,
			end:        4000,
			percentage: 0.005,
		},
		{
			start:      4000,
			end:        1 << 31,
			percentage: 0.0025,
		},
	}
)

// refer - https://github.com/awslabs/amazon-eks-ami/blob/ff690788dfaf399e6919eebb59371ee923617df4/files/bootstrap.sh#L183-L194
// refer - https://github.com/awslabs/amazon-eks-ami/pull/419#issuecomment-609985305
func CalculateKubeletOverhead(nodeCapacity v1.ResourceList) v1.ResourceList {
	overhead := v1.ResourceList{}
	nodeCPU := nodeCapacity.Cpu().MilliValue()
	numPods := nodeCapacity.Pods().Value()
	kubeletCPU := int64(0)
	for _, cpuPercentRange := range cpuPercentRanges {
		if nodeCPU >= cpuPercentRange.start {
			r := float64(cpuPercentRange.end - cpuPercentRange.start)
			if nodeCPU < cpuPercentRange.end {
				r = float64(nodeCPU - cpuPercentRange.start)
			}
			kubeletCPU += int64(r * cpuPercentRange.percentage)
		}
	}
	overhead[v1.ResourceCPU] = *resource.NewMilliQuantity(kubeletCPU, resource.DecimalSI)
	overhead[v1.ResourceMemory] = *resource.NewMilliQuantity((11*numPods)+255, resource.DecimalSI)
	return overhead
}
