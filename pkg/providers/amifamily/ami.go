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
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/option"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/utils/clock"

	"github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type Provider interface {
	// Resolves AMIs from a cache or EC2. This defaults to cache, use SkipCache to force an EC2 lookup.
	List(ctx context.Context, nodeClass *v1.EC2NodeClass, opts ...ListOptions) (AMIs, error)
}

// listOptions is named to avoid collision with Options, the static launch template parameters
type listOptions struct {
	SkipCache bool
}

type ListOptions = option.Function[listOptions]

var SkipCache = func(opts *listOptions) {
	opts.SkipCache = true
}

type DefaultProvider struct {
	sync.Mutex

	clk             clock.Clock
	cache           *cache.Cache
	ec2api          sdk.EC2API
	versionProvider version.Provider
	ssmProvider     ssm.Provider
}

func NewDefaultProvider(clk clock.Clock, versionProvider version.Provider, ssmProvider ssm.Provider, ec2api sdk.EC2API, cache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		clk:             clk,
		cache:           cache,
		ec2api:          ec2api,
		versionProvider: versionProvider,
		ssmProvider:     ssmProvider,
	}
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1.EC2NodeClass, opts ...ListOptions) (AMIs, error) {
	p.Lock()
	defer p.Unlock()
	amis, err := p.amis(ctx, nodeClass, option.Resolve(opts...).SkipCache)
	if err != nil {
		return nil, err
	}
	amis.Sort()
	return amis, nil
}

//nolint:gocyclo
func (p *DefaultProvider) DescribeImageQueries(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]DescribeImageQuery, error) {
	// Aliases are mutually exclusive, both on the term level and field level within a term.
	// This is enforced by a CEL validation, we will treat this as an invariant.
	if alias := nodeClass.Alias(); alias != nil {
		kubernetesVersion := p.versionProvider.Get(ctx)
		if alias.Family == v1.AMIFamilyAL2 {
			minorVersion, err := strconv.Atoi(strings.Split(kubernetesVersion, ".")[1])
			if err == nil && minorVersion >= 33 {
				return nil, &AL2DeprecationError{
					error: fmt.Errorf("AL2 aliases are no longer supported on EKS 1.33+ (see https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions-standard.html#kubernetes-1-33)"),
				}
			}
		}
		if alias.Family == v1.AMIFamilyWindows2025 {
			minorVersion, err := strconv.Atoi(strings.Split(kubernetesVersion, ".")[1])
			if err == nil && minorVersion < 35 {
				return nil, &WS2025UnsupportedVersionError{
					error: fmt.Errorf("Windows Server 2025 requires EKS version 1.35 or higher, current version: %s", kubernetesVersion),
				}
			}
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
		case term.SSMParameter != "":
			imageID, err := p.ssmProvider.Get(ctx, ssm.Parameter{
				Name: term.SSMParameter,
				Type: ssm.CustomParameterType,
			})
			if err != nil {
				if !errors.IsNotFound(err) {
					return []DescribeImageQuery{}, fmt.Errorf("resolving ssm parameter, %w", err)
				}
				log.FromContext(ctx).WithValues("ssmParameter", term.SSMParameter).V(1).Error(err, "parameter not found")
				continue
			}
			if !strings.HasPrefix(imageID, "ami-") {
				log.FromContext(ctx).WithValues("ssmParameter", term.SSMParameter, "id", imageID).V(1).Error(nil, "parameter value is an invalid AMI ID")
				continue
			}
			idFilter.Values = append(idFilter.Values, imageID)
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
func (p *DefaultProvider) amis(ctx context.Context, nodeClass *v1.EC2NodeClass, skipCache bool) (AMIs, error) {
	queries, err := p.DescribeImageQueries(ctx, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("getting AMI queries, %w", err)
	}
	// Each query is cached independently (one DescribeImages call), so NodeClasses
	// resolving to the same query share the cache entry (see #9063).
	images := map[uint64]AMI{}
	for _, query := range queries {
		key := imageQueryCacheKey(query)
		if !skipCache {
			if cached, ok := p.cache.Get(key); ok {
				mergeAMIs(images, cached.(AMIs))
				continue
			}
		}
		queryImages := map[uint64]AMI{}
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
					if v, ok := queryImages[reqsHash]; ok {
						if cmpResult := compareAMI(v, ami); cmpResult <= 0 {
							continue
						}
					}
					queryImages[reqsHash] = ami
				}
			}
		}
		queryAMIs := AMIs(lo.Values(queryImages))
		p.cache.SetDefault(key, queryAMIs)
		mergeAMIs(images, queryAMIs)
	}
	return lo.Values(images), nil
}

// mergeAMIs merges amis into the accumulator keyed by requirements hash, keeping the
// preferred AMI (see compareAMI) when multiple AMIs share a set of requirements.
func mergeAMIs(into map[uint64]AMI, amis AMIs) {
	for i := range amis {
		reqsHash := lo.Must(hashstructure.Hash(amis[i].Requirements.NodeSelectorRequirements(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
		if v, ok := into[reqsHash]; ok {
			if cmpResult := compareAMI(v, amis[i]); cmpResult <= 0 {
				continue
			}
		}
		into[reqsHash] = amis[i]
	}
}

// imageQueryCacheKey keys a DescribeImages query on its owners, filters, and known requirements.
// Sharing an entry between queries that differ only by owner would surface AMIs from an
// account the user never authorized. Known requirements (e.g. from an alias's SSM parameters)
// are baked into the cached AMIs, so an alias query must not share an entry with an ID query
// for the same image.
func imageQueryCacheKey(query DescribeImageQuery) string {
	ownersHash := lo.Must(hashstructure.Hash(lo.Uniq(query.Owners), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	// scheduling.Requirements has no exported fields, so hash its NodeSelectorRequirements instead
	knownRequirements := lo.MapValues(query.KnownRequirements, func(reqs []scheduling.Requirements, _ string) [][]karpv1.NodeSelectorRequirementWithMinValues {
		return lo.Map(reqs, func(r scheduling.Requirements, _ int) []karpv1.NodeSelectorRequirementWithMinValues {
			return r.NodeSelectorRequirements()
		})
	})
	knownRequirementsHash := lo.Must(hashstructure.Hash(knownRequirements, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	return fmt.Sprintf("%016x-%016x-%016x", ownersHash, utils.FilterSetHash(query.Filters), knownRequirementsHash)
}

// MapToInstanceTypes returns a map of AMIIDs that are the most recent on creationDate to compatible instancetypes
func MapToInstanceTypes(instanceTypes []*cloudprovider.InstanceType, amis []v1.AMI) map[string][]*cloudprovider.InstanceType {
	amiIDs := map[string][]*cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		for _, ami := range amis {
			if err := instanceType.Requirements.Compatible(
				scheduling.NewNodeSelectorRequirements(ami.Requirements...),
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
