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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Ubuntu struct {
	v1alpha1.UbuntuOptions
}

func (p *Ubuntu) GetLaunchTemplates(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	return builder.PrepareLauncheTemplates(ctx, p, config, instanceTypes)
}

func (p *Ubuntu) PrepareLaunchTemplate(ctx context.Context, builder *Builder, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error) {
	return builder.Template(ctx, p, &p.BasicLaunchTemplateInput, config, ami, instanceTypes)
}

func (p *Ubuntu) GetImageID(ctx context.Context, builder *Builder, config *Configuration, instanceType cloudprovider.InstanceType) (string, error) {
	if p.ImageID != nil {
		return *p.ImageID, nil
	}
	if p.Version == nil {
		// Search for most recent Ubuntu LTS version
		year, _, _ := time.Now().Date()
		if year%2 == 1 {
			year--
		}
		// Stop at Ubuntu LTS 20.04 as it was the most recent version available when this code was written.
		for year >= 2020 {
			id, err := builder.SsmClient.GetParameterWithContext(ctx, p.getUbuntuAlias(config.KubernetesVersion, fmt.Sprintf("%d.04", year), instanceType))
			if err == nil {
				return id, nil
			}
			year -= 2
		}
		return "", fmt.Errorf("unable to find Ubuntu LTS AMI image")
	}
	return builder.SsmClient.GetParameterWithContext(ctx, p.getUbuntuAlias(config.KubernetesVersion, *p.Version, instanceType))
}

func (p *Ubuntu) getUbuntuAlias(k8sVersion K8sVersion, ubuntuVersion string, instanceType cloudprovider.InstanceType) string {
	return fmt.Sprintf("/aws/service/canonical/ubuntu/eks/%s/%s/stable/current/%s/hvm/ebs-gp2/ami-id", ubuntuVersion, k8sVersion.String(), instanceType.Architecture())
}

func (p *Ubuntu) GetUserData(_ context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error) {
	return getALandUbuntuUserData(config, instanceTypes)
}
