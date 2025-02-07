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

package fake

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"

	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/atomic"
)

type CapacityPool struct {
	CapacityType string
	InstanceType string
	Zone         string
}

// EC2Behavior must be reset between tests otherwise tests will
// pollute each other.
type EC2Behavior struct {
	DescribeImagesOutput                AtomicPtr[ec2.DescribeImagesOutput]
	DescribeLaunchTemplatesOutput       AtomicPtr[ec2.DescribeLaunchTemplatesOutput]
	DescribeSubnetsOutput               AtomicPtr[ec2.DescribeSubnetsOutput]
	DescribeSecurityGroupsOutput        AtomicPtr[ec2.DescribeSecurityGroupsOutput]
	DescribeInstanceTypesOutput         AtomicPtr[ec2.DescribeInstanceTypesOutput]
	DescribeInstanceTypeOfferingsOutput AtomicPtr[ec2.DescribeInstanceTypeOfferingsOutput]
	DescribeAvailabilityZonesOutput     AtomicPtr[ec2.DescribeAvailabilityZonesOutput]
	DescribeSpotPriceHistoryInput       AtomicPtr[ec2.DescribeSpotPriceHistoryInput]
	DescribeSpotPriceHistoryOutput      AtomicPtr[ec2.DescribeSpotPriceHistoryOutput]
	CreateFleetBehavior                 MockedFunction[ec2.CreateFleetInput, ec2.CreateFleetOutput]
	TerminateInstancesBehavior          MockedFunction[ec2.TerminateInstancesInput, ec2.TerminateInstancesOutput]
	DescribeInstancesBehavior           MockedFunction[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput]
	CreateTagsBehavior                  MockedFunction[ec2.CreateTagsInput, ec2.CreateTagsOutput]
	CalledWithCreateLaunchTemplateInput AtomicPtrSlice[ec2.CreateLaunchTemplateInput]
	CalledWithDescribeImagesInput       AtomicPtrSlice[ec2.DescribeImagesInput]
	Instances                           sync.Map
	LaunchTemplates                     sync.Map
	InsufficientCapacityPools           atomic.Slice[CapacityPool]
	NextError                           AtomicError
}

type EC2API struct {
	sdk.EC2API
	EC2Behavior
}

func NewEC2API() *EC2API {
	return &EC2API{}
}

// DefaultSupportedUsageClasses is a var because []*string can't be a const
var DefaultSupportedUsageClasses = []ec2types.UsageClassType{ec2types.UsageClassType("on-demand"), ec2types.UsageClassType("spot")}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (e *EC2API) Reset() {
	e.DescribeImagesOutput.Reset()
	e.DescribeLaunchTemplatesOutput.Reset()
	e.DescribeSubnetsOutput.Reset()
	e.DescribeSecurityGroupsOutput.Reset()
	e.DescribeInstanceTypesOutput.Reset()
	e.DescribeInstanceTypeOfferingsOutput.Reset()
	e.DescribeAvailabilityZonesOutput.Reset()
	e.CreateFleetBehavior.Reset()
	e.TerminateInstancesBehavior.Reset()
	e.DescribeInstancesBehavior.Reset()
	e.CalledWithCreateLaunchTemplateInput.Reset()
	e.CalledWithDescribeImagesInput.Reset()
	e.DescribeSpotPriceHistoryInput.Reset()
	e.DescribeSpotPriceHistoryOutput.Reset()
	e.Instances.Range(func(k, v any) bool {
		e.Instances.Delete(k)
		return true
	})
	e.LaunchTemplates.Range(func(k, v any) bool {
		e.LaunchTemplates.Delete(k)
		return true
	})
	e.InsufficientCapacityPools.Reset()
	e.NextError.Reset()
}

