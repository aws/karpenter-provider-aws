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

package aws

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/mitchellh/hashstructure/v2"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"

	"github.com/patrickmn/go-cache"
)

const (
	launchTemplateNameFormat = "Karpenter-%s-%s"
)

type LaunchTemplateProvider struct {
	ec2api                ec2iface.EC2API
	amiProvider           *AMIProvider
	securityGroupProvider *SecurityGroupProvider
	cache                 *cache.Cache
}

func NewLaunchTemplateProvider(ec2api ec2iface.EC2API, amiProvider *AMIProvider, securityGroupProvider *SecurityGroupProvider) *LaunchTemplateProvider {
	return &LaunchTemplateProvider{
		ec2api:                ec2api,
		amiProvider:           amiProvider,
		securityGroupProvider: securityGroupProvider,
		cache:                 cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func launchTemplateName(options *launchTemplateOptions) string {
	hash, err := hashstructure.Hash(options, hashstructure.FormatV2, nil)
	if err != nil {
		panic(fmt.Sprintf("hashing launch template, %s", err.Error()))
	}
	return fmt.Sprintf(launchTemplateNameFormat, ptr.StringValue(options.Cluster.Name), fmt.Sprint(hash))
}

// launchTemplateOptions is hashed and results in the creation of a real EC2
// LaunchTemplate. Do not change this struct without thinking through the impact
// to the number of LaunchTemplates that will result from this change.
type launchTemplateOptions struct {
	// Edge-triggered fields that will only change on kube events.
	Cluster  v1alpha3.Cluster
	UserData string
	// Level-triggered fields that may change out of sync.
	SecurityGroups []string
	AMIID          string
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, provisioner *v1alpha3.Provisioner, constraints *Constraints) (*LaunchTemplate, error) {
	// 1. If the customer specified a launch template then just use it
	if result := constraints.GetLaunchTemplate(); result != nil {
		return result, nil
	}

	// 2. Get constrained AMI ID
	amiID, err := p.amiProvider.Get(ctx, constraints)
	if err != nil {
		return nil, err
	}

	// 3. Get constrained security groups
	securityGroups, err := p.getSecurityGroupIds(ctx, provisioner, constraints)
	if err != nil {
		return nil, err
	}

	// 3. Get userData for Node
	userData, err := p.getUserData(ctx, provisioner, constraints)
	if err != nil {
		return nil, err
	}

	// 4. Ensure the launch template exists, or create it

	launchTemplate, err := p.ensureLaunchTemplate(ctx, &launchTemplateOptions{
		Cluster:        provisioner.Spec.Cluster,
		UserData:       userData,
		AMIID:          amiID,
		SecurityGroups: securityGroups,
	})
	if err != nil {
		return nil, err
	}
	return &LaunchTemplate{
		Name:    aws.StringValue(launchTemplate.LaunchTemplateName),
		Version: fmt.Sprint(DefaultLaunchTemplateVersion),
	}, nil
}

func (p *LaunchTemplateProvider) ensureLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	var launchTemplate *ec2.LaunchTemplate
	name := launchTemplateName(options)
	// 1. Read from cache
	if launchTemplate, ok := p.cache.Get(name); ok {
		return launchTemplate.(*ec2.LaunchTemplate), nil
	}
	// 2. Attempt to find an existing LT.
	output, err := p.ec2api.DescribeLaunchTemplatesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(name)},
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidLaunchTemplateName.NotFoundException" {
		// 3. Create LT if one doesn't exist
		launchTemplate, err = p.createLaunchTemplate(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("creating launch template, %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	} else if len(output.LaunchTemplates) != 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", len(output.LaunchTemplates))
	} else {
		logging.FromContext(ctx).Debugf("Discovered launch template %s", name)
		launchTemplate = output.LaunchTemplates[0]
	}
	// 4. Populate cache
	p.cache.Set(name, launchTemplate, CacheTTL)
	return launchTemplate, nil
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	output, err := p.ec2api.CreateLaunchTemplateWithContext(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", ptr.StringValue(options.Cluster.Name))),
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("Karpenter/%s", ptr.StringValue(options.Cluster.Name))),
					},
					{
						Key:   aws.String(fmt.Sprintf(ClusterTagKeyFormat, ptr.StringValue(options.Cluster.Name))),
						Value: aws.String("owned"),
					},
					{
						Key:   aws.String(fmt.Sprintf(KarpenterTagKeyFormat, ptr.StringValue(options.Cluster.Name))),
						Value: aws.String("owned"),
					},
				},
			}},
			SecurityGroupIds: aws.StringSlice(options.SecurityGroups),
			UserData:         aws.String(options.UserData),
			ImageId:          aws.String(options.AMIID),
		},
	})
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Created launch template, %s", *output.LaunchTemplate.LaunchTemplateName)
	return output.LaunchTemplate, nil
}

func (p *LaunchTemplateProvider) getSecurityGroupIds(ctx context.Context, provisioner *v1alpha3.Provisioner, constraints *Constraints) ([]string, error) {
	securityGroupIds := []string{}
	securityGroups, err := p.securityGroupProvider.Get(ctx, provisioner, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting security group ids, %w", err)
	}
	for _, securityGroup := range securityGroups {
		securityGroupIds = append(securityGroupIds, aws.StringValue(securityGroup.GroupId))
	}
	return securityGroupIds, nil
}

func (p *LaunchTemplateProvider) getUserData(ctx context.Context, provisioner *v1alpha3.Provisioner, constraints *Constraints) (string, error) {
	var userData bytes.Buffer
	userData.WriteString(fmt.Sprintf("[settings.kubernetes]\napi-server = \"%s\"\n", provisioner.Spec.Cluster.Endpoint))
	userData.WriteString(fmt.Sprintf("cluster-name = \"%s\"\n", *provisioner.Spec.Cluster.Name))
	caBundle, err := provisioner.Spec.Cluster.GetCABundle(ctx)
	if err != nil {
		return "", fmt.Errorf("getting user data, %w", err)
	}
	if caBundle != nil {
		userData.WriteString(fmt.Sprintf("cluster-certificate = \"%s\"\n", *caBundle))
	}
	if len(constraints.Labels) > 0 {
		userData.WriteString("[settings.kubernetes.node-labels]\n")
		for k, v := range constraints.Labels {
			userData.WriteString(fmt.Sprintf("\"%s\" = \"%v\"\n", k, v))
		}
	}
	if len(constraints.Taints) > 0 {
		userData.WriteString("[settings.kubernetes.node-taints]\n")
		for _, taint := range constraints.Taints {
			userData.WriteString(fmt.Sprintf("\"%s\" = \"%s:%s\"\n", taint.Key, taint.Value, taint.Effect))
		}
	}
	return base64.StdEncoding.EncodeToString(userData.Bytes()), nil
}
