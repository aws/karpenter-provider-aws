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

package pod

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	IgnoredOwners = []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
	}
)

func HasFailed(pod *v1.Pod) bool {
	return pod.Status.Phase == "Failed"
}

func IsOwnedByDaemonSet(pod *v1.Pod) bool {
	for _, ignoredOwner := range IgnoredOwners {
		for _, owner := range pod.ObjectMeta.OwnerReferences {
			if owner.APIVersion == ignoredOwner.GroupVersion().String() && owner.Kind == ignoredOwner.Kind {
				return true
			}
		}
	}
	return false
}
