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
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/utils/pointer"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Generic struct {
	v1alpha1.GenericOptions
}

func (p *Generic) GetLaunchTemplates(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	return builder.PrepareLauncheTemplates(ctx, p, config, instanceTypes)
}

func (p *Generic) PrepareLaunchTemplate(ctx context.Context, builder *Builder, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error) {
	return builder.Template(ctx, p, &p.BasicLaunchTemplateInput, config, ami, instanceTypes)
}

func (p *Generic) GetImageID(_ context.Context, builder *Builder, config *Configuration, instanceType cloudprovider.InstanceType) (string, error) {
	return p.ImageID, nil
}

func (p *Generic) GetUserData(_ context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error) {
	templateInput := p.GenericLaunchTemplateInput.UserData
	if p.VerbatimUserData || templateInput == nil {
		return templateInput, nil
	}
	t, err := p.ParseTemplate()
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	err = t.Execute(&buffer, config)
	if err != nil {
		return nil, fmt.Errorf("rendering userData template, %w", err)
	}
	return pointer.String(buffer.String()), nil
}
