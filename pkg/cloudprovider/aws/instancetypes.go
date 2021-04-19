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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/utils"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	allInstanceTypesKey = "all"
)

type InstanceTypeProvider struct {
	ec2api ec2iface.EC2API
	cache  *cache.Cache
}

func NewInstanceTypeProvider(ec2api ec2iface.EC2API) *InstanceTypeProvider {
	return &InstanceTypeProvider{
		ec2api: ec2api,
		cache:  cache.New(CacheTTL, CacheCleanupInterval),
	}
}

// Get instance types that are available per availability zone
func (p *InstanceTypeProvider) Get(ctx context.Context, zonalSubnetOptions map[string][]*ec2.Subnet, constraints Constraints) ([]cloudprovider.InstanceType, error) {
	zones := []string{}
	for zone := range zonalSubnetOptions {
		zones = append(zones, zone)
	}

	var instanceTypes []*InstanceType
	if cached, ok := p.cache.Get(allInstanceTypesKey); ok {
		instanceTypes = cached.([]*InstanceType)
	} else {
		var err error
		instanceTypes, err = p.getZonalInstanceTypes(ctx)
		if err != nil {
			return nil, err
		}
		p.cache.SetDefault(allInstanceTypesKey, instanceTypes)
		zap.S().Debugf("Successfully discovered %d EC2 instance types", len(instanceTypes))
	}

	// Filter by constraints and zones and convert to cloudprovider interface
	constrainedInstanceTypes := []cloudprovider.InstanceType{}
	for _, instanceType := range p.filterFrom(instanceTypes, constraints, zones) {
		constrainedInstanceTypes = append(constrainedInstanceTypes, instanceType)
	}
	return constrainedInstanceTypes, nil
}

// GetAllInstanceTypeNames returns all instance type names without filtering based on constraints
func (p *InstanceTypeProvider) GetAllInstanceTypeNames(ctx context.Context) ([]string, error) {
	supportedInstanceTypes, err := p.Get(ctx, map[string][]*ec2.Subnet{}, Constraints{})
	if err != nil {
		return nil, err
	}
	instanceTypeNames := []string{}
	for _, instanceType := range supportedInstanceTypes {
		instanceTypeNames = append(instanceTypeNames, instanceType.Name())
	}
	return instanceTypeNames, nil
}

func (p *InstanceTypeProvider) getZonalInstanceTypes(ctx context.Context) ([]*InstanceType, error) {
	instanceTypes, err := p.getAllInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving all instance types, %w", err)
	}

	inputs := &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: aws.String("availability-zone"),
	}

	zonalInstanceTypeNames := map[string][]string{}
	err = p.ec2api.DescribeInstanceTypeOfferingsPagesWithContext(ctx, inputs, func(output *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
		for _, offerings := range output.InstanceTypeOfferings {
			zonalInstanceTypeNames[*offerings.Location] = append(zonalInstanceTypeNames[*offerings.Location], *offerings.InstanceType)
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("describing instance type zone offerings, %w", err)
	}

	// aggregate supported zones into each instance type
	ec2InstanceTypes := map[string]*InstanceType{}
	supportedInstanceTypes := []*InstanceType{}
	for _, instanceType := range instanceTypes {
		for zone, instanceTypeNames := range zonalInstanceTypeNames {
			for _, instanceTypeName := range instanceTypeNames {
				if instanceTypeName == *instanceType.InstanceType {
					if it, ok := ec2InstanceTypes[instanceTypeName]; ok {
						it.ZoneOptions = append(it.ZoneOptions, zone)
					} else {
						instanceType := InstanceType{InstanceTypeInfo: *instanceType, ZoneOptions: []string{zone}}
						supportedInstanceTypes = append(supportedInstanceTypes, &instanceType)
						ec2InstanceTypes[instanceTypeName] = &instanceType
					}
				}
			}
		}
	}
	return supportedInstanceTypes, nil
}

// getAllInstanceTypes retrieves all instance types from the ec2 DescribeInstanceTypes API using some opinionated filters
func (p *InstanceTypeProvider) getAllInstanceTypes(ctx context.Context) ([]*ec2.InstanceTypeInfo, error) {
	instanceTypes := []*ec2.InstanceTypeInfo{}
	describeInstanceTypesInput := &ec2.DescribeInstanceTypesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("supported-virtualization-type"),
				Values: []*string{aws.String("hvm")},
			},
		},
	}
	err := p.ec2api.DescribeInstanceTypesPagesWithContext(ctx, describeInstanceTypesInput, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
		instanceTypes = append(instanceTypes, page.InstanceTypes...)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("fetching instance types using ec2.DescribeInstanceTypes, %w", err)
	}
	return instanceTypes, nil
}

