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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Provider interface {
	List(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (AMIs, error)
}

type DefaultProvider struct {
	sync.Mutex
	cache           *cache.Cache
	ec2api          ec2iface.EC2API
	cm              *pretty.ChangeMonitor
	ssmProvider     ssm.Provider
	versionProvider version.Provider
	clk             clock.Clock
}

type AMI struct {
	Name         string
	AmiID        string
	CreationDate string
	Requirements scheduling.Requirements
	Deprecated   bool
}

type AMIs []AMI

// Sort orders the AMIs by creation date in descending order.
// If creation date is nil or two AMIs have the same creation date, the AMIs will be sorted by ID, which is guaranteed to be unique, in ascending order.
func (a AMIs) Sort() {
	sort.Slice(a, func(i, j int) bool {
		itime, _ := time.Parse(time.RFC3339, a[i].CreationDate)
		jtime, _ := time.Parse(time.RFC3339, a[j].CreationDate)
		if itime.Unix() != jtime.Unix() {
			return itime.Unix() > jtime.Unix()
		}
		return a[i].AmiID < a[j].AmiID
	})
}

// MapToInstanceTypes returns a map of AMIIDs that are the most recent on creationDate to compatible instancetypes
func MapToInstanceTypes(instanceTypes []*cloudprovider.InstanceType, amis []v1beta1.AMI) map[string][]*cloudprovider.InstanceType {
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		for _, ami := range amis {
			if err := instanceType.Requirements.Compatible(scheduling.NewNodeSelectorRequirements(ami.Requirements...), scheduling.AllowUndefinedWellKnownLabels); err == nil {
				amiIDs[ami.ID] = append(amiIDs[ami.ID], instanceType)
				break
			}
		}
	}
	return amiIDs
}

func NewDefaultProvider(clock clock.Clock, versionProvider version.Provider, ssmProvider ssm.Provider, ec2api ec2iface.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		cache:           cache,
		ec2api:          ec2api,
		cm:              pretty.NewChangeMonitor(),
		ssmProvider:     ssmProvider,
		versionProvider: versionProvider,
		clk:             clock,
	}
}

// Get Returning a list of AMIs with its associated requirements
func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (AMIs, error) {
	p.Lock()
	defer p.Unlock()

	var err error
	var amis AMIs
	if len(nodeClass.Spec.AMISelectorTerms) == 0 {
		amis, err = p.getDefaultAMIs(ctx, nodeClass)
		if err != nil {
			return nil, err
		}
	} else {
		amis, err = p.getAMIs(ctx, nodeClass)
		if err != nil {
			return nil, err
		}
	}
	amis.Sort()
	uniqueAMIs := lo.Uniq(lo.Map(amis, func(a AMI, _ int) string { return a.AmiID }))
	if p.cm.HasChanged(fmt.Sprintf("amis/%s", nodeClass.Name), uniqueAMIs) {
		log.FromContext(ctx).WithValues(
			"ids", uniqueAMIs).V(1).Info("discovered amis")
	}
	return amis, nil
}

