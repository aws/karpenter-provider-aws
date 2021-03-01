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

package fleet

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	launchTemplateNameFormat = "Karpenter-%s-%s"
	bottlerocketUserData     = `
[settings.kubernetes]
api-server = "{{.Endpoint}}"
cluster-certificate = "{{.CABundle}}"
cluster-name = "{{.Name}}"
[settings.kubernetes.node-labels]
"karpenter.sh/provisioned" = "true"
`
)

type LaunchTemplateProvider struct {
	ec2                     ec2iface.EC2API
	launchTemplateCache     *cache.Cache
	instanceProfileProvider *InstanceProfileProvider
	securityGroupProvider   *SecurityGroupProvider
	ssm                     ssmiface.SSMAPI
	clientSet               *kubernetes.Clientset
}

// Translate architecture into AWS-recognized name for Bottlerocket
// AMI.
func normalizeArchitecture(architecture v1alpha1.Architecture) string {
	switch architecture {
	case v1alpha1.ArchitectureAmd64:
		return "x86_64"
	default:
		return string(architecture)
	}
}

func launchTemplateName(clusterName string, arch string) string {
	return fmt.Sprintf(launchTemplateNameFormat, clusterName, arch)
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, cluster *v1alpha1.ClusterSpec, constraints *cloudprovider.Constraints) (*ec2.LaunchTemplate, error) {
	arch := normalizeArchitecture(*constraints.Architecture)
	name := launchTemplateName(cluster.Name, arch)
	if launchTemplate, ok := p.launchTemplateCache.Get(name); ok {
		return launchTemplate.(*ec2.LaunchTemplate), nil
	}
	launchTemplate, err := p.getLaunchTemplate(ctx, cluster, arch)
	if err != nil {
		return nil, err
	}
	p.launchTemplateCache.Set(name, launchTemplate, CacheTTL)
	return launchTemplate, nil
}

// TODO, reconcile launch template if not equal to desired launch template (AMI upgrade, role changed, etc)
func (p *LaunchTemplateProvider) getLaunchTemplate(ctx context.Context, cluster *v1alpha1.ClusterSpec, arch string) (*ec2.LaunchTemplate, error) {
	describelaunchTemplateOutput, err := p.ec2.DescribeLaunchTemplatesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(launchTemplateName(cluster.Name, arch))},
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidLaunchTemplateName.NotFoundException" {
		return p.createLaunchTemplate(ctx, cluster, arch)
	}
	if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	}
	if length := len(describelaunchTemplateOutput.LaunchTemplates); length > 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", length)
	}
	launchTemplate := describelaunchTemplateOutput.LaunchTemplates[0]
	zap.S().Debugf("Successfully discovered launch template %s for cluster %s", *launchTemplate.LaunchTemplateName, cluster.Name)
	return launchTemplate, nil
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, cluster *v1alpha1.ClusterSpec, arch string) (*ec2.LaunchTemplate, error) {
	securityGroupIds, err := p.getSecurityGroupIds(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("getting security groups, %w", err)
	}
	instanceProfile, err := p.instanceProfileProvider.Get(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("getting instance profile, %w", err)
	}
	amiID, err := p.getAMIID(ctx, arch)
	if err != nil {
		return nil, fmt.Errorf("getting AMI ID, %w", err)
	}
	zap.S().Debugf("Successfully discovered AMI ID %s for architecture %s", *amiID, arch)
	userData, err := p.getUserData(cluster)
	if err != nil {
		return nil, fmt.Errorf("getting user data, %w", err)
	}

	output, err := p.ec2.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(cluster.Name, arch)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: instanceProfile.InstanceProfileName,
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{{
					Key:   aws.String(fmt.Sprintf(ClusterTagKeyFormat, cluster.Name)),
					Value: aws.String("owned"),
				}},
			}},
			SecurityGroupIds: securityGroupIds,
			UserData:         userData,
			ImageId:          amiID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating launch template, %w", err)
	}
	zap.S().Debugf("Successfully created default launch template, %s", *output.LaunchTemplate.LaunchTemplateName)
	return output.LaunchTemplate, nil
}

func (p *LaunchTemplateProvider) getSecurityGroupIds(ctx context.Context, cluster *v1alpha1.ClusterSpec) ([]*string, error) {
	securityGroupIds := []*string{}
	securityGroups, err := p.securityGroupProvider.Get(ctx, cluster.Name)
	if err != nil {
		return nil, err
	}
	for _, securityGroup := range securityGroups {
		securityGroupIds = append(securityGroupIds, securityGroup.GroupId)
	}
	return securityGroupIds, nil
}

func (p *LaunchTemplateProvider) getAMIID(ctx context.Context, arch string) (*string, error) {
	version, err := p.kubeServerVersion()
	if err != nil {
		return nil, fmt.Errorf("kube server version, %w", err)
	}
	paramOutput, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/%s/latest/image_id", version, arch)),
	})
	if err != nil {
		return nil, fmt.Errorf("getting ssm parameter, %w", err)
	}
	return paramOutput.Parameter.Value, nil
}

func (p *LaunchTemplateProvider) getUserData(cluster *v1alpha1.ClusterSpec) (*string, error) {
	t := template.Must(template.New("userData").Parse(bottlerocketUserData))
	var userData bytes.Buffer
	if err := t.Execute(&userData, cluster); err != nil {
		return nil, err
	}
	return aws.String(base64.StdEncoding.EncodeToString(userData.Bytes())), nil
}

func (p *LaunchTemplateProvider) kubeServerVersion() (string, error) {
	version, err := p.clientSet.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", version.Major, strings.TrimSuffix(version.Minor, "+")), nil
}
