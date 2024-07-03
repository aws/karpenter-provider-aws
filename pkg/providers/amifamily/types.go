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
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

type AMI struct {
	Name         string
	AmiID        string
	CreationDate string
	Requirements scheduling.Requirements
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

type Variant string

var (
	VariantStandard    Variant = "standard"
	VariantNvidia      Variant = "nvidia"
	VariantNeuron      Variant = "neuron"
)

func NewVariant(v string) (Variant, error) {
	var wellKnownVariants = sets.New(VariantStandard, VariantNvidia, VariantNeuron)
	variant := Variant(v)
	if !wellKnownVariants.Has(variant) {
		return variant, fmt.Errorf("%q is not a well-known variant", variant)
	}
	return variant, nil
}

func (v Variant) Requirements() scheduling.Requirements {
	switch v {
	case VariantStandard:
		return scheduling.NewRequirements(
			scheduling.NewRequirement(v1.LabelInstanceAcceleratorCount, corev1.NodeSelectorOpDoesNotExist),
			scheduling.NewRequirement(v1.LabelInstanceGPUCount, corev1.NodeSelectorOpDoesNotExist),
		)
	case VariantNvidia:
		return scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelInstanceAcceleratorCount, corev1.NodeSelectorOpExists))
	case VariantNeuron:
		return scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelInstanceGPUCount, corev1.NodeSelectorOpExists))
	}
	return nil
}

type AMIQuery struct {
	Filters           []*ec2.Filter
	Owners            []string
	KnownRequirements map[string][]scheduling.Requirements
}

func (aq AMIQuery) DescribeImagesInput() *ec2.DescribeImagesInput {
	return &ec2.DescribeImagesInput{
		// Don't include filters in the Describe Images call as EC2 API doesn't allow empty filters.
		Filters:    lo.Ternary(len(aq.Filters) > 0, aq.Filters, nil),
		Owners:     lo.Ternary(len(aq.Owners) > 0, lo.ToSlicePtr(aq.Owners), nil),
		MaxResults: aws.Int64(1000),
	}
}
