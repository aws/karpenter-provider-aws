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

package allocation

import (
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// Allocator allocates new capacity for a set of pods
type Allocator interface {
	Allocate(*v1alpha1.Provisioner, []*v1.Pod) error
}