// filterFrom returns a filtered list of instance types based on the provided resource constraints
func (p *InstanceTypeProvider) filterFrom(instanceTypes []*InstanceType, constraints Constraints, zones []string) []*InstanceType {
	filtered := []*InstanceType{}
	for _, instanceType := range instanceTypes {
		requests := resources.RequestsForPods(constraints.Pods...)
		if p.validateInstanceType(constraints.InstanceTypes, instanceType) &&
			p.validateCapacityType(constraints.GetCapacityType(), instanceType) &&
			p.validateArchitecture(utils.NormalizeArchitecture(constraints.Architecture), instanceType) &&
			p.validateZones(zones, instanceType) &&
			p.validateNvidiaGPU(requests, instanceType) &&
			p.validateAWSNeuron(requests, instanceType) {
			filtered = append(filtered, instanceType)
		}
	}
	return filtered
}

func (p *InstanceTypeProvider) validateInstanceType(instanceTypeConstraints []string, instanceType *InstanceType) bool {
	if len(instanceTypeConstraints) == 0 && p.isDefaultInstanceType(instanceType) {
		return true
	}
	if len(instanceTypeConstraints) != 0 && functional.ContainsString(instanceTypeConstraints, instanceType.Name()) {
		return true
	}
	return false
}

// isDefaultInstanceType returns true if the instance type provided conforms to the default instance type criteria
// This function is used to make sure we launch instance types that are suited for general workloads
func (p *InstanceTypeProvider) isDefaultInstanceType(instanceType *InstanceType) bool {
	return instanceType.FpgaInfo == nil &&
		!*instanceType.BareMetal &&
		functional.HasAnyPrefix(instanceType.Name(),
			"m", "c", "r", "a", // Standard
			"t3", "t4", // Burstable
			"p", "inf", "g", // Accelerators
		)
}

func (p *InstanceTypeProvider) validateArchitecture(architecture *string, instanceType *InstanceType) bool {
	if architecture == nil {
		return true
	}
	return functional.ContainsString(aws.StringValueSlice(instanceType.ProcessorInfo.SupportedArchitectures), *architecture)
}

func (p *InstanceTypeProvider) validateCapacityType(capacityType string, instanceType *InstanceType) bool {
	if capacityType == "" {
		return true
	}
	return functional.ContainsString(aws.StringValueSlice(instanceType.SupportedUsageClasses), capacityType)
}

func (p *InstanceTypeProvider) validateNvidiaGPU(requests v1.ResourceList, instanceType *InstanceType) bool {
	if _, ok := requests[resources.NvidiaGPU]; !ok {
		return true
	}
	return !instanceType.NvidiaGPUs().IsZero()
}

func (p *InstanceTypeProvider) validateAWSNeuron(requests v1.ResourceList, instanceType *InstanceType) bool {
	if _, ok := requests[resources.AWSNeuron]; !ok {
		return true
	}
	return !instanceType.AWSNeurons().IsZero()
}

func (p *InstanceTypeProvider) validateZones(zones []string, instanceType *InstanceType) bool {
	if len(zones) == 0 {
		return true
	}
	return len(functional.IntersectStringSlice(instanceType.Zones(), zones)) > 0
}

type InstanceType struct {
	ec2.InstanceTypeInfo
	ZoneOptions []string
}

func (i InstanceType) Name() string {
	return *i.InstanceType
}
func (i InstanceType) Zones() []string {
	return i.ZoneOptions
}

func (i InstanceType) CPU() *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*i.VCpuInfo.DefaultVCpus))
}

func (i InstanceType) Memory() *resource.Quantity {
	return resources.Quantity(fmt.Sprintf("%dMi", *i.MemoryInfo.SizeInMiB))
}

func (i InstanceType) Pods() *resource.Quantity {
	// The number of pods per node is calculated using the formula:
	// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
	return resources.Quantity(fmt.Sprint(*i.NetworkInfo.MaximumNetworkInterfaces*(*i.NetworkInfo.Ipv4AddressesPerInterface-1) + 2))
}

func (i InstanceType) NvidiaGPUs() *resource.Quantity {
	count := int64(0)
	if i.GpuInfo != nil {
		for _, gpu := range i.GpuInfo.Gpus {
			if *i.GpuInfo.Gpus[0].Manufacturer == "NVIDIA" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i InstanceType) AWSNeurons() *resource.Quantity {
	count := int64(0)
	if i.InferenceAcceleratorInfo != nil {
		for _, accelerator := range i.InferenceAcceleratorInfo.Accelerators {
			count += *accelerator.Count
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}
