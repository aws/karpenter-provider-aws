/*
Licensed under the Apache License, Version 2.0
*/

// Package baselineperformance resolves the
// karpenter.k8s.aws/instance-baseline-performance requirement label by calling
// the EC2 GetInstanceTypesFromInstanceRequirements API with the
// BaselinePerformance parameter.
package baselineperformance

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
)

// EC2BaselineAPI is the subset of the EC2 client used by this package.
type EC2BaselineAPI interface {
	GetInstanceTypesFromInstanceRequirements(
		ctx context.Context,
		input *ec2.GetInstanceTypesFromInstanceRequirementsInput,
		optFns ...func(*ec2.Options),
	) (*ec2.GetInstanceTypesFromInstanceRequirementsOutput, error)
}

// Filter resolves the LabelInstanceBaselineCPUPerformance requirement against
// the live EC2 API and maintains a per-family result cache.
type Filter struct {
	ec2api EC2BaselineAPI

	mu    sync.RWMutex
	cache map[string]sets.Set[string] // family -> set of compatible EC2 instance type names
}

// NewFilter constructs a Filter backed by the supplied EC2 client.
func NewFilter(api EC2BaselineAPI) *Filter {
	return &Filter{
		ec2api: api,
		cache:  make(map[string]sets.Set[string]),
	}
}

// InvalidateCache drops all cached results.
func (f *Filter) InvalidateCache() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = make(map[string]sets.Set[string])
}

// CompatibleInstanceTypes returns the set of EC2 instance type names whose CPU
// baseline performance is equivalent to every family listed in families.
// Multiple families are unioned (matching semantics of an "In" requirement).
func (f *Filter) CompatibleInstanceTypes(ctx context.Context, families []string) (sets.Set[string], error) {
	if len(families) == 0 {
		return nil, fmt.Errorf("at least one instance family must be specified")
	}
	result := sets.New[string]()
	for _, family := range families {
		compatible, err := f.compatibleForFamily(ctx, family)
		if err != nil {
			return nil, fmt.Errorf("baseline performance lookup for family %q: %w", family, err)
		}
		result = result.Union(compatible)
	}
	return result, nil
}

func (f *Filter) compatibleForFamily(ctx context.Context, family string) (sets.Set[string], error) {
	f.mu.RLock()
	if cached, ok := f.cache[family]; ok {
		f.mu.RUnlock()
		return cached, nil
	}
	f.mu.RUnlock()

	compatible, err := f.fetchFromEC2(ctx, family)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.cache[family] = compatible
	f.mu.Unlock()
	return compatible, nil
}

func (f *Filter) fetchFromEC2(ctx context.Context, family string) (sets.Set[string], error) {
	result := sets.New[string]()
	input := &ec2.GetInstanceTypesFromInstanceRequirementsInput{
		ArchitectureTypes: []ec2types.ArchitectureType{
			ec2types.ArchitectureTypeX8664,
			ec2types.ArchitectureTypeArm64,
		},
		VirtualizationTypes: []ec2types.VirtualizationType{ec2types.VirtualizationTypeHvm},
		InstanceRequirements: &ec2types.InstanceRequirementsRequest{
			BaselinePerformance: &ec2types.BaselinePerformanceFactorsRequest{
				Cpu: &ec2types.CpuPerformanceFactorRequest{
					References: []ec2types.PerformanceFactorReferenceRequest{
						{InstanceFamily: aws.String(family)},
					},
				},
			},
			VCpuCount: &ec2types.VCpuCountRangeRequest{Min: aws.Int32(1)},
			MemoryMiB: &ec2types.MemoryMiBRequest{Min: aws.Int32(1)},
		},
	}
	paginator := ec2.NewGetInstanceTypesFromInstanceRequirementsPaginator(f.ec2api, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("GetInstanceTypesFromInstanceRequirements (family=%s): %w", family, err)
		}
		for _, it := range page.InstanceTypes {
			if it.InstanceType != nil {
				result.Insert(string(*it.InstanceType))
			}
		}
	}
	return result, nil
}

// FilterInstanceTypes filters instance type names, retaining only those in the compatible set.
func (f *Filter) FilterInstanceTypes(ctx context.Context, families []string, instanceTypeNames []string) ([]string, error) {
	compatible, err := f.CompatibleInstanceTypes(ctx, families)
	if err != nil {
		return nil, err
	}
	return lo.Filter(instanceTypeNames, func(name string, _ int) bool {
		return compatible.Has(name)
	}), nil
}
