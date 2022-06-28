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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	aws "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type AWSNodeTemplateOptions struct {
	metav1.ObjectMeta
	UserData *string
	AWS      *aws.AWS
	AMIs     []v1alpha1.AMI
}

func AWSNodeTemplate(overrides ...AWSNodeTemplateOptions) *v1alpha1.AWSNodeTemplate {
	options := AWSNodeTemplateOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge aws node template options: %s", err))
		}
	}
	return &v1alpha1.AWSNodeTemplate{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1alpha1.AWSNodeTemplateSpec{
			UserData: options.UserData,
			AWS:      *options.AWS,
			AMIs:     options.AMIs,
		},
	}
}
