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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	v1alpha1 "github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/restconfig"
	"github.com/mitchellh/hashstructure/v2"
	"k8s.io/client-go/transport"
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
	return fmt.Sprintf(launchTemplateNameFormat, options.ClusterName, fmt.Sprint(hash))
}

// launchTemplateOptions is hashed and results in the creation of a real EC2
// LaunchTemplate. Do not change this struct without thinking through the impact
// to the number of LaunchTemplates that will result from this change.
type launchTemplateOptions struct {
	// Edge-triggered fields that will only change on kube events.
	ClusterName     string
	UserData        string
	InstanceProfile string
	// Level-triggered fields that may change out of sync.
	SecurityGroupsIds []string
	AMIID             string
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType) (string, error) {
	// 1. If the customer specified a launch template then just use it
	if constraints.LaunchTemplate != nil {
		return ptr.StringValue(constraints.LaunchTemplate), nil
	}

	// 2. Get constrained AMI ID
	amiID, err := p.amiProvider.Get(ctx, constraints, instanceTypes)
	if err != nil {
		return "", err
	}

	// 3. Get constrained security groups
	securityGroupsIds, err := p.securityGroupProvider.Get(ctx, constraints)
	if err != nil {
		return "", err
	}

	// 3. Get userData for Node
	userData, err := p.getUserData(ctx, constraints)
	if err != nil {
		return "", err
	}

	// 4. Ensure the launch template exists, or create it
	launchTemplate, err := p.ensureLaunchTemplate(ctx, &launchTemplateOptions{
		UserData:          userData,
		ClusterName:       constraints.Cluster.Name,
		InstanceProfile:   constraints.InstanceProfile,
		AMIID:             amiID,
		SecurityGroupsIds: securityGroupsIds,
	})
	if err != nil {
		return "", err
	}
	return aws.StringValue(launchTemplate.LaunchTemplateName), nil
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
				Name: aws.String(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", options.ClusterName)),
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("Karpenter/%s", options.ClusterName)),
					},
					{
						Key:   aws.String(fmt.Sprintf(ClusterTagKeyFormat, options.ClusterName)),
						Value: aws.String("owned"),
					},
					{
						Key:   aws.String(fmt.Sprintf(KarpenterTagKeyFormat, options.ClusterName)),
						Value: aws.String("owned"),
					},
				},
			}},
			SecurityGroupIds: aws.StringSlice(options.SecurityGroupsIds),
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

func (p *LaunchTemplateProvider) getUserData(ctx context.Context, constraints *v1alpha1.Constraints) (string, error) {
	var userData bytes.Buffer
	userData.WriteString(fmt.Sprintf(`#!/bin/bash
/etc/eks/bootstrap.sh '%s' \
    --container-runtime containerd \
    --apiserver-endpoint '%s'`,
		constraints.Cluster.Name,
		constraints.Cluster.Endpoint))
	caBundle, err := p.GetCABundle(ctx)
	if err != nil {
		return "", fmt.Errorf("getting ca bundle for user data, %w", err)
	}
	if caBundle != nil {
		userData.WriteString(fmt.Sprintf(` \
    --b64-cluster-ca '%s'`,
			*caBundle))
	}
	var nodeLabelArgs bytes.Buffer
	if len(constraints.Labels) > 0 {
		nodeLabelArgs.WriteString("--node-labels=")
		first := true
		for k, v := range constraints.Labels {
			if !first {
				nodeLabelArgs.WriteString(",")
			}
			first = false
			nodeLabelArgs.WriteString(fmt.Sprintf("%s=%v", k, v))
		}
	}
	var nodeTaintsArgs bytes.Buffer
	if len(constraints.Taints) > 0 {
		nodeTaintsArgs.WriteString("--register-with-taints=")
		first := true
		for _, taint := range constraints.Taints {
			if !first {
				nodeTaintsArgs.WriteString(",")
			}
			first = false
			nodeTaintsArgs.WriteString(fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	kubeletExtraArgs := strings.Trim(strings.Join([]string{nodeLabelArgs.String(), nodeTaintsArgs.String()}, " "), " ")
	if len(kubeletExtraArgs) > 0 {
		userData.WriteString(fmt.Sprintf(` \
    --kubelet-extra-args '%s'`, kubeletExtraArgs))
	}
	return base64.StdEncoding.EncodeToString(userData.Bytes()), nil
}

func (p *LaunchTemplateProvider) GetCABundle(ctx context.Context) (*string, error) {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	restConfig := restconfig.Get(ctx)
	if restConfig == nil {
		return nil, nil
	}
	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		logging.FromContext(ctx).Debugf("Unable to discover caBundle, loading transport config, %v", err)
		return nil, err
	}
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	if err != nil {
		logging.FromContext(ctx).Debugf("Unable to discover caBundle, loading TLS config, %v", err)
		return nil, err
	}
	logging.FromContext(ctx).Debugf("Discovered caBundle, length %d", len(transportConfig.TLS.CAData))
	return ptr.String(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}