func (p *DefaultProvider) getDefaultAMIs(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (res AMIs, err error) {
	hash := utils.GetNodeClassHash(nodeClass)
	if images, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a deep-copy of AMIs so alterations
		// to the data don't affect the original
		return append(AMIs{}, images.(AMIs)...), nil
	}
	amiFamily := GetAMIFamily(nodeClass.Spec.AMIFamily, &Options{})
	kubernetesVersion, err := p.versionProvider.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}
	defaultAMIs := amiFamily.DefaultAMIs(kubernetesVersion)
	for _, ami := range defaultAMIs {
		if id, err := p.resolveSSMParameter(ctx, ami.Query); err != nil {
			log.FromContext(ctx).WithValues("query", ami.Query).Error(err, "failed discovering amis from ssm")
		} else {
			res = append(res, AMI{AmiID: id, Requirements: ami.Requirements})
		}
	}
	// Resolve Name and CreationDate information into the DefaultAMIs
	if err = p.ec2api.DescribeImagesPagesWithContext(ctx, &ec2.DescribeImagesInput{
		Filters:           []*ec2.Filter{{Name: aws.String("image-id"), Values: aws.StringSlice(lo.Map(res, func(a AMI, _ int) string { return a.AmiID }))}},
		MaxResults:        aws.Int64(500),
		IncludeDeprecated: lo.ToPtr(true),
	}, func(page *ec2.DescribeImagesOutput, _ bool) bool {
		for i := range page.Images {
			for j := range res {
				if res[j].AmiID == aws.StringValue(page.Images[i].ImageId) {
					res[j].Name = aws.StringValue(page.Images[i].Name)
					res[j].CreationDate = aws.StringValue(page.Images[i].CreationDate)
					res[j].Deprecated = p.IsDeprecated(page.Images[i])
				}
			}
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("describing images, %w", err)
	}
	p.cache.SetDefault(hash, res)
	return res, nil
}

func (p *DefaultProvider) resolveSSMParameter(ctx context.Context, name string) (string, error) {
	imageID, err := p.ssmProvider.Get(ctx, ssm.Parameter{
		Name:      name,
		IsMutable: true,
	})
	if err != nil {
		return "", err
	}
	return imageID, nil
}

//nolint:gocyclo
func (p *DefaultProvider) getAMIs(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (AMIs, error) {
	filterAndOwnerSets := GetFilterAndOwnerSets(nodeClass.Spec.AMISelectorTerms)
	hash := utils.GetNodeClassHash(nodeClass)
	if images, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a deep-copy of AMIs so alterations
		// to the data don't affect the original
		return append(AMIs{}, images.(AMIs)...), nil
	}
	images := map[uint64]AMI{}
	for _, filtersAndOwners := range filterAndOwnerSets {
		if err := p.ec2api.DescribeImagesPagesWithContext(ctx, &ec2.DescribeImagesInput{
			// Don't include filters in the Describe Images call as EC2 API doesn't allow empty filters.
			Filters:    lo.Ternary(len(filtersAndOwners.Filters) > 0, filtersAndOwners.Filters, nil),
			Owners:     lo.Ternary(len(filtersAndOwners.Owners) > 0, aws.StringSlice(filtersAndOwners.Owners), nil),
			MaxResults: aws.Int64(1000),
		}, func(page *ec2.DescribeImagesOutput, _ bool) bool {
			for i := range page.Images {
				reqs := p.getRequirementsFromImage(page.Images[i])
				if !v1beta1.WellKnownArchitectures.Has(reqs.Get(v1.LabelArchStable).Any()) {
					continue
				}
				reqsHash := lo.Must(hashstructure.Hash(reqs.NodeSelectorRequirements(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
				// If the proposed image is newer, store it so that we can return it
				if v, ok := images[reqsHash]; ok {
					candidateCreationTime, _ := time.Parse(time.RFC3339, lo.FromPtr(page.Images[i].CreationDate))
					existingCreationTime, _ := time.Parse(time.RFC3339, v.CreationDate)
					if existingCreationTime.Equal(candidateCreationTime) && lo.FromPtr(page.Images[i].Name) < v.Name {
						continue
					}
					if candidateCreationTime.Unix() < existingCreationTime.Unix() {
						continue
					}
				}
				images[reqsHash] = AMI{
					Name:         lo.FromPtr(page.Images[i].Name),
					AmiID:        lo.FromPtr(page.Images[i].ImageId),
					CreationDate: lo.FromPtr(page.Images[i].CreationDate),
					Requirements: reqs,
					Deprecated:   p.IsDeprecated(page.Images[i]),
				}
			}
			return true
		}); err != nil {
			return nil, fmt.Errorf("describing images, %w", err)
		}
	}
	p.cache.SetDefault(hash, AMIs(lo.Values(images)))
	return lo.Values(images), nil
}

type FiltersAndOwners struct {
	Filters []*ec2.Filter
	Owners  []string
}

func GetFilterAndOwnerSets(terms []v1beta1.AMISelectorTerm) (res []FiltersAndOwners) {
	idFilter := &ec2.Filter{Name: aws.String("image-id")}
	for _, term := range terms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, aws.String(term.ID))
		default:
			elem := FiltersAndOwners{
				Owners: lo.Ternary(term.Owner != "", []string{term.Owner}, []string{}),
			}
			if term.Name != "" {
				// Default owners to self,amazon to ensure Karpenter only discovers cross-account AMIs if the user specifically allows it.
				// Removing this default would cause Karpenter to discover publicly shared AMIs passing the name filter.
				elem = FiltersAndOwners{
					Owners: lo.Ternary(term.Owner != "", []string{term.Owner}, []string{"self", "amazon"}),
				}
				elem.Filters = append(elem.Filters, &ec2.Filter{
					Name:   aws.String("name"),
					Values: aws.StringSlice([]string{term.Name}),
				})

			}
			for k, v := range term.Tags {
				if v == "*" {
					elem.Filters = append(elem.Filters, &ec2.Filter{
						Name:   aws.String("tag-key"),
						Values: []*string{aws.String(k)},
					})
				} else {
					elem.Filters = append(elem.Filters, &ec2.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []*string{aws.String(v)},
					})
				}
			}
			res = append(res, elem)
		}
	}
	if len(idFilter.Values) > 0 {
		res = append(res, FiltersAndOwners{Filters: []*ec2.Filter{idFilter}})
	}
	return res
}

func (p *DefaultProvider) getRequirementsFromImage(ec2Image *ec2.Image) scheduling.Requirements {
	requirements := scheduling.NewRequirements()
	// Always add the architecture of an image as a requirement, irrespective of what's specified in EC2 tags.
	architecture := *ec2Image.Architecture
	if value, ok := v1beta1.AWSToKubeArchitectures[architecture]; ok {
		architecture = value
	}
	requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, architecture))
	return requirements
}

func (p *DefaultProvider) IsDeprecated(image *ec2.Image) bool {
	if image.DeprecationTime == nil {
		return false
	}
	if deprecationTime := lo.Must(time.Parse(time.RFC3339, *image.DeprecationTime)); deprecationTime.After(p.clk.Now()) {
		return false
	}
	return true
}
