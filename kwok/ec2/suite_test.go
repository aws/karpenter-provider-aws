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

package ec2

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	clock "k8s.io/utils/clock/testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEC2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KWOK EC2")
}

// priceStrategy scores an offering by a fixed price per instance type and capacity type
type priceStrategy map[string]float64

func (p priceStrategy) GetScore(instanceType, capacityType, _ string) float64 {
	if price, ok := p[fmt.Sprintf("%s/%s", instanceType, capacityType)]; ok {
		return price
	}
	return math.MaxFloat64
}

var _ = Describe("CreateFleet", func() {
	var ctx context.Context
	var cancel context.CancelFunc
	var client *Client

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		readBackupCompleted := make(chan struct{})
		close(readBackupCompleted)
		client = &Client{
			rateLimiterProvider: NewNopRateLimiterProvider(),
			clock:               clock.NewFakeClock(time.Now()),
			instanceTypes: lo.Map([]ec2types.InstanceType{"c6g.large", "c6gd.large", "c6a.large", "m6a.large"}, func(it ec2types.InstanceType, _ int) ec2types.InstanceTypeInfo {
				return ec2types.InstanceTypeInfo{
					InstanceType:  it,
					ProcessorInfo: &ec2types.ProcessorInfo{SupportedArchitectures: []ec2types.ArchitectureType{lo.Ternary(it[2] == 'g', ec2types.ArchitectureTypeArm64, ec2types.ArchitectureTypeX8664)}},
				}
			}),
			subnets: []ec2types.Subnet{
				{SubnetId: lo.ToPtr("subnet-a"), AvailabilityZone: lo.ToPtr("test-zone-1a"), VpcId: lo.ToPtr("vpc-1")},
				{SubnetId: lo.ToPtr("subnet-b"), AvailabilityZone: lo.ToPtr("test-zone-1b"), VpcId: lo.ToPtr("vpc-1")},
			},
			strategy: priceStrategy{
				"c6g.large/on-demand":  0.068,
				"c6gd.large/on-demand": 0.077,
				"c6a.large/on-demand":  0.0765,
				"m6a.large/on-demand":  0.0864,
			},
			instances:              sync.Map{},
			instanceLaunchCancels:  sync.Map{},
			readBackupCompleted:    readBackupCompleted,
			launchTemplates:        sync.Map{},
			launchTemplateNameToID: sync.Map{},
		}
		for _, name := range []string{"lt-amd64", "lt-arm64-instance-store", "lt-arm64"} {
			_, err := client.CreateLaunchTemplate(ctx, &ec2.CreateLaunchTemplateInput{
				LaunchTemplateName: lo.ToPtr(name),
				LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
					IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{Name: lo.ToPtr("profile")},
					MetadataOptions:    &ec2types.LaunchTemplateInstanceMetadataOptionsRequest{},
					Monitoring:         &ec2types.LaunchTemplatesMonitoringRequest{},
					NetworkInterfaces:  []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{{Groups: []string{"sg-123"}}},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		}
	})
	AfterEach(func() {
		// Stops the simulated instance startup goroutines
		cancel()
	})

	override := func(instanceType ec2types.InstanceType, subnetID string) ec2types.FleetLaunchTemplateOverridesRequest {
		return ec2types.FleetLaunchTemplateOverridesRequest{
			InstanceType: instanceType,
			SubnetId:     lo.ToPtr(subnetID),
			ImageId:      lo.ToPtr("ami-123"),
		}
	}
	ltConfig := func(name string, overrides ...ec2types.FleetLaunchTemplateOverridesRequest) ec2types.FleetLaunchTemplateConfigRequest {
		return ec2types.FleetLaunchTemplateConfigRequest{
			LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: lo.ToPtr(name),
				Version:            lo.ToPtr("$Latest"),
			},
			Overrides: overrides,
		}
	}
	createFleetInput := func(configs ...ec2types.FleetLaunchTemplateConfigRequest) *ec2.CreateFleetInput {
		input := &ec2.CreateFleetInput{
			Type:                  ec2types.FleetTypeInstant,
			LaunchTemplateConfigs: configs,
			TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
				DefaultTargetCapacityType: ec2types.DefaultTargetCapacityTypeOnDemand,
				TotalTargetCapacity:       lo.ToPtr[int32](1),
			},
		}
		return input
	}
	expectLaunched := func(out *ec2.CreateFleetOutput, ltName string, instanceType ec2types.InstanceType, subnetID string) {
		GinkgoHelper()
		Expect(out.Instances).To(HaveLen(1))
		fi := out.Instances[0]
		Expect(fi.InstanceType).To(Equal(instanceType))
		Expect(lo.FromPtr(fi.LaunchTemplateAndOverrides.LaunchTemplateSpecification.LaunchTemplateName)).To(Equal(ltName))
		Expect(fi.LaunchTemplateAndOverrides.Overrides.InstanceType).To(Equal(instanceType))
		Expect(lo.FromPtr(fi.LaunchTemplateAndOverrides.Overrides.SubnetId)).To(Equal(subnetID))
		ltID, ok := client.launchTemplateNameToID.Load(ltName)
		Expect(ok).To(BeTrue())
		Expect(fi.LaunchTemplateAndOverrides.LaunchTemplateSpecification.LaunchTemplateId).To(Equal(lo.ToPtr(ltID.(string))))
		raw, ok := client.instances.Load(fi.InstanceIds[0])
		Expect(ok).To(BeTrue())
		instance := raw.(ec2types.Instance)
		Expect(instance.InstanceType).To(Equal(instanceType))
		Expect(lo.FromPtr(instance.SubnetId)).To(Equal(subnetID))
		Expect(lo.FromPtr(instance.Placement.AvailabilityZone)).To(Equal(lo.FromPtr(fi.LaunchTemplateAndOverrides.Overrides.AvailabilityZone)))
	}

	It("should launch the cheapest override within a single launch template config", func() {
		out, err := client.CreateFleet(ctx, createFleetInput(
			ltConfig("lt-amd64", override("m6a.large", "subnet-a"), override("c6a.large", "subnet-b")),
		))
		Expect(err).ToNot(HaveOccurred())
		expectLaunched(out, "lt-amd64", "c6a.large", "subnet-b")
	})
	It("should launch the cheapest override across all launch template configs", func() {
		out, err := client.CreateFleet(ctx, createFleetInput(
			ltConfig("lt-arm64-instance-store", override("c6gd.large", "subnet-a")),
			ltConfig("lt-amd64", override("c6a.large", "subnet-a"), override("m6a.large", "subnet-a")),
			ltConfig("lt-arm64", override("c6g.large", "subnet-b")),
		))
		Expect(err).ToNot(HaveOccurred())
		expectLaunched(out, "lt-arm64", "c6g.large", "subnet-b")
	})
	It("should launch the same offering regardless of launch template config order", func() {
		configs := []ec2types.FleetLaunchTemplateConfigRequest{
			ltConfig("lt-arm64-instance-store", override("c6gd.large", "subnet-a")),
			ltConfig("lt-arm64", override("c6g.large", "subnet-a")),
			ltConfig("lt-amd64", override("c6a.large", "subnet-a")),
		}
		for i := range configs {
			rotated := append(append([]ec2types.FleetLaunchTemplateConfigRequest{}, configs[i:]...), configs[:i]...)
			out, err := client.CreateFleet(ctx, createFleetInput(rotated...))
			Expect(err).ToNot(HaveOccurred())
			expectLaunched(out, "lt-arm64", "c6g.large", "subnet-a")
		}
	})
	It("should fail if any launch template config references an unknown launch template", func() {
		_, err := client.CreateFleet(ctx, createFleetInput(
			ltConfig("lt-arm64", override("c6g.large", "subnet-a")),
			ltConfig("lt-missing", override("c6a.large", "subnet-a")),
		))
		Expect(err).To(HaveOccurred())
	})
})
