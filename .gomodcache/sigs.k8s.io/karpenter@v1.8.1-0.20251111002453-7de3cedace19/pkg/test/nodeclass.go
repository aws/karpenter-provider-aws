/*
Copyright The Kubernetes Authors.

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

package test

import (
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	"github.com/imdario/mergo"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
)

var (
	// defaultNodeClass is the default NodeClass type used when creating NodeClassRefs for NodePools and NodeClaims
	defaultNodeClass status.Object = &v1alpha1.TestNodeClass{}
)

// SetDefaultNodeClassType configures the default NodeClass type used when generating NodeClassRefs for test NodePools and NodeClaims.
func SetDefaultNodeClassType(nc status.Object) {
	defaultNodeClass = nc
}

// NodeClass creates a test NodeClass with defaults that can be overridden by overrides.
// Overrides are applied in order, with a last write wins semantic.
func NodeClass(overrides ...v1alpha1.TestNodeClass) *v1alpha1.TestNodeClass {
	override := v1alpha1.TestNodeClass{}
	for _, opts := range overrides {
		if err := mergo.Merge(&override, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("failed to merge: %v", err))
		}
	}
	if override.Name == "" {
		override.Name = RandomName()
	}
	if override.Status.Conditions == nil {
		override.StatusConditions().SetTrue(status.ConditionReady)
	}
	return &v1alpha1.TestNodeClass{
		ObjectMeta: ObjectMeta(override.ObjectMeta),
		Status:     override.Status,
	}
}
