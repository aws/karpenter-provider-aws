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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type Provider interface {
	List(ctx context.Context, nodeClass *v1.EC2NodeClass) (AMIs, error)
}

type DefaultProvider struct {
	sync.Mutex
	cache           *cache.Cache
	ec2api          ec2iface.EC2API
	cm              *pretty.ChangeMonitor
	versionProvider version.Provider
	ssmProvider     ssm.Provider
}

func NewDefaultProvider(versionProvider version.Provider, ssmProvider ssm.Provider, ec2api ec2iface.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		cache:           cache,
		ec2api:          ec2api,
		cm:              pretty.NewChangeMonitor(),
		versionProvider: versionProvider,
		ssmProvider:     ssmProvider,
	}
}

// Get Returning a list of AMIs with its associated requirements
func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1.EC2NodeClass) (AMIs, error) {
	p.Lock()
	defer p.Unlock()
	queries, err := p.DescribeImageQueries(ctx, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("getting AMI queries, %w", err)
	}
	amis, err := p.amis(ctx, queries)
	if err != nil {
		return nil, err
	}
	amis.Sort()
	uniqueAMIs := lo.Uniq(lo.Map(amis, func(a AMI, _ int) string { return a.AmiID }))
	if p.cm.HasChanged(fmt.Sprintf("amis/%s", nodeClass.Name), uniqueAMIs) {
		log.FromContext(ctx).WithValues(
			"ids", uniqueAMIs).V(1).Info("discovered amis")
	}
	return amis, nil
}

func (p *DefaultProvider) DescribeImageQueries(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]DescribeImageQuery, error) {
	// Aliases are mutually exclusive, both on the term level and field level within a term.
	// This is enforced by a CEL validation, we will treat this as an invariant.
	if term, ok := lo.Find(nodeClass.Spec.AMISelectorTerms, func(term v1.AMISelectorTerm) bool {
		return term.Alias != ""
	}); ok {
		kubernetesVersion, err := p.versionProvider.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting kubernetes version, %w", err)
		}
		amiFamily := GetAMIFamily(v1.AMIFamilyFromAlias(term.Alias), nil)
		query, err := amiFamily.DescribeImageQuery(ctx, p.ssmProvider, kubernetesVersion, v1.AMIVersionFromAlias(term.Alias))
		if err != nil {
			return []DescribeImageQuery{}, err
		}
		return []DescribeImageQuery{query}, nil
	}

	idFilter := &ec2.Filter{Name: aws.String("image-id")}
	queries := []DescribeImageQuery{}
	for _, term := range nodeClass.Spec.AMISelectorTerms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, aws.String(term.ID))
		default:
			query := DescribeImageQuery{
				Owners: lo.Ternary(term.Owner != "", []string{term.Owner}, []string{}),
			}
			if term.Name != "" {
				// Default owners to self,amazon to ensure Karpenter only discovers cross-account AMIs if the user specifically allows it.
				// Removing this default would cause Karpenter to discover publicly shared AMIs passing the name filter.
				query = DescribeImageQuery{
					Owners: lo.Ternary(term.Owner != "", []string{term.Owner}, []string{"self", "amazon"}),
				}
				query.Filters = append(query.Filters, &ec2.Filter{
					Name:   aws.String("name"),
					Values: aws.StringSlice([]string{term.Name}),
				})

			}
			for k, v := range term.Tags {
				if v == "*" {
					query.Filters = append(query.Filters, &ec2.Filter{
						Name:   aws.String("tag-key"),
						Values: []*string{aws.String(k)},
					})
				} else {
					query.Filters = append(query.Filters, &ec2.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []*string{aws.String(v)},
					})
				}
			}
			queries = append(queries, query)
		}
	}
	if len(idFilter.Values) > 0 {
		queries = append(queries, DescribeImageQuery{Filters: []*ec2.Filter{idFilter}})
	}
	return queries, nil
}

