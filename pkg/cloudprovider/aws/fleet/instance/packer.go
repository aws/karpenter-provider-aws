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

package instance

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	v1 "k8s.io/api/core/v1"
)

// Return a list of viable instance types

type Packer struct {

}


func (i *Packer) Pack(pods []*v1.Pod, overhead *v1.ResourceList) []*Packing {
	return nil
}

type Packing struct {
	// Pods to be packed onto this instance.
	Pods []*v1.Pod
	// InstanceTypes that are viable, ordered by price
	InstanceTypes []ec2.InstanceType
}


// Try to add Pod to Node
// If fits, continue
// If not fits, increase node size (for resource)
