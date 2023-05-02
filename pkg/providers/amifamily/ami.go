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
	"github.com/aws/karpenter-core/pkg/utils/sets"
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
	Name         string
	AmiID        string
	CreationDate string
	Requirements scheduling.Requirements
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

// MapInstanceTypes returns a map of AMIIDs that are the most recent on creationDate to compatible instancetypes
func MapInstanceTypes(amis []AMI, instanceTypes []*cloudprovider.InstanceType) map[string][]*cloudprovider.InstanceType {
	amiIDs := map[string][]*cloudprovider.InstanceType{}

	for _, instanceType := range instanceTypes {
		for _, ami := range amis {
			if err := instanceType.Requirements.Compatible(ami.Requirements); err == nil {
				amiIDs[ami.AmiID] = append(amiIDs[ami.AmiID], instanceType)
				break
			}
		}
	}
	return amiIDs
}

// Get Returning a list of AMIs with its associated requirements
func (p *Provider) Get(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, options *Options) ([]AMI, error) {
	var err error
	var amis []AMI
	if len(nodeTemplate.Spec.AMISelector) == 0 {
		amis, err = p.getDefaultAMIFromSSM(ctx, nodeTemplate, options)
		if err != nil {
			return nil, err
		}
	} else {
		amis, err = p.getAMIsFromSelector(ctx, nodeTemplate.Spec.AMISelector)
		if err != nil {
			return nil, err
		}
	}
	amis = sortAMIsByCreationDate(amis)
	amis = groupAMIsByRequirements(amis)
	return amis, nil
}

// groupAMIsByRequirements gets the most recent AMIs, by creation date, that have a unique set of requirements
func groupAMIsByRequirements(amis []AMI) []AMI {
	var result []AMI
	requirementsHash := sets.New[uint64]()
	for _, ami := range amis {
		hash := lo.Must(hashstructure.Hash(ami.Requirements.NodeSelectorRequirements(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
		if !requirementsHash.Has(hash) {
			result = append(result, ami)
		}
		requirementsHash.Insert(hash)
	}
	return result
}

func (p *Provider) getDefaultAMIFromSSM(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, options *Options) ([]AMI, error) {
	amiFamily := GetAMIFamily(nodeTemplate.Spec.AMIFamily, options)
	kubernetesVersion, err := p.KubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}

	var amis []AMI
	ssmRequirements := amiFamily.DefaultAMIs(kubernetesVersion)
	for _, ssmOutput := range ssmRequirements {
		amiID, err := p.fetchAMIsFromSSM(ctx, ssmOutput.Query)
		if err != nil {
			return nil, err
		}
		amis = append(amis, AMI{AmiID: amiID, Requirements: ssmOutput.Requirements})
	}

	amis, err = p.findAMINames(ctx, amis)
	if err != nil {
		return nil, err
	}

	return amis, nil
}

// Associate a list of amiIDs with there ec2 AMI names
func (p *Provider) findAMINames(ctx context.Context, amis []AMI) ([]AMI, error) {
	// Creating selector filter by making a string of amiIds into a comma delineated string
	ids := lo.Reduce(amis, func(agg string, item AMI, _ int) string {
		return agg + item.AmiID + ","
	}, "")
	selector := map[string]string{"aws-ids": ids}
	// Collecting the AMI details of the default AMIs from EC2
	amisDetails, err := p.fetchAMIsFromEC2(ctx, selector)
	if err != nil {
		return nil, err
	}

	// matching up the AMIs details that is received from EC2 with the default AMIs
	// collecting the names of the default AMIs
	amis = lo.Map(amis, func(x AMI, _ int) AMI {
		ami, ok := lo.Find(amisDetails, func(image *ec2.Image) bool {
			return *image.ImageId == x.AmiID
		})
		if ok {
			x.Name = *ami.Name
		}
		return x
	})

	return amis, nil
}

func (p *Provider) fetchAMIsFromSSM(ctx context.Context, ssmQuery string) (string, error) {
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
		logging.FromContext(ctx).With("ami", ami, "query", ssmQuery).Debugf("discovered ami")
	}
	return ami, nil
}

func (p *Provider) getAMIsFromSelector(ctx context.Context, selector map[string]string) (amis []AMI, err error) {
	ec2AMIs, err := p.fetchAMIsFromEC2(ctx, selector)
	if err != nil {
		return nil, err
	}
	for _, ec2AMI := range ec2AMIs {
		a := AMI{*ec2AMI.Name, *ec2AMI.ImageId, *ec2AMI.CreationDate, p.getRequirementsFromImage(ec2AMI)}
		// Only support well-known architectures for AMI resolution
		if v1alpha1.WellKnownArchitectures.Has(a.Requirements.Get(v1.LabelArchStable).Any()) {
			amis = append(amis, a)
		}
	}
	return amis, nil
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
		logging.FromContext(ctx).With("ids", concatenateAmiIDs(amiIDs), "count", len(amiIDs)).Debugf("discovered images")
	}
	return output.Images, nil
}

func concatenateAmiIDs(amisIds []string) string {
	var amiString string
	if len(amisIds) > 25 {
		amiString = strings.Join(amisIds[:25], ", ")
		amiString += fmt.Sprintf(" and %d other(s)", len(amisIds)-25)
	} else {
		amiString = strings.Join(amisIds, ", ")
	}
	return amiString
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

// sortAMIsByCreationDate the AMIs are sorted by creation date in descending order.
// If creation date is nil or two AMIs have the same creation date, the AMIs will be sorted by name in ascending order.
func sortAMIsByCreationDate(amis []AMI) []AMI {
	sort.Slice(amis, func(i, j int) bool {
		if amis[i].CreationDate != "" || amis[j].CreationDate != "" {
			itime, _ := time.Parse(time.RFC3339, amis[i].CreationDate)
			jtime, _ := time.Parse(time.RFC3339, amis[j].CreationDate)
			if itime.Unix() != jtime.Unix() {
				return itime.Unix() >= jtime.Unix()
			}
		}
		return amis[i].Name >= amis[j].Name
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
