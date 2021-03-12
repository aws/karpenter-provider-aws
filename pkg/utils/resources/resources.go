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

package resources

import v1 "k8s.io/api/core/v1"

func Merge(resources ...v1.ResourceList) v1.ResourceList {
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

// ForPods returns the total resources of a variadic list of pods.
func ForPods(pods ...*v1.PodSpec) v1.ResourceList {
	resources := []v1.ResourceList{}
	for _, pod := range pods {
		for _, container := range pod.Containers {
			resources = append(resources, container.Resources.Requests)
		}
	}
	return Merge(resources...)
}
