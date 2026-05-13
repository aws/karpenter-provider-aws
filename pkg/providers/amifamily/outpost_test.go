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

package amifamily_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
)

var _ = Describe("Outpost EBS Defaults", func() {
	It("should default to gp2 when OutpostArn is set and no explicit blockDeviceMappings", func() {
		amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion)
		launchTemplates, err := amiResolver.Resolve(nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{
			ClusterName: "test",
			OutpostArn:  "arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0",
		}, "", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(launchTemplates).ToNot(BeEmpty())
		for _, lt := range launchTemplates {
			for _, bdm := range lt.BlockDeviceMappings {
				if bdm.EBS != nil {
					Expect(lo.FromPtr(bdm.EBS.VolumeType)).To(Equal(string(ec2types.VolumeTypeGp2)))
				}
			}
		}
	})

	It("should default to gp3 when OutpostArn is NOT set", func() {
		amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion)
		launchTemplates, err := amiResolver.Resolve(nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{
			ClusterName: "test",
		}, "", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(launchTemplates).ToNot(BeEmpty())
		for _, lt := range launchTemplates {
			for _, bdm := range lt.BlockDeviceMappings {
				if bdm.EBS != nil {
					Expect(lo.FromPtr(bdm.EBS.VolumeType)).To(Equal(string(ec2types.VolumeTypeGp3)))
				}
			}
		}
	})

	It("should preserve user-specified blockDeviceMappings when OutpostArn is set", func() {
		nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/xvda"),
				EBS: &v1.BlockDevice{
					Encrypted:  aws.Bool(true),
					VolumeType: aws.String(string(ec2types.VolumeTypeIo1)),
					VolumeSize: lo.ToPtr(resource.MustParse("50Gi")),
					IOPS:       aws.Int64(3000),
				},
			},
		}
		amiResolver := amifamily.NewDefaultResolver(fake.DefaultRegion)
		launchTemplates, err := amiResolver.Resolve(nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, string(ec2types.TenancyDefault), &amifamily.Options{
			ClusterName: "test",
			OutpostArn:  "arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0",
		}, "", 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(launchTemplates).ToNot(BeEmpty())
		for _, lt := range launchTemplates {
			Expect(lt.BlockDeviceMappings).To(HaveLen(1))
			Expect(lo.FromPtr(lt.BlockDeviceMappings[0].EBS.VolumeType)).To(Equal(string(ec2types.VolumeTypeIo1)))
		}
	})
})
