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
	"fmt"

	"github.com/imdario/mergo"

	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

func AWSNodeTemplate(overrides ...v1alpha1.AWSNodeTemplateSpec) *v1alpha1.AWSNodeTemplate {
	options := v1alpha1.AWSNodeTemplateSpec{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}

	if options.AWS.SecurityGroupSelector == nil {
		options.AWS.SecurityGroupSelector = map[string]string{"*": "*"}
	}

	if options.AWS.SubnetSelector == nil {
		options.AWS.SubnetSelector = map[string]string{"*": "*"}
	}

	return &v1alpha1.AWSNodeTemplate{
		ObjectMeta: test.ObjectMeta(),
		Spec:       options,
	}
}
