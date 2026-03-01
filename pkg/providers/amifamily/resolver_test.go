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
	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("Resolver", func() {
	Describe("injectSnapshotIDForVolumeInitialization", func() {
		var (
			amiRootDeviceName       string
			amiRootDeviceSnapshotID string
		)

		BeforeEach(func() {
			amiRootDeviceName = "/dev/xvda"
			amiRootDeviceSnapshotID = "snap-0123456789"
		})

		It("should inject snapshot ID when volumeInitializationRate is set and snapshotID is missing", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[0].EBS.SnapshotID).To(Equal("snap-0123456789"))
		})

		It("should inject snapshot ID when rootVolume flag is set", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"), // Different device name
					RootVolume: true,
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[0].EBS.SnapshotID).To(Equal("snap-0123456789"))
		})

		It("should not inject snapshot ID when explicit snapshotID is already set", func() {
			existingSnapshotID := "snap-existing"
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
						SnapshotID:               aws.String(existingSnapshotID),
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[0].EBS.SnapshotID).To(Equal(existingSnapshotID))
		})

		It("should not inject snapshot ID for non-root device", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sdb"), // Non-root device
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).To(BeNil())
		})

		It("should not inject snapshot ID when volumeInitializationRate is not set", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(100, resource.Giga),
						// No VolumeInitializationRate
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).To(BeNil())
		})

		It("should return original mappings when AMI root device info is missing", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
					},
				},
			}

			// Missing root device name
			result := injectSnapshotIDForVolumeInitialization(bdms, "", amiRootDeviceSnapshotID)
			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).To(BeNil())

			// Missing snapshot ID
			result = injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, "")
			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS.SnapshotID).To(BeNil())
		})

		It("should handle empty block device mappings", func() {
			var bdms []*v1.BlockDeviceMapping

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(0))
		})

		It("should handle nil EBS in block device mapping", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS:        nil, // nil EBS
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(1))
			Expect(result[0].EBS).To(BeNil())
		})

		It("should handle multiple block device mappings with mixed configurations", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					RootVolume: true,
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
						// No SnapshotID - should be injected
					},
				},
				{
					DeviceName: aws.String("/dev/sdb"),
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(200, resource.Giga),
						VolumeInitializationRate: aws.Int32(300),
						SnapshotID:               aws.String("snap-data"), // Explicit snapshot
					},
				},
				{
					DeviceName: aws.String("/dev/sdc"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
						// No VolumeInitializationRate
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			Expect(result).To(HaveLen(3))
			// First device: root volume with injected snapshot ID
			Expect(result[0].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[0].EBS.SnapshotID).To(Equal("snap-0123456789"))
			// Second device: keeps explicit snapshot ID
			Expect(result[1].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[1].EBS.SnapshotID).To(Equal("snap-data"))
			// Third device: no snapshot ID (no volumeInitializationRate)
			Expect(result[2].EBS.SnapshotID).To(BeNil())
		})

		It("should not mutate original block device mappings", func() {
			bdms := []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize:               resource.NewScaledQuantity(100, resource.Giga),
						VolumeInitializationRate: aws.Int32(500),
					},
				},
			}

			result := injectSnapshotIDForVolumeInitialization(bdms, amiRootDeviceName, amiRootDeviceSnapshotID)

			// Original should be unchanged
			Expect(bdms[0].EBS.SnapshotID).To(BeNil())
			// Result should have the injected snapshot ID
			Expect(result[0].EBS.SnapshotID).ToNot(BeNil())
			Expect(*result[0].EBS.SnapshotID).To(Equal("snap-0123456789"))
		})
	})
})
