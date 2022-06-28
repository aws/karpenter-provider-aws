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
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/scheduling"
)

type AMIProvider struct {
	cache      *cache.Cache
	ssm        ssmiface.SSMAPI
	kubeClient client.Client
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
// If AMI overrides are specified in the AWSNodeTemplate, then only those AMIs will be chosen.
func (p *AMIProvider) Get(ctx context.Context, provider *awsv1alpha1.AWS, nodeRequest *cloudprovider.NodeRequest, options *Options, amiFamily AMIFamily) (map[string][]cloudprovider.InstanceType, error) {
	amiIDs := map[string][]cloudprovider.InstanceType{}
	amiRequirements, err := p.GetAMIRequirements(ctx, nodeRequest.Template.ProviderRef)
	if err != nil {
		return nil, err
	}
	var amiID string
	if len(amiRequirements) > 0 {
		for _, instanceType := range nodeRequest.InstanceTypeOptions {
			amiID = getAMIOverride(instanceType, amiRequirements)
			if amiID != "" {
				amiIDs[amiID] = append(amiIDs[amiID], instanceType)
			}
		}
		if len(amiIDs) == 0 {
			return nil, fmt.Errorf("no instance types satisfy requirements of amis %v,", Keys(amiRequirements))
		}
	} else {
		for _, instanceType := range nodeRequest.InstanceTypeOptions {
			amiID, err = p.GetDefaultAMIFromSSM(ctx, instanceType, amiFamily.SSMAlias(options.KubernetesVersion, instanceType))
			if err != nil {
				return nil, err
			}
			amiIDs[amiID] = append(amiIDs[amiID], instanceType)
		}
	}
	return amiIDs, nil
}

func (p *AMIProvider) GetDefaultAMIFromSSM(ctx context.Context, instanceType cloudprovider.InstanceType, ssmQuery string) (string, error) {
	if id, ok := p.cache.Get(ssmQuery); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(ssmQuery)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", ssmQuery, err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.cache.SetDefault(ssmQuery, ami)
	logging.FromContext(ctx).Debugf("Discovered %s for query %q", ami, ssmQuery)
	return ami, nil
}

func (p *AMIProvider) GetAMIRequirements(ctx context.Context, providerRef *v1alpha5.ProviderRef) (map[string]scheduling.Requirements, error) {
	amiRequirements := make(map[string]scheduling.Requirements)
	if providerRef != nil {
		var ant v1alpha1.AWSNodeTemplate
		if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: providerRef.Name}, &ant); err != nil {
			return amiRequirements, fmt.Errorf("retrieving provider reference, %w", err)
		}
		for _, ami := range ant.Spec.AMIs {
			amiRequirements[ami.ID] = scheduling.NewAMIRequirements(ami)
		}
	}
	return amiRequirements, nil
}

func getAMIOverride(instanceType cloudprovider.InstanceType, amiRequirements map[string]scheduling.Requirements) string {
	for amiID, requirements := range amiRequirements {
		if err := instanceType.Requirements().Compatible(requirements); err == nil {
			return amiID
		}
	}
	return ""
}

// Keys returns a slice of all the keys in a map
func Keys(m map[string]scheduling.Requirements) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
