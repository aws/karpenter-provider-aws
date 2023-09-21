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

	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

func EC2NodeClass(overrides ...v1beta1.EC2NodeClass) *v1beta1.EC2NodeClass {
	options := v1beta1.EC2NodeClass{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}
	if options.Spec.AMIFamily == nil {
		options.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
	}
	if options.Spec.Role == "" {
		options.Spec.Role = "test-role"
	}
	if len(options.Spec.SecurityGroupSelectorTerms) == 0 {
		options.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
	}
	if len(options.Spec.SubnetSelectorTerms) == 0 {
		options.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
	}
	return &v1beta1.EC2NodeClass{
		ObjectMeta: test.ObjectMeta(options.ObjectMeta),
		Spec:       options.Spec,
	}
}
