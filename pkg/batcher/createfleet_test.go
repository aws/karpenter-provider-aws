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

package batcher

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/samber/lo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateFleet Batching", func() {
	var cfb *CreateFleetBatcher

	BeforeEach(func() {
		fakeEC2API.Reset()
		cfb = NewCreateFleetBatcher(ctx, fakeEC2API)
	})

	It("should batch the same inputs into a single call", func() {
		input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}
		var wg sync.WaitGroup
		var receivedInstance int64
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.CreateFleet(ctx, input)
				Expect(err).To(BeNil())
				instanceIds := lo.Flatten(lo.Map(rsp.Instances, func(rsv ec2types.CreateFleetInstance, _ int) []string {
					return rsv.InstanceIds
				}))
				atomic.AddInt64(&receivedInstance, 1)
				Expect(instanceIds).To(HaveLen(1))
			}()
		}
		wg.Wait()

		Expect(receivedInstance).To(BeNumerically("==", 5))
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(*call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 5))
	})
	It("should batch different inputs into multiple calls", func() {
		east1input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}
		east2input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-2"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}
		var wg sync.WaitGroup
		var receivedInstance int64
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(i int) {
				defer GinkgoRecover()
				defer wg.Done()
				input := east1input
				// 4 instances for us-east-1 and 1 instance in us-east-2
				if i == 3 {
					input = east2input
				}
				rsp, err := cfb.CreateFleet(ctx, input)
				Expect(err).To(BeNil())

				instanceIds := lo.Flatten(lo.Map(rsp.Instances, func(rsv ec2types.CreateFleetInstance, _ int) []string {
					return rsv.InstanceIds
				}))
				atomic.AddInt64(&receivedInstance, 1)
				Expect(instanceIds).To(HaveLen(1))
				time.Sleep(100 * time.Millisecond)
			}(i)
		}
		wg.Wait()

		Expect(receivedInstance).To(BeNumerically("==", 5))
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
		east2Call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		east1Call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		if *east2Call.TargetCapacitySpecification.TotalTargetCapacity > *east1Call.TargetCapacitySpecification.TotalTargetCapacity {
			east2Call, east1Call = east1Call, east2Call
		}
		Expect(*east2Call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 1))
		Expect(*east2Call.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone).To(Equal("us-east-2"))
		Expect(*east1Call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 4))
		Expect(*east1Call.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone).To(Equal("us-east-1"))
	})
	It("should return any errors to callers", func() {
		input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}

		fakeEC2API.CreateFleetBehavior.Output.Set(&ec2.CreateFleetOutput{
			Errors: []ec2types.CreateFleetError{
				{
					ErrorCode:    aws.String("some-error"),
					ErrorMessage: aws.String("some-error"),
					LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
						LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecification{
							LaunchTemplateName: aws.String("my-template"),
						},
						Overrides: &ec2types.FleetLaunchTemplateOverrides{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
				{
					ErrorCode:    aws.String("some-other-error"),
					ErrorMessage: aws.String("some-other-error"),
					LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
						LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecification{
							LaunchTemplateName: aws.String("my-template"),
						},
						Overrides: &ec2types.FleetLaunchTemplateOverrides{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			FleetId: aws.String("some-id"),
			Instances: []ec2types.CreateFleetInstance{
				{
					InstanceIds:                []string{"id-1", "id-2", "id-3", "id-4", "id-5"},
					InstanceType:               "",
					LaunchTemplateAndOverrides: nil,
					Lifecycle:                  "",
					Platform:                   "",
				},
			},
		})
		var wg sync.WaitGroup
		var receivedInstance int64
		var numErrors int64
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.CreateFleet(ctx, input)
				Expect(err).To(BeNil())

				if len(rsp.Errors) != 0 {
					// should receive errors for each caller
					atomic.AddInt64(&numErrors, 1)
				}

				instanceIds := lo.Flatten(lo.Map(rsp.Instances, func(rsv ec2types.CreateFleetInstance, _ int) []string {
					return rsv.InstanceIds
				}))
				atomic.AddInt64(&receivedInstance, 1)
				Expect(instanceIds).To(HaveLen(1))
			}()
		}
		wg.Wait()

		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		// requested 5 instances
		Expect(*call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 5))
		// but got three instances and two failures
		Expect(receivedInstance).To(BeNumerically("==", 5))
		Expect(numErrors).To(BeNumerically("==", 5))
	})
	It("should handle partial fulfillment", func() {
		input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}

		fakeEC2API.CreateFleetBehavior.Output.Set(&ec2.CreateFleetOutput{
			FleetId: aws.String("some-id2"),
			Instances: []ec2types.CreateFleetInstance{
				{
					InstanceIds:                []string{"id-1", "id-2"},
					InstanceType:               "",
					LaunchTemplateAndOverrides: nil,
					Lifecycle:                  "",
					Platform:                   "",
				},
			},
		})

		var wg sync.WaitGroup
		var receivedInstance int64
		var numErrors int64
		for i := 0; i < 7; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.CreateFleet(ctx, input)
				// partial fulfillment shouldn't cause an error at the CreateFleet call
				Expect(err).To(BeNil())

				if len(rsp.Errors) != 0 {
					atomic.AddInt64(&numErrors, 1)
				}

				instanceIds := lo.Flatten(lo.Map(rsp.Instances, func(rsv ec2types.CreateFleetInstance, _ int) []string {
					return rsv.InstanceIds
				}))
				Expect(instanceIds).To(Or(HaveLen(0), HaveLen(1)))
				if len(instanceIds) == 1 {
					atomic.AddInt64(&receivedInstance, 1)
				}
			}()
		}
		wg.Wait()

		// 3 Retries
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 3))
		call := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		// Last call for last 3
		Expect(*call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 3))
		call = fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		// Second call for remaining 5 instances
		Expect(*call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 5))
		call = fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		// First call for all 7 instances
		Expect(*call.TargetCapacitySpecification.TotalTargetCapacity).To(BeNumerically("==", 7))

		// Got 6 out of 7 instances due to request only returning 2 per call * 3 retries
		Expect(receivedInstance).To(BeNumerically("==", 6))

		// 1 error due to the last call failing to return all instances
		Expect(numErrors).To(BeNumerically("==", 1))
	})
	It("should retry failed createfleet requests up to 3 times", func() {
		input := &ec2.CreateFleetInput{
			LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
				{
					LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
						LaunchTemplateName: aws.String("my-template"),
					},
					Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
						{
							AvailabilityZone: aws.String("us-east-1"),
						},
					},
				},
			},
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				TotalTargetCapacity: aws.Int32(1),
			},
		}

		fakeEC2API.CreateFleetBehavior.Error.Set(fmt.Errorf("some error"), fake.MaxCalls(3))

		var numErrors int64
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.CreateFleet(ctx, input)
				Expect(err).To(BeNil())
				if len(rsp.Errors) != 0 {
					atomic.AddInt64(&numErrors, 1)
				}
			}()
		}
		wg.Wait()

		Expect(numErrors).To(BeNumerically("==", 2))
		Expect(fakeEC2API.CreateFleetBehavior.Calls()).To(BeNumerically("==", 3))
	})
})
