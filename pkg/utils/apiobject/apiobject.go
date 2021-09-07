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

package apiobject

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PodNamespacedNames(pods []*v1.Pod) []types.NamespacedName {
	namespacedNames := []types.NamespacedName{}
	for _, pod := range pods {
		namespacedNames = append(namespacedNames, client.ObjectKeyFromObject(pod))
	}
	return namespacedNames
}

func MatchingLabelsSelector(labelSelector *metav1.LabelSelector) client.ListOption {
	listOption := client.MatchingLabelsSelector{Selector: labels.NewSelector()}
	if labelSelector == nil {
		return listOption
	}
	for key, value := range labelSelector.MatchLabels {
		requirement, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		listOption.Selector.Add(*requirement)
	}
	for _, expression := range labelSelector.MatchExpressions {
		requirement, _ := labels.NewRequirement(expression.Key, selection.Operator(expression.Operator), expression.Values)
		listOption.Selector.Add(*requirement)
	}
	return listOption
}
