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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	LaunchTemplateNameFormat = "Karpenter-%s"
	IAMInstanceProfileName   = "KarpenterNodeRole"
)

type LaunchTemplateProvider struct {
	ec2                     ec2iface.EC2API
	launchTemplateCache     *cache.Cache
	instanceProfileProvider *InstanceProfileProvider
	securityGroupProvider   *SecurityGroupProvider
	ssm                     ssmiface.SSMAPI
	clientSet               *kubernetes.Clientset
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*ec2.LaunchTemplate, error) {
	if launchTemplate, ok := p.launchTemplateCache.Get(cluster.Name); ok {
		return launchTemplate.(*ec2.LaunchTemplate), nil
	}
	launchTemplate, err := p.getLaunchTemplate(ctx, cluster)
	if err != nil {
		return nil, err
	}
	p.launchTemplateCache.Set(cluster.Name, launchTemplate, CacheTTL)
	return launchTemplate, nil
}

// TODO, reconcile launch template if not equal to desired launch template (AMI upgrade, role changed, etc)
func (p *LaunchTemplateProvider) getLaunchTemplate(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*ec2.LaunchTemplate, error) {
	describelaunchTemplateOutput, err := p.ec2.DescribeLaunchTemplatesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(fmt.Sprintf(LaunchTemplateNameFormat, cluster.Name))},
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidLaunchTemplateName.NotFoundException" {
		return p.createLaunchTemplate(ctx, cluster)
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

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, cluster *v1alpha1.ClusterSpec) (*ec2.LaunchTemplate, error) {
	securityGroupIds, err := p.getSecurityGroupIds(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("getting security groups, %w", err)
	}

	instanceProfile, err := p.instanceProfileProvider.Get(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("getting instance profile, %w", err)
	}
	amiID, err := p.getAMIID(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting AMI ID, %w", err)
	}
	zap.S().Debugf("AMI ID from SSM %s", amiID)

	output, err := p.ec2.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(fmt.Sprintf(LaunchTemplateNameFormat, cluster.Name)),
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
			UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
				#!/bin/bash
				yum install -y https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_amd64/amazon-ssm-agent.rpm
				/etc/eks/bootstrap.sh %s \
					--kubelet-extra-args '--node-labels=karpenter.sh/provisioned=true' \
					--b64-cluster-ca %s \
					--apiserver-endpoint %s`,
				cluster.Name,
				cluster.CABundle,
				cluster.Endpoint,
			)))),
			// TODO discover this with SSM
			ImageId: amiID,
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

func (p *LaunchTemplateProvider) getAMIID(context context.Context) (*string, error) {
	version, err := p.kubeServerVersion()
	if err != nil {
		return nil, fmt.Errorf("kube server version, %w", err)
	}
	paramOutput, err := p.ssm.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended", version)),
	})
	if err != nil {
		return nil, fmt.Errorf("ssm get parameter, %w", err)
	}
	output := struct {
		ImageID string `json:"image_id"`
	}{}
	if err := json.Unmarshal([]byte(*paramOutput.Parameter.Value), &output); err != nil {
		return nil, fmt.Errorf("unmarshal parameter output, %w", err)
	}
	return &output.ImageID, nil
}

func (p *LaunchTemplateProvider) kubeServerVersion() (string, error) {
	version, err := p.clientSet.Discovery().ServerVersion()
	return fmt.Sprintf("%s.%s", version.Major, strings.TrimSuffix(version.Minor, "+")), err
}