// nolint: gocyclo
func (e *EC2API) CreateFleet(_ context.Context, input *ec2.CreateFleetInput, _ ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error) {
	return e.CreateFleetBehavior.Invoke(input, func(input *ec2.CreateFleetInput) (*ec2.CreateFleetOutput, error) {
		if input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName == nil {
			return nil, fmt.Errorf("missing launch template name")
		}
		var instanceIds []string
		var skippedPools []CapacityPool
		var spotInstanceRequestID *string

		if string(input.TargetCapacitySpecification.DefaultTargetCapacityType) == karpv1.CapacityTypeSpot {
			spotInstanceRequestID = aws.String(test.RandomName())
		}

		fulfilled := 0
		for _, ltc := range input.LaunchTemplateConfigs {
			for _, override := range ltc.Overrides {
				skipInstance := false
				e.InsufficientCapacityPools.Range(func(pool CapacityPool) bool {
					if pool.InstanceType == string(override.InstanceType) &&
						pool.Zone == aws.ToString(override.AvailabilityZone) &&
						pool.CapacityType == string(input.TargetCapacitySpecification.DefaultTargetCapacityType) {
						skippedPools = append(skippedPools, pool)
						skipInstance = true
						return false
					}
					return true
				})
				if skipInstance {
					continue
				}
				amiID := aws.String("")
				if e.CalledWithCreateLaunchTemplateInput.Len() > 0 {
					lt := e.CalledWithCreateLaunchTemplateInput.Pop()
					amiID = lt.LaunchTemplateData.ImageId
					e.CalledWithCreateLaunchTemplateInput.Add(lt)
				}
				instanceState := ec2types.InstanceStateNameRunning
				for ; fulfilled < int(*input.TargetCapacitySpecification.TotalTargetCapacity); fulfilled++ {
					instance := ec2types.Instance{
						ImageId:               aws.String(*amiID),
						InstanceId:            aws.String(test.RandomName()),
						Placement:             &ec2types.Placement{AvailabilityZone: input.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone},
						PrivateDnsName:        aws.String(randomdata.IpV4Address()),
						InstanceType:          input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
						SpotInstanceRequestId: spotInstanceRequestID,
						State: &ec2types.InstanceState{
							Name: instanceState,
						},
					}
					e.Instances.Store(*instance.InstanceId, instance)
					instanceIds = append(instanceIds, *instance.InstanceId)
				}
			}
			if fulfilled == int(*input.TargetCapacitySpecification.TotalTargetCapacity) {
				break
			}
		}
		result := &ec2.CreateFleetOutput{Instances: []ec2types.CreateFleetInstance{
			{
				InstanceIds:  instanceIds,
				InstanceType: input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
				Lifecycle:    ec2types.InstanceLifecycle(input.TargetCapacitySpecification.DefaultTargetCapacityType),
				LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
					Overrides: &ec2types.FleetLaunchTemplateOverrides{
						SubnetId:         input.LaunchTemplateConfigs[0].Overrides[0].SubnetId,
						ImageId:          input.LaunchTemplateConfigs[0].Overrides[0].ImageId,
						InstanceType:     input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
						AvailabilityZone: input.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone,
					},
				},
			},
		}}
		for _, pool := range skippedPools {
			result.Errors = append(result.Errors, ec2types.CreateFleetError{
				ErrorCode: aws.String("InsufficientInstanceCapacity"),
				LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
					Overrides: &ec2types.FleetLaunchTemplateOverrides{
						InstanceType:     ec2types.InstanceType(pool.InstanceType),
						AvailabilityZone: aws.String(pool.Zone),
					},
				},
			})
		}
		return result, nil
	})
}

func (e *EC2API) TerminateInstances(_ context.Context, input *ec2.TerminateInstancesInput, _ ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	return e.TerminateInstancesBehavior.Invoke(input, func(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
		var instanceStateChanges []ec2types.InstanceStateChange
		for _, id := range input.InstanceIds {
			if _, ok := e.Instances.LoadAndDelete(id); ok {
				instanceStateChanges = append(instanceStateChanges, ec2types.InstanceStateChange{
					PreviousState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
					CurrentState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameShuttingDown, Code: aws.Int32(32)},
					InstanceId:    aws.String(id),
				})
			}
		}
		return &ec2.TerminateInstancesOutput{TerminatingInstances: instanceStateChanges}, nil
	})
}

