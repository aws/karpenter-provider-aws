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

package packing

import (
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type sortablePods []*v1.Pod

func (pods sortablePods) Len() int {
	return len(pods)
}

func (pods sortablePods) Swap(i, j int) {
	pods[i], pods[j] = pods[j], pods[i]
}

type byResourceRequested struct{ sortablePods }

func (r byResourceRequested) Less(a, b int) bool {
	cpuPodA := cpuFor(r.sortablePods[a])
	cpuPodB := cpuFor(r.sortablePods[b])
	if cpuPodA.Equal(*cpuPodB) {
		// check for memory
		memPodA := memoryFor(r.sortablePods[a])
		memPodB := memoryFor(r.sortablePods[b])
		return memPodA.Cmp(*memPodB) == -1
	}
	return cpuPodA.Cmp(*cpuPodB) == -1
}

func cpuFor(pod *v1.Pod) *resource.Quantity {
	resources := scheduling.GetResources(&pod.Spec)
	return resources.Cpu()
}

func memoryFor(pod *v1.Pod) *resource.Quantity {
	resources := scheduling.GetResources(&pod.Spec)
	return resources.Memory()
}
