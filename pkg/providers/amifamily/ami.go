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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
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

	clk             clock.Clock
	cache           *cache.Cache
	ec2api          sdk.EC2API
	cm              *pretty.ChangeMonitor
	versionProvider version.Provider
	ssmProvider     ssm.Provider
}

func NewDefaultProvider(clk clock.Clock, versionProvider version.Provider, ssmProvider ssm.Provider, ec2api sdk.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		clk:             clk,
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
	if alias := nodeClass.Alias(); alias != nil {
		kubernetesVersion, err := p.versionProvider.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting kubernetes version, %w", err)
		}
		query, err := GetAMIFamily(alias.Family, nil).DescribeImageQuery(ctx, p.ssmProvider, kubernetesVersion, alias.Version)
		if err != nil {
			return []DescribeImageQuery{}, err
		}
		return []DescribeImageQuery{query}, nil
	}

	idFilter := ec2types.Filter{Name: aws.String("image-id")}
	queries := []DescribeImageQuery{}
	for _, term := range nodeClass.Spec.AMISelectorTerms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, term.ID)
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
				query.Filters = append(query.Filters, ec2types.Filter{
					Name:   aws.String("name"),
					Values: []string{term.Name},
				})

			}
			for k, v := range term.Tags {
				if v == "*" {
					query.Filters = append(query.Filters, ec2types.Filter{
						Name:   aws.String("tag-key"),
						Values: []string{k},
					})
				} else {
					query.Filters = append(query.Filters, ec2types.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []string{v},
					})
				}
			}
			queries = append(queries, query)
		}
	}
	if len(idFilter.Values) > 0 {
		queries = append(queries, DescribeImageQuery{Filters: []ec2types.Filter{idFilter}})
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
	for _, query := range queries {
		paginator := ec2.NewDescribeImagesPaginator(p.ec2api, query.DescribeImagesInput())
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("describing images, %w", err)
			}
			for _, image := range page.Images {
				arch, ok := v1.AWSToKubeArchitectures[string(image.Architecture)]
				if !ok {
					continue
				}
				// Each image may have multiple associated sets of requirements. For example, an image may be compatible with Neuron instances
				// and GPU instances. In that case, we'll have a set of requirements for each, and will create one "image" for each.
				for _, reqs := range query.RequirementsForImageWithArchitecture(lo.FromPtr(image.ImageId), arch) {
					// Checks and store for AMIs
					// Following checks are needed in order to always priortize non deprecated AMIs
					// If we already have an image with the same set of requirements, but this image (candidate) is newer, replace the previous (existing) image.
					// If we already have an image with the same set of requirements which is deprecated, but this image (candidate) is newer or non deprecated, replace the previous (existing) image
					reqsHash := lo.Must(hashstructure.Hash(reqs.NodeSelectorRequirements(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
					candidateDeprecated := parseTimeWithDefault(lo.FromPtr(image.DeprecationTime), maxTime).Unix() <= p.clk.Now().Unix()
					ami := AMI{
						Name:         lo.FromPtr(image.Name),
						AmiID:        lo.FromPtr(image.ImageId),
						CreationDate: lo.FromPtr(image.CreationDate),
						Deprecated:   candidateDeprecated,
						Requirements: reqs,
					}
					if v, ok := images[reqsHash]; ok {
						if cmpResult := compareAMI(v, ami); cmpResult <= 0 {
							continue
						}
					}
					images[reqsHash] = ami
				}
			}
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

// Compare two AMI's based on their deprecation status, creation time or name
// If both AMIs are deprecated, compare creation time and return the one with the newer creation time
// If both AMIs are non-deprecated, compare creation time and return the one with the newer creation time
// If one AMI is deprecated, return the non deprecated one
// The result will be
// 0 if AMI i == AMI j, where creation date, deprecation status and name are all equal
// -1 if AMI i < AMI j, if AMI i is non-deprecated or newer than AMI j
// +1 if AMI i > AMI j, if AMI j is non-deprecated or newer than AMI i
func compareAMI(i, j AMI) int {
	iCreationDate := parseTimeWithDefault(i.CreationDate, minTime)
	jCreationDate := parseTimeWithDefault(j.CreationDate, minTime)
	// Prioritize non-deprecated AMIs over deprecated ones
	if i.Deprecated != j.Deprecated {
		return lo.Ternary(i.Deprecated, 1, -1)
	}
	// If both are either non-deprecated or deprecated, compare by creation date
	if iCreationDate.Unix() != jCreationDate.Unix() {
		return lo.Ternary(iCreationDate.Unix() > jCreationDate.Unix(), -1, 1)
	}
	// If they have the same creation date, use the name as a tie-breaker
	if i.Name != j.Name {
		return lo.Ternary(i.Name > j.Name, -1, 1)
	}
	// If all attributes are are equal, both AMIs are exactly identical
	return 0
}
