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
	"github.com/aws/karpenter/pkg/apis/v1beta1"

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
		logging.FromContext(ctx).With("version", version).Debugf("discovered kubernetes version")
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
func (p *Provider) Get(ctx context.Context, nodeClass *v1beta1.NodeClass, options *Options) ([]AMI, error) {
	var err error
	var amis []AMI
	if len(nodeClass.Spec.AMISelectorTerms) == 0 {
		amis, err = p.getDefaultAMIsFromSSM(ctx, nodeClass, options)
		if err != nil {
			return nil, err
		}
	} else {
		amis, err = p.getAMIs(ctx, nodeClass.Spec.AMISelectorTerms)
		if err != nil {
			return nil, err
		}
	}
	amis = groupAMIsByRequirements(SortAMIsByCreationDate(amis))
	if p.cm.HasChanged(fmt.Sprintf("amis/%t/%s", nodeClass.IsNodeTemplate, nodeClass.Name), amis) {
		logging.FromContext(ctx).With("ids", amiList(amis), "count", len(amis)).Debugf("discovered amis")
	}
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

func (p *Provider) getDefaultAMIsFromSSM(ctx context.Context, nodeClass *v1beta1.NodeClass, options *Options) ([]AMI, error) {
	var res []AMI

	amiFamily := GetAMIFamily(nodeClass.Spec.AMIFamily, options)
	kubernetesVersion, err := p.KubeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes version %w", err)
	}
	defaultAMIs := amiFamily.DefaultAMIs(kubernetesVersion)

	for _, ami := range defaultAMIs {
		if id, err := p.getAMIsFromSSM(ctx, ami.Query); err != nil {
			logging.FromContext(ctx).With("query", ami.Query).Errorf("discovering amis from ssm, %s", err)
		} else {
			res = append(res, AMI{AmiID: id, Requirements: ami.Requirements})
		}
	}
	images, err := p.getAMIs(ctx, lo.Map(res, func(a AMI, _ int) v1beta1.AMISelectorTerm {
		return v1beta1.AMISelectorTerm{
			ID: a.AmiID,
		}
	}))
	if err != nil {
		return nil, fmt.Errorf("discovering amis, %w", err)
	}
	// Resolve additional information from the set of default AMIs
	for i := range res {
		if image, ok := lo.Find(images, func(a AMI) bool {
			return res[i].AmiID == a.AmiID
		}); ok {
			res[i].Name = image.Name
			res[i].CreationDate = image.CreationDate
		}
	}
	return res, nil
}

func (p *Provider) getAMIsFromSSM(ctx context.Context, ssmQuery string) (string, error) {
	if id, ok := p.ssmCache.Get(ssmQuery); ok {
		return id.(string), nil
	}
	output, err := p.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(ssmQuery)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q, %w", ssmQuery, err)
	}
	ami := aws.StringValue(output.Parameter.Value)
	p.ssmCache.SetDefault(ssmQuery, ami)
	return ami, nil
}

func (p *Provider) getAMIs(ctx context.Context, terms []v1beta1.AMISelectorTerm) ([]AMI, error) {
	ec2AMIs, err := p.getAMIsFromEC2(ctx, terms)
	if err != nil {
		return nil, err
	}
	return lo.FilterMap(ec2AMIs, func(i *ec2.Image, _ int) (AMI, bool) {
		reqs := p.getRequirementsFromImage(i)
		return AMI{
			Name:         lo.FromPtr(i.Name),
			AmiID:        lo.FromPtr(i.ImageId),
			CreationDate: lo.FromPtr(i.CreationDate),
			Requirements: reqs,
		}, v1beta1.WellKnownArchitectures.Has(reqs.Get(v1.LabelArchStable).Any())
	}), nil
}

func (p *Provider) getAMIsFromEC2(ctx context.Context, terms []v1beta1.AMISelectorTerm) ([]*ec2.Image, error) {
	filterAndOwnerSets := GetFilterAndOwnerSets(terms)
	hash, err := hashstructure.Hash(filterAndOwnerSets, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if images, ok := p.ec2Cache.Get(fmt.Sprint(hash)); ok {
		return images.([]*ec2.Image), nil
	}
	images := map[string]*ec2.Image{}
	for _, filtersAndOwners := range filterAndOwnerSets {
		// This API is not paginated, so a single call suffices.
		output, err := p.ec2api.DescribeImagesWithContext(ctx, &ec2.DescribeImagesInput{
			// Don't include filters in the Describe Images call as EC2 API doesn't allow empty filters.
			Filters: lo.Ternary(len(filtersAndOwners.Filters) > 0, filtersAndOwners.Filters, nil),
			Owners:  lo.Ternary(len(filtersAndOwners.Owners) > 0, aws.StringSlice(filtersAndOwners.Owners), nil),
		})
		if err != nil {
			return nil, fmt.Errorf("describing images, %w", err)
		}
		for i := range output.Images {
			images[lo.FromPtr(output.Images[i].ImageId)] = output.Images[i]
		}
	}
	p.ec2Cache.SetDefault(fmt.Sprint(hash), lo.Values(images))
	return lo.Values(images), nil
}

func amiList(amis []AMI) string {
	var sb strings.Builder
	ids := lo.Map(amis, func(a AMI, _ int) string { return a.AmiID })
	if len(amis) > 25 {
		sb.WriteString(strings.Join(ids[:25], ", "))
		sb.WriteString(fmt.Sprintf(" and %d other(s)", len(amis)-25))
	} else {
		sb.WriteString(strings.Join(ids, ", "))
	}
	return sb.String()
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
				Owners: lo.Ternary(term.Owner != "", []string{term.Owner}, []string{"self", "amazon"}),
			}
			if term.Name != "" {
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
						Values: aws.StringSlice(functional.SplitCommaSeparatedString(v)),
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

// SortAMIsByCreationDate the AMIs are sorted by creation date in descending order.
// If creation date is nil or two AMIs have the same creation date, the AMIs will be sorted by name in ascending order.
func SortAMIsByCreationDate(amis []AMI) []AMI {
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