//nolint:gocyclo
func (p *DefaultProvider) amis(ctx context.Context, queries []DescribeImageQuery) (AMIs, error) {
	hash, err := hashstructure.Hash(queries, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if images, ok := p.cache.Get(fmt.Sprintf("%d", hash)); ok {
		// Ensure what's returned from this function is a deep-copy of AMIs so alterations
		// to the data don't affect the original
		return append(AMIs{}, images.(AMIs)...), nil
	}
	images := map[uint64]AMI{}
	currentTime := time.Now().Unix()
	for _, query := range queries {
		if err = p.ec2api.DescribeImagesPagesWithContext(ctx, query.DescribeImagesInput(), func(page *ec2.DescribeImagesOutput, _ bool) bool {
			for _, image := range page.Images {
				arch, ok := v1.AWSToKubeArchitectures[lo.FromPtr(image.Architecture)]
				if !ok {
					continue
				}
				// Each image may have multiple associated sets of requirements. For example, an image may be compatible with Neuron instances
				// and GPU instances. In that case, we'll have a set of requirements for each, and will create one "image" for each.
				for _, reqs := range query.RequirementsForImageWithArchitecture(lo.FromPtr(image.ImageId), arch) {
					// Checks and store for AMIs
					// Following checks are needed in order to always priortize non deprecated AMIs
					// If we already have an image with the same set of requirements, but this image (candidate) is newer, replace the previous (existing) image.
					// If we already have an image with the same set of requirements which is deprecated, but this image (candidate) is newer and non deprecated, replace the previous (existing) image
					reqsHash := lo.Must(hashstructure.Hash(reqs.NodeSelectorRequirements(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
					if v, ok := images[reqsHash]; ok {
						candidateCreationTime := ParseTimeWithDefault(lo.FromPtr(image.CreationDate), MinTime)
						existingCreationTime := ParseTimeWithDefault(v.CreationDate, MinTime)
						candidateDeprecationTime := ParseTimeWithDefault(lo.FromPtr(image.DeprecationTime), MaxTime)
						existingDeprecationTime := ParseTimeWithDefault(v.DeprecationTime, MaxTime)
						if existingCreationTime == candidateCreationTime && lo.FromPtr(image.Name) < v.Name {
							continue
						}
						// If existing AMI is non deprecated, skip storing and returning the candidate AMI
						if existingDeprecationTime.Unix() >= currentTime && candidateDeprecationTime.Unix() <= currentTime {
							continue
						}
						// If both AMIs are non deprecated and the candidate AMI creation time is less than the existing AMI, skip storing and returning the candidate AMI
						if existingDeprecationTime.Unix() >= currentTime && candidateDeprecationTime.Unix() >= currentTime && candidateCreationTime.Unix() < existingCreationTime.Unix() {
							continue
						}
						// If both AMIs are deprecated and the candidate AMI creation time is less than the existing AMI, skip storing and returning the candidate AMI
						if existingDeprecationTime.Unix() <= currentTime && candidateDeprecationTime.Unix() <= currentTime && candidateCreationTime.Unix() < existingCreationTime.Unix() {
							continue
						}
					}
					images[reqsHash] = AMI{
						Name:            lo.FromPtr(image.Name),
						AmiID:           lo.FromPtr(image.ImageId),
						CreationDate:    lo.FromPtr(image.CreationDate),
						DeprecationTime: lo.FromPtr(image.DeprecationTime),
						Requirements:    reqs,
					}
				}
			}
			return true
		}); err != nil {
			return nil, fmt.Errorf("describing images, %w", err)
		}
	}
	p.cache.SetDefault(fmt.Sprintf("%d", hash), AMIs(lo.Values(images)))
	return lo.Values(images), nil
}

// MapToInstanceTypes returns a map of AMIIDs that are the most recent on creationDate to compatible instancetypes
func MapToInstanceTypes(instanceTypes []*cloudprovider.InstanceType, amis []v1.AMI) map[string][]*cloudprovider.InstanceType {
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		for _, ami := range amis {
			if err := instanceType.Requirements.Compatible(
				scheduling.NewNodeSelectorRequirements(ami.Requirements...),
				scheduling.AllowUndefinedWellKnownLabels,
			); err == nil {
				amiIDs[ami.ID] = append(amiIDs[ami.ID], instanceType)
				break
			}
		}
	}
	return amiIDs
}
