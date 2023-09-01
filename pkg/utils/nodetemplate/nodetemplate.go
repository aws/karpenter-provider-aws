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

package nodetemplate

import (
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

func New(nodeClass *v1beta1.NodeClass) *v1alpha1.AWSNodeTemplate {
	return &v1alpha1.AWSNodeTemplate{
		TypeMeta:   nodeClass.TypeMeta,
		ObjectMeta: nodeClass.ObjectMeta,
		Spec: v1alpha1.AWSNodeTemplateSpec{
			UserData: nodeClass.Spec.UserData,
			AWS: v1alpha1.AWS{
				AMIFamily:             nodeClass.Spec.AMIFamily,
				Context:               nodeClass.Spec.Context,
				InstanceProfile:       nodeClass.Spec.InstanceProfile,
				SubnetSelector:        nodeClass.Spec.OriginalSubnetSelector,
				SecurityGroupSelector: nodeClass.Spec.OriginalSecurityGroupSelector,
				Tags:                  nodeClass.Spec.Tags,
				LaunchTemplate: v1alpha1.LaunchTemplate{
					LaunchTemplateName:  nodeClass.Spec.LaunchTemplateName,
					MetadataOptions:     NewMetadataOptions(nodeClass.Spec.MetadataOptions),
					BlockDeviceMappings: NewBlockDeviceMappings(nodeClass.Spec.BlockDeviceMappings),
				},
			},
			AMISelector:        nodeClass.Spec.OriginalAMISelector,
			DetailedMonitoring: nodeClass.Spec.DetailedMonitoring,
		},
		Status: v1alpha1.AWSNodeTemplateStatus{
			Subnets:        NewSubnets(nodeClass.Status.Subnets),
			SecurityGroups: NewSecurityGroups(nodeClass.Status.SecurityGroups),
			AMIs:           NewAMIs(nodeClass.Status.AMIs),
		},
	}
}

func NewBlockDeviceMappings(bdms []*v1beta1.BlockDeviceMapping) []*v1alpha1.BlockDeviceMapping {
	if bdms == nil {
		return nil
	}
	return lo.Map(bdms, func(bdm *v1beta1.BlockDeviceMapping, _ int) *v1alpha1.BlockDeviceMapping {
		return NewBlockDeviceMapping(bdm)
	})
}

func NewBlockDeviceMapping(bdm *v1beta1.BlockDeviceMapping) *v1alpha1.BlockDeviceMapping {
	if bdm == nil {
		return nil
	}
	return &v1alpha1.BlockDeviceMapping{
		DeviceName: bdm.DeviceName,
		EBS:        NewBlockDevice(bdm.EBS),
	}
}

func NewBlockDevice(bd *v1beta1.BlockDevice) *v1alpha1.BlockDevice {
	if bd == nil {
		return nil
	}
	return &v1alpha1.BlockDevice{
		DeleteOnTermination: bd.DeleteOnTermination,
		Encrypted:           bd.Encrypted,
		IOPS:                bd.IOPS,
		KMSKeyID:            bd.KMSKeyID,
		SnapshotID:          bd.SnapshotID,
		Throughput:          bd.Throughput,
		VolumeSize:          bd.VolumeSize,
		VolumeType:          bd.VolumeType,
	}
}

func NewMetadataOptions(mo *v1beta1.MetadataOptions) *v1alpha1.MetadataOptions {
	if mo == nil {
		return nil
	}
	return &v1alpha1.MetadataOptions{
		HTTPEndpoint:            mo.HTTPEndpoint,
		HTTPProtocolIPv6:        mo.HTTPProtocolIPv6,
		HTTPPutResponseHopLimit: mo.HTTPPutResponseHopLimit,
		HTTPTokens:              mo.HTTPTokens,
	}
}

func NewSubnets(subnets []v1beta1.Subnet) []v1alpha1.Subnet {
	if subnets == nil {
		return nil
	}
	return lo.Map(subnets, func(s v1beta1.Subnet, _ int) v1alpha1.Subnet {
		return v1alpha1.Subnet{
			ID:   s.ID,
			Zone: s.Zone,
		}
	})
}

func NewSecurityGroups(securityGroups []v1beta1.SecurityGroup) []v1alpha1.SecurityGroup {
	if securityGroups == nil {
		return nil
	}
	return lo.Map(securityGroups, func(s v1beta1.SecurityGroup, _ int) v1alpha1.SecurityGroup {
		return v1alpha1.SecurityGroup{
			ID:   s.ID,
			Name: s.Name,
		}
	})
}

func NewAMIs(amis []v1beta1.AMI) []v1alpha1.AMI {
	if amis == nil {
		return nil
	}
	return lo.Map(amis, func(a v1beta1.AMI, _ int) v1alpha1.AMI {
		return v1alpha1.AMI{
			ID:           a.ID,
			Name:         a.Name,
			Requirements: a.Requirements,
		}
	})
}