func (e *EC2API) CreateLaunchTemplate(_ context.Context, input *ec2.CreateLaunchTemplateInput, _ ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.CalledWithCreateLaunchTemplateInput.Add(input)
	launchTemplate := ec2types.LaunchTemplate{LaunchTemplateName: input.LaunchTemplateName}
	e.LaunchTemplates.Store(input.LaunchTemplateName, launchTemplate)
	return &ec2.CreateLaunchTemplateOutput{LaunchTemplate: lo.ToPtr(launchTemplate)}, nil
}

func (e *EC2API) CreateTags(_ context.Context, input *ec2.CreateTagsInput, _ ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return e.CreateTagsBehavior.Invoke(input, func(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
		// Update passed in instances with the passed tags
		for _, id := range input.Resources {
			raw, ok := e.Instances.Load(id)
			if !ok {
				return nil, fmt.Errorf("instance with id '%s' does not exist", id)
			}
			instance := raw.(ec2types.Instance)

			// Upsert any tags that have the same key
			tagsToMap := func(tag ec2types.Tag) (string, string) {
				return *tag.Key, *tag.Value
			}
			tags := lo.Assign(lo.SliceToMap(instance.Tags, tagsToMap), lo.SliceToMap(input.Tags, tagsToMap))
			instance.Tags = lo.MapToSlice(tags, func(key, value string) ec2types.Tag {
				return ec2types.Tag{Key: aws.String(key), Value: aws.String(value)}
			})
			e.Instances.Swap(lo.FromPtr(instance.InstanceId), instance)
		}
		return nil, nil
	})
}

func (e *EC2API) DescribeInstances(_ context.Context, input *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return e.DescribeInstancesBehavior.Invoke(input, func(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
		var instances []ec2types.Instance

		// If it's a list call and no instance ids are specified
		if len(input.InstanceIds) == 0 {
			e.Instances.Range(func(k interface{}, v interface{}) bool {
				instances = append(instances, v.(ec2types.Instance))
				return true
			})
		}
		for _, instanceID := range input.InstanceIds {
			instance, _ := e.Instances.Load(instanceID)
			if instance == nil {
				continue
			}
			instances = append(instances, instance.(ec2types.Instance))
		}
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{{Instances: filterInstances(instances, input.Filters)}},
		}, nil
	})
}

//nolint:gocyclo
func filterInstances(instances []ec2types.Instance, filters []ec2types.Filter) []ec2types.Instance {
	var ret []ec2types.Instance
	for _, instance := range instances {
		passesFilter := true
	OUTER:
		for _, filter := range filters {
			switch {
			case aws.ToString(filter.Name) == "instance-state-name":
				if !sets.New(filter.Values...).Has(string(instance.State.Name)) {
					passesFilter = false
					break OUTER
				}
			case aws.ToString(filter.Name) == "tag-key":
				values := sets.New(filter.Values...)
				if _, ok := lo.Find(instance.Tags, func(t ec2types.Tag) bool {
					return values.Has(aws.ToString(t.Key))
				}); !ok {
					passesFilter = false
					break OUTER
				}
			case strings.HasPrefix(aws.ToString(filter.Name), "tag:"):
				k := strings.TrimPrefix(aws.ToString(filter.Name), "tag:")
				tag, ok := lo.Find(instance.Tags, func(t ec2types.Tag) bool {
					return aws.ToString(t.Key) == k
				})
				if !ok {
					passesFilter = false
					break OUTER
				}
				switch {
				case lo.Contains(filter.Values, "*"):
				case lo.Contains(filter.Values, aws.ToString(tag.Value)):
				default:
					passesFilter = false
					break OUTER
				}
			}
		}
		if passesFilter {
			ret = append(ret, instance)
		}
	}
	return ret
}

