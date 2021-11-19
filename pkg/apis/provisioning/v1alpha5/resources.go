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

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// Resources contains a list of all the allocatable resources that can be used to define a bound on penter's
// scaling actions.
type Resources struct {
	// CPU, in cores. (500m = .5 cores)
	CPU *resource.Quantity `json:"cpu,omitempty"`
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	Memory *resource.Quantity `json:"memory,omitempty"`
}
