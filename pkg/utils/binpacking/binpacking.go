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