func (e *EC2API) DescribeImages(_ context.Context, input *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.CalledWithDescribeImagesInput.Add(input)
	if !e.DescribeImagesOutput.IsNil() {
		describeImagesOutput := e.DescribeImagesOutput.Clone()
		describeImagesOutput.Images = FilterDescribeImages(describeImagesOutput.Images, input.Filters)
		return describeImagesOutput, nil
	}
	if input.Filters[0].Values[0] == "invalid" {
		return &ec2.DescribeImagesOutput{}, nil
	}
	return &ec2.DescribeImagesOutput{
		Images: []ec2types.Image{
			{
				Name:         aws.String(test.RandomName()),
				ImageId:      aws.String(test.RandomName()),
				CreationDate: aws.String(time.Now().Format(time.UnixDate)),
				Architecture: "x86_64",
				State:        ec2types.ImageStateAvailable,
			},
		},
	}, nil
}

func (e *EC2API) DescribeLaunchTemplates(_ context.Context, input *ec2.DescribeLaunchTemplatesInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeLaunchTemplatesOutput.IsNil() {
		return e.DescribeLaunchTemplatesOutput.Clone(), nil
	}
	output := &ec2.DescribeLaunchTemplatesOutput{}
	e.LaunchTemplates.Range(func(key, value interface{}) bool {
		launchTemplate := value.(ec2types.LaunchTemplate)
		if lo.Contains(aws.StringSlice(input.LaunchTemplateNames), launchTemplate.LaunchTemplateName) || len(input.Filters) != 0 && Filter(input.Filters, aws.ToString(launchTemplate.LaunchTemplateId), aws.ToString(launchTemplate.LaunchTemplateName), launchTemplate.Tags) {
			output.LaunchTemplates = append(output.LaunchTemplates, launchTemplate)
		}
		return true
	})
	if len(input.Filters) != 0 {
		return output, nil
	}
	if len(output.LaunchTemplates) == 0 {
		return nil, &smithy.GenericAPIError{
			Code:    "InvalidLaunchTemplateName.NotFoundException",
			Message: "At least one of the launch templates specified in the request does not exist.",
		}
	}
	return output, nil
}

func (e *EC2API) DeleteLaunchTemplate(_ context.Context, input *ec2.DeleteLaunchTemplateInput, _ ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.LaunchTemplates.Delete(input.LaunchTemplateName)
	return nil, nil
}

func (e *EC2API) DescribeSubnets(_ context.Context, input *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeSubnetsOutput.IsNil() {
		describeSubnetsOutput := e.DescribeSubnetsOutput.Clone()
		describeSubnetsOutput.Subnets = FilterDescribeSubnets(describeSubnetsOutput.Subnets, input.Filters)
		return describeSubnetsOutput, nil
	}
	subnets := []ec2types.Subnet{
		{
			SubnetId:                aws.String("subnet-test1"),
			AvailabilityZone:        aws.String("test-zone-1a"),
			AvailabilityZoneId:      aws.String("tstz1-1a"),
			AvailableIpAddressCount: aws.Int32(100),
			MapPublicIpOnLaunch:     aws.Bool(false),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-1")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			SubnetId:                aws.String("subnet-test2"),
			AvailabilityZone:        aws.String("test-zone-1b"),
			AvailabilityZoneId:      aws.String("tstz1-1b"),
			AvailableIpAddressCount: aws.Int32(100),
			MapPublicIpOnLaunch:     aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-2")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			SubnetId:                aws.String("subnet-test3"),
			AvailabilityZone:        aws.String("test-zone-1c"),
			AvailabilityZoneId:      aws.String("tstz1-1c"),
			AvailableIpAddressCount: aws.Int32(100),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-3")},
				{Key: aws.String("TestTag")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			SubnetId:                aws.String("subnet-test4"),
			AvailabilityZone:        aws.String("test-zone-1a-local"),
			AvailabilityZoneId:      aws.String("tstz1-1alocal"),
			AvailableIpAddressCount: aws.Int32(100),
			MapPublicIpOnLaunch:     aws.Bool(true),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-4")},
			},
		},
	}
	if len(input.Filters) == 0 {
		return nil, fmt.Errorf("InvalidParameterValue: The filter 'null' is invalid")
	}
	return &ec2.DescribeSubnetsOutput{Subnets: FilterDescribeSubnets(subnets, input.Filters)}, nil
}

