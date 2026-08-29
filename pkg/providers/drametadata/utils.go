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

package drametadata

import (
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Attributes converts scraped device attributes into their DRA form.
func Attributes(in map[string]DRADeviceAttribute) map[resourcev1.QualifiedName]resourcev1.DeviceAttribute {
	if len(in) == 0 {
		return nil
	}
	out := make(map[resourcev1.QualifiedName]resourcev1.DeviceAttribute, len(in))
	for name, attribute := range in {
		out[resourcev1.QualifiedName(name)] = resourcev1.DeviceAttribute{
			StringValue:  attribute.String,
			IntValue:     attribute.Int,
			BoolValue:    attribute.Bool,
			VersionValue: attribute.Version,
		}
	}
	return out
}

// Capacity converts scraped device capacities into their DRA form.
func Capacity(in map[string]string) map[resourcev1.QualifiedName]resourcev1.DeviceCapacity {
	if len(in) == 0 {
		return nil
	}
	out := make(map[resourcev1.QualifiedName]resourcev1.DeviceCapacity, len(in))
	for name, quantity := range in {
		out[resourcev1.QualifiedName(name)] = resourcev1.DeviceCapacity{Value: resource.MustParse(quantity)}
	}
	return out
}
