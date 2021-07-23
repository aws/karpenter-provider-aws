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
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/mitchellh/hashstructure/v2"
	"knative.dev/pkg/ptr"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	launchTemplateNameFormat = "Karpenter-%s-%s"
	bottlerocketUserData     = `
[settings.kubernetes]
api-server = "{{.Cluster.Endpoint}}"
{{if .Cluster.CABundle}}{{if len .Cluster.CABundle}}cluster-certificate = "{{.Cluster.CABundle}}"{{end}}{{end}}
cluster-name = "{{if .Cluster.Name}}{{.Cluster.Name}}{{end}}"
{{if .Constraints.Labels }}[settings.kubernetes.node-labels]{{ end }}
{{ range $Key, $Value := .Constraints.Labels }}"{{ $Key }}" = "{{ $Value }}"
{{ end }}
{{if .Constraints.Taints }}[settings.kubernetes.node-taints]{{ end }}
{{ range $Taint := .Constraints.Taints }}"{{ $Taint.Key }}" = "{{ $Taint.Value}}:{{ $Taint.Effect }}"
{{ end }}
`
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
		zap.S().Panicf("hashing launch template, %w", err)
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

	// 4. Ensure the launch template exists, or create it
	launchTemplate, err := p.ensureLaunchTemplate(ctx, &launchTemplateOptions{
		Cluster:        provisioner.Spec.Cluster,
		UserData:       p.getUserData(provisioner, constraints),
		AMIID:          amiID,
		SecurityGroups: securityGroups,
	})
	if err != nil {
		return nil, err
	}
	return &LaunchTemplate{
		Id:      aws.StringValue(launchTemplate.LaunchTemplateId),
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
		zap.S().Debugf("Discovered launch template %s", name)
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
	zap.S().Debugf("Created launch template, %s", *output.LaunchTemplate.LaunchTemplateName)
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

func (p *LaunchTemplateProvider) getUserData(provisioner *v1alpha3.Provisioner, constraints *Constraints) string {
	t := template.Must(template.New("userData").Parse(bottlerocketUserData))
	var userData bytes.Buffer
	if err := t.Execute(&userData, struct {
		Constraints *Constraints
		Cluster     v1alpha3.Cluster
	}{constraints, provisioner.Spec.Cluster}); err != nil {
		panic(fmt.Sprintf("Parsing user data from %v, %v, %s", provisioner, constraints, err.Error()))
	}
	return base64.StdEncoding.EncodeToString(userData.Bytes())
}
