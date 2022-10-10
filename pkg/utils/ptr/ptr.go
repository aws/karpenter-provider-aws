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
	"k8s.io/apimachinery/pkg/api/resource"
)

func Pod(pod v1.Pod) *v1.Pod {
	return &pod
}

func Node(node v1.Node) *v1.Node {
	return &node
}

func Quantity(quantity resource.Quantity) *resource.Quantity {
	return &quantity
}

func To[T any](v T) *T {
	return &v
}

func From[T any](v *T) T {
	return *v
}
