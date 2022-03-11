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

package launchtemplate

import (
	"context"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Predefined struct {
	v1alpha1.PredefinedOptions
}

func (p *Predefined) GetLaunchTemplates(_ context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	return map[Input][]cloudprovider.InstanceType{
		{
			ByReference: &v1alpha1.LauchtemplateReference{
				LaunchTemplateID:   p.LaunchTemplateID,
				LaunchTemplateName: p.LaunchTemplateName,
				Version:            p.Version,
			},
		}: instanceTypes,
	}, nil
}
