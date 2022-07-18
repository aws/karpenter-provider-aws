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

package amifamily

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/sets"
)

type AMIProvider struct {
	ssmCache   *cache.Cache
	ec2Cache   *cache.Cache
	ssm        ssmiface.SSMAPI
	kubeClient client.Client
	ec2api     ec2iface.EC2API
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
// If AMI overrides are specified in the AWSNodeTemplate, then only those AMIs will be chosen.
func (p *AMIProvider) Get(ctx context.Context, provider *awsv1alpha1.AWS, nodeRequest *cloudprovider.NodeRequest, options *Options, amiFamily AMIFamily) (map[string][]cloudprovider.InstanceType, error) {
	amiIDs := map[string][]cloudprovider.InstanceType{}
	amiRequirements, err := p.getAMIRequirements(ctx, nodeRequest.Template.ProviderRef)
	if err != nil {
		return nil, err
	}
	if len(amiRequirements) > 0 {
		for _, instanceType := range nodeRequest.InstanceTypeOptions {
			for amiID, requirements := range amiRequirements {
				if err := instanceType.Requirements().Compatible(requirements); err == nil {
					amiIDs[amiID] = append(amiIDs[amiID], instanceType)
				}
			}
		}
		if len(amiIDs) == 0 {
			return nil, fmt.Errorf("no instance types satisfy requirements of amis %v,", lo.Keys(amiRequirements))
		}
	} else {
		for _, instanceType := range nodeRequest.InstanceTypeOptions {
			amiID, err := p.getDefaultAMIFromSSM(ctx, instanceType, amiFamily.SSMAlias(options.KubernetesVersion, instanceType))
			if err != nil {
				return nil, err
			}
			amiIDs[amiID] = append(amiIDs[amiID], instanceType)
		}
	}
	return amiIDs, nil
}

func (p *AMIProvider) getDefaultAMIFromSSM(ctx context.Context, instanceType cloudprovider.InstanceType, ssmQuery string) (string, error) {
	if id, ok := p.ssmCache.Get(ssmQuery); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(ssmQuery)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", ssmQuery, err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.ssmCache.SetDefault(ssmQuery, ami)
	logging.FromContext(ctx).Debugf("Discovered %s for query %q", ami, ssmQuery)
	return ami, nil
}

func (p *AMIProvider) getAMIRequirements(ctx context.Context, providerRef *v1alpha5.ProviderRef) (map[string]scheduling.Requirements, error) {
	amiRequirements := map[string]scheduling.Requirements{}
	if providerRef != nil {
		var ant v1alpha1.AWSNodeTemplate
		if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: providerRef.Name}, &ant); err != nil {
			return amiRequirements, fmt.Errorf("retrieving provider reference, %w", err)
		}
		if len(ant.Spec.AMISelector) == 0 {
			return amiRequirements, nil
		}
		return p.selectAMIs(ctx, ant.Spec.AMISelector)
	}
	return amiRequirements, nil
}

func (p *AMIProvider) selectAMIs(ctx context.Context, amiSelector map[string]string) (map[string]scheduling.Requirements, error) {
	ec2AMIs, err := p.fetchAMIsFromEC2(ctx, amiSelector)
	if err != nil {
		return nil, err
	}
	if len(ec2AMIs) == 0 {
		return nil, fmt.Errorf("no amis exist given constraints")
	}
	var amiIDs = map[string]scheduling.Requirements{}
	for _, ec2AMI := range ec2AMIs {
		amiIDs[*ec2AMI.ImageId] = p.getRequirementsFromImage(ec2AMI)
	}
	return amiIDs, nil
}

func (p *AMIProvider) fetchAMIsFromEC2(ctx context.Context, amiSelector map[string]string) ([]*ec2.Image, error) {
	filters := getFilters(amiSelector)
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if amis, ok := p.ec2Cache.Get(fmt.Sprint(hash)); ok {
		return amis.([]*ec2.Image), nil
	}
	// This API is not paginated, so a single call suffices.
	output, err := p.ec2api.DescribeImagesWithContext(ctx, &ec2.DescribeImagesInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing images %+v, %w", filters, err)
	}
	p.ec2Cache.SetDefault(fmt.Sprint(hash), output.Images)
	amiIDs := lo.Map(output.Images, func(ami *ec2.Image, _ int) string { return *ami.ImageId })
	logging.FromContext(ctx).Debugf("Discovered images: %s", amiIDs)
	return output.Images, nil
}

func getFilters(amiSelector map[string]string) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range amiSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("image-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: []*string{aws.String(value)},
			})
		}
	}
	return filters
}

func (p *AMIProvider) getRequirementsFromImage(ec2Image *ec2.Image) scheduling.Requirements {
	requirements := scheduling.NewRequirements()
	for _, tag := range ec2Image.Tags {
		if v1alpha5.WellKnownLabels.Has(*tag.Key) {
			requirements.Add(scheduling.Requirements{*tag.Key: sets.NewSet(*tag.Value)})
		}
	}
	// Always add the architecture of an image as a requirement, irrespective of what's specified in EC2 tags.
	architecture := *ec2Image.Architecture
	if value, ok := awsv1alpha1.AWSToKubeArchitectures[architecture]; ok {
		architecture = value
	}
	requirements.Add(scheduling.Requirements{v1.LabelArchStable: sets.NewSet(architecture)})
	return requirements
}
