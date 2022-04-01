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
	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PodNamespacedNames(pods []*v1.Pod) []types.NamespacedName {
	namespacedNames := []types.NamespacedName{}
	for _, pod := range pods {
		namespacedNames = append(namespacedNames, client.ObjectKeyFromObject(pod))
	}
	return namespacedNames
}

func MustHash(object interface{}) uint64 {
	hash, err := hashstructure.Hash(object, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	runtime.Must(err)
	return hash
}
