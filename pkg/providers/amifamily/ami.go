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
	"sort"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type Provider struct {
	ssmCache               *cache.Cache
	ec2Cache               *cache.Cache
	kubernetesVersionCache *cache.Cache
	ssm                    ssmiface.SSMAPI
	kubeClient             client.Client
	ec2api                 ec2iface.EC2API
	cm                     *pretty.ChangeMonitor
	kubernetesInterface    kubernetes.Interface
}

type AMI struct {
	AmiID        string
	CreationDate string
}

const (
	kubernetesVersionCacheKey = "kubernetesVersion"
)

func NewProvider(kubeClient client.Client, kubernetesInterface kubernetes.Interface, ssm ssmiface.SSMAPI, ec2api ec2iface.EC2API,
	ssmCache, ec2Cache, kubernetesVersionCache *cache.Cache) *Provider {
	return &Provider{
		ssmCache:               ssmCache,
		ec2Cache:               ec2Cache,
		kubernetesVersionCache: kubernetesVersionCache,
		ssm:                    ssm,
		kubeClient:             kubeClient,
		ec2api:                 ec2api,
		cm:                     pretty.NewChangeMonitor(),
		kubernetesInterface:    kubernetesInterface,
	}
}

func (p *Provider) KubeServerVersion(ctx context.Context) (string, error) {
	if version, ok := p.kubernetesVersionCache.Get(kubernetesVersionCacheKey); ok {
		return version.(string), nil
	}
	serverVersion, err := p.kubernetesInterface.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	version := fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))
	p.kubernetesVersionCache.SetDefault(kubernetesVersionCacheKey, version)
	if p.cm.HasChanged("kubernetes-version", version) {
		logging.FromContext(ctx).With("kubernetes-version", version).Debugf("discovered kubernetes version")
	}
	return version, nil
}

// Get returns a set of AMIIDs and corresponding instance types. AMI may vary due to architecture, accelerator, etc
// If AMI overrides are specified in the AWSNodeTemplate, then only those AMIs will be chosen.
func (p *Provider) Get(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, instanceTypes []*cloudprovider.InstanceType, amiFamily AMIFamily) (map[string][]*cloudprovider.InstanceType, error) {
	kubernetesVersion, err := p.KubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	amiRequirements, err := p.getAMIRequirements(ctx, nodeTemplate)
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
			amiID, err := p.getDefaultAMIFromSSM(ctx, amiFamily.SSMAlias(kubernetesVersion, instanceType))
			if err != nil {
				return nil, err
			}
			amiIDs[amiID] = append(amiIDs[amiID], instanceType)
		}
	}
	return amiIDs, nil
}

func (p *Provider) GetAMIWithRequirements(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, instanceTypes []*cloudprovider.InstanceType, amiFamily AMIFamily) (map[AMI]scheduling.Requirements, error) {
	kubernetesVersion, err := p.KubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	amiRequirements, err := p.getAMIRequirements(ctx, nodeTemplate)
	if err != nil {
		return nil, err
	}
	if len(amiRequirements) == 0 {
		for _, instanceType := range instanceTypes {
			amiID, err := p.getDefaultAMIFromSSM(ctx, amiFamily.SSMAlias(kubernetesVersion, instanceType))
			if err != nil {
				return nil, err
			}
			amiIDs[amiID] = append(amiIDs[amiID], instanceType)
		}
		amifilter := map[string]string{"aws-ids": strings.Join(lo.Keys(amiIDs), ",")}
		amiRequirements, err = p.selectAMIs(ctx, amifilter)
		if err != nil {
			return nil, err
		}
	}

	// Iterate through AMIs in order of creation date to use latest AMI
	amis := sortAMIsByCreationDate(amiRequirements)
	return lo.Associate(amis, func(ami AMI) (AMI, scheduling.Requirements) {
		return ami, amiRequirements[ami]
	}), nil
}

func (p *Provider) getDefaultAMIFromSSM(ctx context.Context, ssmQuery string) (string, error) {
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

func (p *Provider) getAMIRequirements(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (map[AMI]scheduling.Requirements, error) {
	if len(nodeTemplate.Spec.AMISelector) == 0 {
		return map[AMI]scheduling.Requirements{}, nil
	}
	return p.selectAMIs(ctx, nodeTemplate.Spec.AMISelector)
}

func (p *Provider) selectAMIs(ctx context.Context, amiSelector map[string]string) (map[AMI]scheduling.Requirements, error) {
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

func (p *Provider) fetchAMIsFromEC2(ctx context.Context, amiSelector map[string]string) ([]*ec2.Image, error) {
	filters, owners := getFiltersAndOwners(amiSelector)
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if amis, ok := p.ec2Cache.Get(fmt.Sprint(hash)); ok {
		return amis.([]*ec2.Image), nil
	}
	describeImagesInput := &ec2.DescribeImagesInput{Owners: owners}
	// Don't include filters in the Describe Images call as EC2 API doesn't allow empty filters.
	if len(filters) != 0 {
		describeImagesInput.Filters = filters
	}
	// This API is not paginated, so a single call suffices.
	output, err := p.ec2api.DescribeImagesWithContext(ctx, describeImagesInput)
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

func getFiltersAndOwners(amiSelector map[string]string) ([]*ec2.Filter, []*string) {
	filters := []*ec2.Filter{}
	var owners []*string
	imagesSet := false
	for key, value := range amiSelector {
		switch key {
		case "aws-ids", "aws::ids":
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("image-id"),
				Values: aws.StringSlice(filterValues),
			})
			imagesSet = true
		case "aws::owners":
			ownerValues := functional.SplitCommaSeparatedString(value)
			owners = aws.StringSlice(ownerValues)
		case "aws::name":
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("name"),
				Values: []*string{aws.String(value)},
			})
		default:
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: []*string{aws.String(value)},
			})
		}
	}
	if owners == nil && !imagesSet {
		owners = []*string{aws.String("self"), aws.String("amazon")}
	}

	return filters, owners
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

func (p *Provider) getRequirementsFromImage(ec2Image *ec2.Image) scheduling.Requirements {
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

func (p *Provider) Reset() {
	p.ssmCache.Flush()
	p.ec2Cache.Flush()
	p.kubernetesVersionCache.Flush()
}
