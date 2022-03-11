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

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Input struct {
	ByReference *v1alpha1.LauchtemplateReference
	ByContent   *ec2.RequestLaunchTemplateData
}

// Configuration a combination of user provided configuration and ambient information.
type Configuration struct {
	Constraints *v1alpha1.Constraints
	// KubernetesVersion the kubernetes server version.
	KubernetesVersion K8sVersion
	//ClusterEndpoint
	ClusterEndpoint string
	// CaBundle
	CABundle *string
	// ClusterName the EKS Kubernetes cluster name
	ClusterName string
	NodeLabels  map[string]string
	// DefaultInstanceProfile the default EC2 instance profile
	DefaultInstanceProfile string
	// AWSENILimitedPodDensity
	AWSENILimitedPodDensity bool
}

type GenericOsProvider interface {
	GetLaunchTemplates(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error)
}

type OsProvider interface {
	// GetImageId lookup the AMI ImageId for a specific configuration and instance type.
	GetImageID(ctx context.Context, builder *Builder, config *Configuration, instanceType cloudprovider.InstanceType) (string, error)

	// UserData plain (not base64 encoded) user data
	GetUserData(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error)

	// PrepareLaunchTemplate
	PrepareLaunchTemplate(ctx context.Context, builder *Builder, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error)
}

func OSProviderOf(a *v1alpha1.AWS) GenericOsProvider {
	if a.Amazonlinux != nil {
		return &Amazonlinux{*a.Amazonlinux}
	}
	if a.Bottlerocket != nil {
		return &Bottlerocket{*a.Bottlerocket}
	}
	if a.Ubuntu != nil {
		return &Ubuntu{*a.Ubuntu}
	}
	if a.Generic != nil {
		return &Generic{*a.Generic}
	}
	if a.Predefined != nil {
		return &Predefined{*a.Predefined}
	}
	return &Amazonlinux{}
}
