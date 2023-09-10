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

package expectations

import (
	. "github.com/onsi/gomega" //nolint:revive,stylecheck
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

func ExpectBlockDeviceMappingsEqual(bdm1 []*v1alpha1.BlockDeviceMapping, bdm2 []*v1beta1.BlockDeviceMapping) {
	// Expect that all BlockDeviceMappings are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(bdm1).To(HaveLen(len(bdm2)))
	for i := range bdm1 {
		Expect(lo.FromPtr(bdm1[i].DeviceName)).To(Equal(lo.FromPtr(bdm2[i].DeviceName)))
		ExpectBlockDevicesEqual(bdm1[i].EBS, bdm2[i].EBS)
	}
}

func ExpectBlockDevicesEqual(bd1 *v1alpha1.BlockDevice, bd2 *v1beta1.BlockDevice) {
	Expect(bd1 == nil).To(Equal(bd2 == nil))
	if bd1 != nil {
		Expect(lo.FromPtr(bd1.DeleteOnTermination)).To(Equal(lo.FromPtr(bd2.VolumeType)))
		Expect(lo.FromPtr(bd1.Encrypted)).To(Equal(lo.FromPtr(bd2.Encrypted)))
		Expect(lo.FromPtr(bd1.IOPS)).To(Equal(lo.FromPtr(bd2.IOPS)))
		Expect(lo.FromPtr(bd1.KMSKeyID)).To(Equal(lo.FromPtr(bd2.KMSKeyID)))
		Expect(lo.FromPtr(bd1.SnapshotID)).To(Equal(lo.FromPtr(bd2.SnapshotID)))
		Expect(lo.FromPtr(bd1.Throughput)).To(Equal(lo.FromPtr(bd2.Throughput)))
		Expect(lo.FromPtr(bd1.VolumeSize)).To(Equal(lo.FromPtr(bd2.VolumeSize)))
		Expect(lo.FromPtr(bd1.VolumeType)).To(Equal(lo.FromPtr(bd2.VolumeType)))
	}
}

func ExpectMetadataOptionsEqual(mo1 *v1alpha1.MetadataOptions, mo2 *v1beta1.MetadataOptions) {
	Expect(mo1 == nil).To(Equal(mo2 == nil))
	if mo1 != nil {
		Expect(lo.FromPtr(mo1.HTTPEndpoint)).To(Equal(lo.FromPtr(mo2.HTTPEndpoint)))
		Expect(lo.FromPtr(mo1.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(mo2.HTTPProtocolIPv6)))
		Expect(lo.FromPtr(mo1.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(mo2.HTTPPutResponseHopLimit)))
		Expect(lo.FromPtr(mo1.HTTPTokens)).To(Equal(lo.FromPtr(mo2.HTTPTokens)))
	}
}

func ExpectSubnetStatusEqual(subnets1 []v1alpha1.Subnet, subnets2 []v1beta1.Subnet) {
	// Expect that all Subnet Status entries are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(subnets1).To(HaveLen(len(subnets2)))
	for i := range subnets1 {
		Expect(subnets1[i].ID).To(Equal(subnets2[i].ID))
		Expect(subnets1[i].Zone).To(Equal(subnets2[i].Zone))
	}
}

func ExpectSecurityGroupStatusEqual(securityGroups1 []v1alpha1.SecurityGroup, securityGroups2 []v1beta1.SecurityGroup) {
	// Expect that all SecurityGroup Status entries are present and the same
	// Ensure that they are the same by ensuring a consistent ordering
	Expect(securityGroups1).To(HaveLen(len(securityGroups2)))
	for i := range securityGroups1 {
		Expect(securityGroups1[i].ID).To(Equal(securityGroups2[i].ID))
		Expect(securityGroups1[i].Name).To(Equal(securityGroups2[i].Name))
	}
}

func ExpectAMIStatusEqual(amis1 []v1alpha1.AMI, amis2 []v1beta1.AMI) {
	// Expect that all AMI Status entries are present and the same
	Expect(amis1).To(HaveLen(len(amis2)))
	for i := range amis1 {
		Expect(amis1[i].ID).To(Equal(amis2[i].ID))
		Expect(amis1[i].Name).To(Equal(amis2[i].Name))
		Expect(amis1[i].Requirements).To(ConsistOf(lo.Map(amis2[i].Requirements, func(r v1.NodeSelectorRequirement, _ int) interface{} { return BeEquivalentTo(r) })...))
	}
}
