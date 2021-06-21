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
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/mitchellh/hashstructure/v2"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	launchTemplateNameFormat = "Karpenter-%s/%s/%s-%s"
	bottlerocketUserData     = `
[settings.kubernetes]
api-server = "{{.Cluster.Endpoint}}"
cluster-certificate = "{{.Cluster.CABundle}}"
cluster-name = "{{.Cluster.Name}}"
{{if .Labels }}[settings.kubernetes.node-labels]{{ end }}
{{ range $Key, $Value := .Labels }}"{{ $Key }}" = "{{ $Value }}"
{{ end }}
{{if .Taints }}[settings.kubernetes.node-taints]{{ end }}
{{ range $Taint := .Taints }}"{{ $Taint.Key }}" = "{{ $Taint.Value}}:{{ $Taint.Effect }}"
{{ end }}
`
)

type LaunchTemplateProvider struct {
	ec2api                ec2iface.EC2API
	cache                 *cache.Cache
	securityGroupProvider *SecurityGroupProvider
	ssm                   ssmiface.SSMAPI
	clientSet             *kubernetes.Clientset
}

func launchTemplateName(options *launchTemplateOptions) string {
	hash, err := hashstructure.Hash(options, hashstructure.FormatV2, nil)
	if err != nil {
		zap.S().Panicf("hashing launch template, %w", err)
	}
	return fmt.Sprintf(launchTemplateNameFormat, options.Cluster.Name, options.Provisioner.Name, options.Provisioner.Namespace, fmt.Sprint(hash))
}

// launchTemplateOptions is hashed and results in the creation of a real EC2
// LaunchTemplate. Do not change this struct without thinking through the impact
// to the number of LaunchTemplates that will result from this change.
type launchTemplateOptions struct {
	Provisioner  types.NamespacedName
	Cluster      v1alpha1.ClusterSpec
	Architecture string
	Labels       map[string]string
	Taints       []v1.Taint
}

func (p *LaunchTemplateProvider) Get(ctx context.Context, provisioner *v1alpha1.Provisioner, constraints *Constraints) (*LaunchTemplate, error) {
	// If the customer specified a launch template then just use it
	if result := constraints.GetLaunchTemplate(); result != nil {
		return result, nil
	}

	options := launchTemplateOptions{
		Provisioner:  types.NamespacedName{Name: provisioner.Name, Namespace: provisioner.Namespace},
		Cluster:      *provisioner.Spec.Cluster,
		Architecture: KubeToAWSArchitectures[*constraints.Architecture],
		Labels:       constraints.Labels,
		Taints:       constraints.Taints,
	}
	// See if we have a cached copy of the default one first, to avoid
	// making an API call to EC2
	key, err := hashstructure.Hash(options, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, fmt.Errorf("hashing launch template, %w", err)
	}

	result := &LaunchTemplate{Version: aws.String(DefaultLaunchTemplateVersion)}
	if cached, ok := p.cache.Get(fmt.Sprint(key)); ok {
		result.Id = cached.(*ec2.LaunchTemplate).LaunchTemplateId
		return result, nil
	}

	// Call EC2 to get launch template, creating if necessary
	launchTemplate, err := p.getLaunchTemplate(ctx, &options)
	if err != nil {
		return nil, err
	}
	result.Id = launchTemplate.LaunchTemplateId
	p.cache.Set(fmt.Sprint(key), launchTemplate, CacheTTL)
	return result, nil
}

// TODO, reconcile launch template if not equal to desired launch template (AMI upgrade, role changed, etc)
func (p *LaunchTemplateProvider) getLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	describelaunchTemplateOutput, err := p.ec2api.DescribeLaunchTemplatesWithContext(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(launchTemplateName(options))},
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidLaunchTemplateName.NotFoundException" {
		return p.createLaunchTemplate(ctx, options)
	}
	if err != nil {
		return nil, fmt.Errorf("describing launch templates, %w", err)
	}
	if length := len(describelaunchTemplateOutput.LaunchTemplates); length > 1 {
		return nil, fmt.Errorf("expected to find one launch template, but found %d", length)
	}
	launchTemplate := describelaunchTemplateOutput.LaunchTemplates[0]
	zap.S().Debugf("Successfully discovered launch template %s for %s/%s", *launchTemplate.LaunchTemplateName, options.Provisioner.Name, options.Provisioner.Namespace)
	return launchTemplate, nil
}

func (p *LaunchTemplateProvider) createLaunchTemplate(ctx context.Context, options *launchTemplateOptions) (*ec2.LaunchTemplate, error) {
	securityGroupIds, err := p.getSecurityGroupIds(ctx, options.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("getting security groups, %w", err)
	}
	amiID, err := p.getAMIID(ctx, options.Architecture)
	if err != nil {
		return nil, fmt.Errorf("getting AMI ID, %w", err)
	}
	zap.S().Debugf("Successfully discovered AMI ID %s for architecture %s", *amiID, options.Architecture)
	userData, err := p.getUserData(options)
	if err != nil {
		return nil, fmt.Errorf("getting user data, %w", err)
	}

	output, err := p.ec2api.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(launchTemplateName(options)),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", options.Cluster.Name)),
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(fmt.Sprintf(ClusterTagKeyFormat, options.Cluster.Name)),
						Value: aws.String("owned"),
					},
					{
						Key:   aws.String(fmt.Sprintf(KarpenterTagKeyFormat, options.Cluster.Name)),
						Value: aws.String("owned"),
					},
				},
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

func (p *LaunchTemplateProvider) getSecurityGroupIds(ctx context.Context, clusterName string) ([]*string, error) {
	securityGroupIds := []*string{}
	securityGroups, err := p.securityGroupProvider.Get(ctx, clusterName)
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

func (p *LaunchTemplateProvider) getUserData(options *launchTemplateOptions) (*string, error) {
	t := template.Must(template.New("userData").Parse(bottlerocketUserData))
	var userData bytes.Buffer
	if err := t.Execute(&userData, options); err != nil {
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
