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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type pods []*v1.Pod

func (pods pods) Len() int {
	return len(pods)
}

func (pods pods) Swap(i, j int) {
	pods[i], pods[j] = pods[j], pods[i]
}

type byResourceRequested struct{ pods }

func (r byResourceRequested) Less(a, b int) bool {
	cpuPodA := calculateCPURequested(r.pods[a])
	cpuPodB := calculateCPURequested(r.pods[b])
	if cpuPodA.Equal(cpuPodB) {
		// check for memory
		memPodA := calculateMemoryRequested(r.pods[a])
		memPodB := calculateMemoryRequested(r.pods[b])
		return memPodA.MilliValue() < memPodB.MilliValue()
	}
	return cpuPodA.MilliValue() < cpuPodB.MilliValue()
}

func calculateCPURequested(pod *v1.Pod) resource.Quantity {
	cpu := resource.MustParse("0")
	for _, container := range pod.Spec.Containers {
		cpu.Add(*container.Resources.Requests.Cpu())
	}
	return cpu
}

func calculateMemoryRequested(pod *v1.Pod) resource.Quantity {
	memory := resource.MustParse("0")
	for _, container := range pod.Spec.Containers {
		memory.Add(*container.Resources.Requests.Memory())
	}
	return memory
}
