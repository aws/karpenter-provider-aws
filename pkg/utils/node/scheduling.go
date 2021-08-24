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

import v1 "k8s.io/api/core/v1"

func UniqueTaints(taints []v1.Taint, extraTaints ...v1.Taint) []v1.Taint {
	unique := make([]v1.Taint, 0)
	for _, input := range [][]v1.Taint{taints, extraTaints} {
		for _, t := range input {
			for _, u := range unique {
				if t.Key == u.Key && t.Value == u.Value && t.Effect == u.Effect {
					continue
				}
			}
			unique = append(unique, t)
		}
	}
	return unique
}
