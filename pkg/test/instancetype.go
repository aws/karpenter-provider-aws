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

package test

import (
	"context"
	"fmt"

	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/instancetype/v1alpha1"
)

// InstanceTypeOptions customizes an InstanceType.
type InstanceTypeOptions struct {
	metav1.ObjectMeta
	Resources v1.ResourceList
}

// InstanceType creates a test instance type with defaults that can be overridden by InstanceTypeOptions.
// Overrides are applied in order, with a last write wins semantic.
func InstanceType(name string, overrides ...InstanceTypeOptions) *v1alpha1.InstanceType {
	options := InstanceTypeOptions{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge instance type options: %s", err))
		}
	}
	if options.Name == "" {
		panic("instance types must specify a name for testing")
	}

	instanceType := &v1alpha1.InstanceType{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1alpha1.InstanceTypeSpec{
			Resources: options.Resources,
		},
	}
	instanceType.SetDefaults(context.Background())
	return instanceType
}
