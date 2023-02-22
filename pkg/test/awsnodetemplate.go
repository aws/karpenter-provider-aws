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
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

func AWSNodeTemplate(overrides ...v1alpha1.AWSNodeTemplateSpec) *v1alpha1.AWSNodeTemplate {
	return &v1alpha1.AWSNodeTemplate{
		ObjectMeta: test.ObjectMeta(),
		Spec: test.MustMerge(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SubnetSelector:        map[string]string{"*": "*"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			},
		}, overrides...),
	}
}
