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

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// RequestsForPods returns the total resources of a variadic list of podspecs.
func RequestsForPods(pods ...*v1.Pod) v1.ResourceList {
	resources := []v1.ResourceList{}
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			resources = append(resources, container.Resources.Requests)
		}
	}
	return Merge(resources...)
}

// LimitsForPods returns the total resources of a variadic list of podspecs
func LimitsForPods(pods ...*v1.Pod) v1.ResourceList {
	resources := []v1.ResourceList{}
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			resources = append(resources, container.Resources.Limits)
		}
	}
	return Merge(resources...)
}

// Merge the resources from the variadic into a single v1.ResourceList
func Merge(resources ...v1.ResourceList) v1.ResourceList {
	result := v1.ResourceList{}
	for _, resourceList := range resources {
		for resourceName, quantity := range resourceList {
			current := result[resourceName]
			current.Add(quantity)
			result[resourceName] = current
		}
	}
	return result
}

// Quantity parses the string value into a *Quantity
func Quantity(value string) *resource.Quantity {
	r := resource.MustParse(value)
	return &r
}

// Cmp implements lhs.Cmp(rhs). This method is provided to make some code a bit cleaner as the Quantity.Cmp takes
// a pointer receiver and map index expressions aren't addressable, so it can't be called directly.
func Cmp(lhs, rhs resource.Quantity) int {
	return lhs.Cmp(rhs)
}

// IsZero implements r.IsZero(). This method is provided to make some code a bit cleaner as the Quantity.IsZero() takes
// a pointer receiver and map index expressions aren't addressable, so it can't be called directly.
func IsZero(r resource.Quantity) bool {
	return r.IsZero()
}
