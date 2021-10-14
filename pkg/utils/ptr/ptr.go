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

package ptr

import (
	v1 "k8s.io/api/core/v1"
)

func Pod(pod v1.Pod) *v1.Pod {
	return &pod
}

func Node(node v1.Node) *v1.Node {
	return &node
}

func PodListToSlice(pods *v1.PodList) []*v1.Pod {
	podPointers := []*v1.Pod{}
	for _, pod := range pods.Items {
		podPointers = append(podPointers, Pod(pod))
	}
	return podPointers
}

func Int64Value(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
