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
	awscontext "github.com/aws/karpenter/pkg/context"
	"k8s.io/client-go/kubernetes"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type AMIProvider struct {
	ssmCache            *cache.Cache
	ec2Cache            *cache.Cache
	ssm                 ssmiface.SSMAPI
	kubeClient          client.Client
	ec2api              ec2iface.EC2API
	cm                  *pretty.ChangeMonitor
	kubernetesInterface kubernetes.Interface
	cache               *cache.Cache
}

type AMI struct {
	AmiID        string
	CreationDate string
}

const (
	kubernetesVersionCacheKey = "kubernetesVersion"
)

func NewAmiProvider(kubeClient client.Client, ssm ssmiface.SSMAPI, ec2api ec2iface.EC2API, ssmCache *cache.Cache, ec2Cache *cache.Cache, kubernetesInterface kubernetes.Interface) *AMIProvider {
	return &AMIProvider{
		ssm:                 ssm,
		ssmCache:            ssmCache,
		ec2Cache:            ec2Cache,
		kubeClient:          kubeClient,
		ec2api:              ec2api,
		cm:                  pretty.NewChangeMonitor(),
		kubernetesInterface: kubernetesInterface,
		cache:               cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval),
	}
}

func (p *AMIProvider) KubeServerVersion(ctx context.Context) (string, error) {
	if version, ok := p.cache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.cache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernete-version", version) {
		logging.FromContext(ctx).With("kubernete-version", version).Debugf("discovered kubernetes version")
	}
	return version, nil
}

func (p *AMIProvider) GetAMIsForProvider(ctx context.Context, providerRef *v1alpha5.ProviderRef, instanceTypes []*cloudprovider.InstanceType, amiFamilyName *string) ([]string, error) {
	amiFamily := GetAMIFamily(amiFamilyName, &Options{})
	kubernetesVersion, err := p.KubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}
	amiIds, err := p.Get(ctx, providerRef, kubernetesVersion, instanceTypes, amiFamily)
	if err != nil {
		return nil, fmt.Errorf("getting amiIds %w", err)
	}
	return lo.Keys(amiIds), nil
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
// If AMI overrides are specified in the AWSNodeTemplate, then only those AMIs will be chosen.
func (p *AMIProvider) Get(ctx context.Context, providerRef *v1alpha5.ProviderRef, kubenetesVersion string, instanceTypes []*cloudprovider.InstanceType, amiFamily AMIFamily) (map[string][]*cloudprovider.InstanceType, error) {
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	amiRequirements, err := p.getAMIRequirements(ctx, providerRef)
	if err != nil {
		return nil, err
	}
	if len(amiRequirements) > 0 {
		// Iterate through AMIs in order of creation date to use latest AMI
		amis := sortAMIsByCreationDate(amiRequirements)
		for _, instanceType := range instanceTypes {
			for _, ami := range amis {
				if err := instanceType.Requirements.Compatible(amiRequirements[ami]); err == nil {
					amiIDs[ami.AmiID] = append(amiIDs[ami.AmiID], instanceType)
					break
				}
			}
		}
		if len(amiIDs) == 0 {
			return nil, fmt.Errorf("no instance types satisfy requirements of amis %v,", lo.Keys(amiRequirements))
		}
	} else {
		for _, instanceType := range instanceTypes {
			amiID, err := p.getDefaultAMIFromSSM(ctx, amiFamily.SSMAlias(kubenetesVersion, instanceType))
			if err != nil {
				return nil, err
			}
			amiIDs[amiID] = append(amiIDs[amiID], instanceType)
		}
	}
	return amiIDs, nil
}

func (p *AMIProvider) getDefaultAMIFromSSM(ctx context.Context, ssmQuery string) (string, error) {
	if id, ok := p.ssmCache.Get(ssmQuery); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(ssmQuery)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", ssmQuery, err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.ssmCache.SetDefault(ssmQuery, ami)
	if p.cm.HasChanged("ssmquery-"+ssmQuery, ami) {
		logging.FromContext(ctx).With("ami", ami, "query", ssmQuery).Debugf("discovered new ami")
	}
	return ami, nil
}

func (p *AMIProvider) getAMIRequirements(ctx context.Context, providerRef *v1alpha5.ProviderRef) (map[AMI]scheduling.Requirements, error) {
	amiRequirements := map[AMI]scheduling.Requirements{}
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

func (p *AMIProvider) selectAMIs(ctx context.Context, amiSelector map[string]string) (map[AMI]scheduling.Requirements, error) {
	ec2AMIs, err := p.fetchAMIsFromEC2(ctx, amiSelector)
	if err != nil {
		return nil, err
	}
	if len(ec2AMIs) == 0 {
		return nil, fmt.Errorf("no amis exist given constraints")
	}
	var amiIDs = map[AMI]scheduling.Requirements{}
	for _, ec2AMI := range ec2AMIs {
		amiIDs[AMI{*ec2AMI.ImageId, *ec2AMI.CreationDate}] = p.getRequirementsFromImage(ec2AMI)
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
	if p.cm.HasChanged("amiIDs", amiIDs) {
		logging.FromContext(ctx).With("ami-ids", amiIDs).Debugf("discovered images")
	}
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

func sortAMIsByCreationDate(amiRequirements map[AMI]scheduling.Requirements) []AMI {
	amis := lo.Keys(amiRequirements)

	sort.Slice(amis, func(i, j int) bool {
		itime, _ := time.Parse(time.RFC3339, amis[i].CreationDate)
		jtime, _ := time.Parse(time.RFC3339, amis[j].CreationDate)
		return itime.Unix() >= jtime.Unix()
	})
	return amis
}

func (p *AMIProvider) getRequirementsFromImage(ec2Image *ec2.Image) scheduling.Requirements {
	requirements := scheduling.NewRequirements()
	for _, tag := range ec2Image.Tags {
		if v1alpha5.WellKnownLabels.Has(*tag.Key) {
			requirements.Add(scheduling.NewRequirement(*tag.Key, v1.NodeSelectorOpIn, *tag.Value))
		}
	}
	// Always add the architecture of an image as a requirement, irrespective of what's specified in EC2 tags.
	architecture := *ec2Image.Architecture
	if value, ok := v1alpha1.AWSToKubeArchitectures[architecture]; ok {
		architecture = value
	}
	requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, architecture))
	return requirements
}
