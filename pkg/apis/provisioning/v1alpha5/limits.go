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
	"context"
	"math"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Limits define bounds on the resources being provisioned by Karpenter
type Limits struct {
	// Resources contains all the allocatable resources that Karpenter supports for limiting.
	Resources Resources `json:"resources,omitempty"`
}

func (l *Limits) Default(ctx context.Context) {
	if l.Resources.CPU == nil {
		l.Resources.CPU = resource.NewScaledQuantity(math.MaxInt64, 0)
	}
	if l.Resources.Memory == nil {
		l.Resources.Memory = resource.NewScaledQuantity(math.MaxInt64, 0)
	}
}