func (e *EC2API) DescribeSecurityGroups(_ context.Context, input *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeSecurityGroupsOutput.IsNil() {
		describeSecurityGroupsOutput := e.DescribeSecurityGroupsOutput.Clone()
		describeSecurityGroupsOutput.SecurityGroups = FilterDescribeSecurtyGroups(describeSecurityGroupsOutput.SecurityGroups, input.Filters)
		return e.DescribeSecurityGroupsOutput.Clone(), nil
	}
	sgs := []ec2types.SecurityGroup{
		{
			GroupId:   aws.String("sg-test1"),
			GroupName: aws.String("securityGroup-test1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-1")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			GroupId:   aws.String("sg-test2"),
			GroupName: aws.String("securityGroup-test2"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-2")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			GroupId:   aws.String("sg-test3"),
			GroupName: aws.String("securityGroup-test3"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-3")},
				{Key: aws.String("TestTag")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
	}
	if len(input.Filters) == 0 {
		return nil, fmt.Errorf("InvalidParameterValue: The filter 'null' is invalid")
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: FilterDescribeSecurtyGroups(sgs, input.Filters)}, nil
}

func (e *EC2API) DescribeAvailabilityZones(context.Context, *ec2.DescribeAvailabilityZonesInput, ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeAvailabilityZonesOutput.IsNil() {
		return e.DescribeAvailabilityZonesOutput.Clone(), nil
	}
	return &ec2.DescribeAvailabilityZonesOutput{AvailabilityZones: []ec2types.AvailabilityZone{
		{ZoneName: aws.String("test-zone-1a"), ZoneId: aws.String("tstz1-1a"), ZoneType: aws.String("availability-zone")},
		{ZoneName: aws.String("test-zone-1b"), ZoneId: aws.String("tstz1-1b"), ZoneType: aws.String("availability-zone")},
		{ZoneName: aws.String("test-zone-1c"), ZoneId: aws.String("tstz1-1c"), ZoneType: aws.String("availability-zone")},
		{ZoneName: aws.String("test-zone-1a-local"), ZoneId: aws.String("tstz1-1alocal"), ZoneType: aws.String("local-zone")},
	}}, nil
}

func (e *EC2API) DescribeInstanceTypes(_ context.Context, _ *ec2.DescribeInstanceTypesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeInstanceTypesOutput.IsNil() {
		return e.DescribeInstanceTypesOutput.Clone(), nil
	}
	return defaultDescribeInstanceTypesOutput, nil
}

func (e *EC2API) DescribeInstanceTypeOfferings(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeInstanceTypeOfferingsOutput.IsNil() {
		return e.DescribeInstanceTypeOfferingsOutput.Clone(), nil
	}
	return &ec2.DescribeInstanceTypeOfferingsOutput{
		InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
			{
				InstanceType: "m5.large",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.large",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "m5.large",
				Location:     aws.String("test-zone-1c"),
			},
			{
				InstanceType: "m5.large",
				Location:     aws.String("test-zone-1a-local"),
			},
			{
				InstanceType: "m5.xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "m5.2xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.4xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.8xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "p3.8xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "p3.8xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "dl1.24xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "dl1.24xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "g4dn.8xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "g4dn.8xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "g4ad.16xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "g4ad.16xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "t3.large",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "t3.large",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "inf2.xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "inf2.24xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "trn1.2xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "c6g.large",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.metal",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m5.metal",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "m5.metal",
				Location:     aws.String("test-zone-1c"),
			},
			{
				InstanceType: "m6idn.32xlarge",
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: "m6idn.32xlarge",
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: "m6idn.32xlarge",
				Location:     aws.String("test-zone-1c"),
			},
		},
	}, nil
}

func (e *EC2API) DescribeSpotPriceHistory(_ context.Context, input *ec2.DescribeSpotPriceHistoryInput, _ ...func(*ec2.Options)) (*ec2.DescribeSpotPriceHistoryOutput, error) {
	e.DescribeSpotPriceHistoryInput.Set(input)
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeSpotPriceHistoryOutput.IsNil() {
		return e.DescribeSpotPriceHistoryOutput.Clone(), nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return nil, errors.New("no pricing data provided")
}
