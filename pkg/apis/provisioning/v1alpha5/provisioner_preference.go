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

package v1alpha5

import v1 "k8s.io/api/core/v1"

// Rank returns an integer score that represents how many specifed keys are compatible.
// A positible rank value does not mean that the pod is compatible to the provisioner's requirements.
func (p *Provisioner) Rank(pod *v1.Pod) int {
	return p.Spec.Requirements.Keys().Intersection(NewPodRequirements(pod).Keys()).Len()
}
