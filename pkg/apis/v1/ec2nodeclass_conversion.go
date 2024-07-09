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

package v1

import (
	"context"

	"github.com/samber/lo"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

func (in *EC2NodeClass) ConvertTo(ctx context.Context, to apis.Convertible) error {
	v1beta1enc := to.(*v1beta1.EC2NodeClass)
	v1beta1enc.ObjectMeta = in.ObjectMeta

	in.Spec.convertTo(&v1beta1enc.Spec)
	in.Status.convertTo((&v1beta1enc.Status))
	return nil
}

func (in *EC2NodeClassSpec) convertTo(v1beta1enc *v1beta1.EC2NodeClassSpec) {
	v1beta1enc.SubnetSelectorTerms = lo.Map(in.SubnetSelectorTerms, func(subnet SubnetSelectorTerm, _ int) v1beta1.SubnetSelectorTerm {
		return v1beta1.SubnetSelectorTerm{
			ID:   subnet.ID,
			Tags: subnet.Tags,
		}
	})
	v1beta1enc.SecurityGroupSelectorTerms = lo.Map(in.SecurityGroupSelectorTerms, func(sg SecurityGroupSelectorTerm, _ int) v1beta1.SecurityGroupSelectorTerm {
		return v1beta1.SecurityGroupSelectorTerm{
			ID:   sg.ID,
			Name: sg.Name,
			Tags: sg.Tags,
		}
	})
	v1beta1enc.AMISelectorTerms = lo.Map(in.AMISelectorTerms, func(ami AMISelectorTerm, _ int) v1beta1.AMISelectorTerm {
		return v1beta1.AMISelectorTerm{
			ID:    ami.ID,
			Name:  ami.Name,
			Owner: ami.Owner,
			Tags:  ami.Tags,
		}
	})
	v1beta1enc.AMIFamily = in.AMIFamily
	v1beta1enc.AssociatePublicIPAddress = in.AssociatePublicIPAddress
	v1beta1enc.Context = in.Context
	v1beta1enc.DetailedMonitoring = in.DetailedMonitoring
	v1beta1enc.Role = in.Role
	v1beta1enc.InstanceProfile = in.InstanceProfile
	v1beta1enc.InstanceStorePolicy = (*v1beta1.InstanceStorePolicy)(in.InstanceStorePolicy)
	v1beta1enc.Tags = in.Tags
	v1beta1enc.UserData = in.UserData
	v1beta1enc.MetadataOptions = (*v1beta1.MetadataOptions)(in.MetadataOptions)
	v1beta1enc.BlockDeviceMappings = lo.Map(in.BlockDeviceMappings, func(bdm *BlockDeviceMapping, _ int) *v1beta1.BlockDeviceMapping {
		return &v1beta1.BlockDeviceMapping{
			DeviceName: bdm.DeviceName,
			RootVolume: bdm.RootVolume,
			EBS:        (*v1beta1.BlockDevice)(bdm.EBS),
		}
	})
}

func (in *EC2NodeClassStatus) convertTo(v1beta1enc *v1beta1.EC2NodeClassStatus) {
	v1beta1enc.Subnets = lo.Map(in.Subnets, func(subnet Subnet, _ int) v1beta1.Subnet {
		return v1beta1.Subnet{
			ID:     subnet.ID,
			Zone:   subnet.Zone,
			ZoneID: subnet.ZoneID,
		}
	})
	v1beta1enc.SecurityGroups = lo.Map(in.SecurityGroups, func(sg SecurityGroup, _ int) v1beta1.SecurityGroup {
		return v1beta1.SecurityGroup{
			ID:   sg.ID,
			Name: sg.Name,
		}
	})
	v1beta1enc.AMIs = lo.Map(in.AMIs, func(ami AMI, _ int) v1beta1.AMI {
		return v1beta1.AMI{
			ID:           ami.ID,
			Name:         ami.Name,
			Requirements: ami.Requirements,
		}
	})
	v1beta1enc.InstanceProfile = in.InstanceProfile
	v1beta1enc.Conditions = in.Conditions
}

func (in *EC2NodeClass) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	v1beta1enc := from.(*v1beta1.EC2NodeClass)
	in.ObjectMeta = v1beta1enc.ObjectMeta

	in.Spec.convertFrom(&v1beta1enc.Spec)
	in.Status.convertFrom((&v1beta1enc.Status))
	return nil
}

func (in *EC2NodeClassSpec) convertFrom(v1beta1enc *v1beta1.EC2NodeClassSpec) {
	in.SubnetSelectorTerms = lo.Map(v1beta1enc.SubnetSelectorTerms, func(subnet v1beta1.SubnetSelectorTerm, _ int) SubnetSelectorTerm {
		return SubnetSelectorTerm{
			ID:   subnet.ID,
			Tags: subnet.Tags,
		}
	})
	in.SecurityGroupSelectorTerms = lo.Map(v1beta1enc.SecurityGroupSelectorTerms, func(sg v1beta1.SecurityGroupSelectorTerm, _ int) SecurityGroupSelectorTerm {
		return SecurityGroupSelectorTerm{
			ID:   sg.ID,
			Name: sg.Name,
			Tags: sg.Tags,
		}
	})
	in.AMISelectorTerms = lo.Map(v1beta1enc.AMISelectorTerms, func(ami v1beta1.AMISelectorTerm, _ int) AMISelectorTerm {
		return AMISelectorTerm{
			ID:    ami.ID,
			Name:  ami.Name,
			Owner: ami.Owner,
			Tags:  ami.Tags,
		}
	})
	in.AMIFamily = v1beta1enc.AMIFamily
	in.AssociatePublicIPAddress = v1beta1enc.AssociatePublicIPAddress
	in.Context = v1beta1enc.Context
	in.DetailedMonitoring = v1beta1enc.DetailedMonitoring
	in.Role = v1beta1enc.Role
	in.InstanceProfile = v1beta1enc.InstanceProfile
	in.InstanceStorePolicy = (*InstanceStorePolicy)(v1beta1enc.InstanceStorePolicy)
	in.Tags = v1beta1enc.Tags
	in.UserData = v1beta1enc.UserData
	in.MetadataOptions = (*MetadataOptions)(v1beta1enc.MetadataOptions)
	in.BlockDeviceMappings = lo.Map(v1beta1enc.BlockDeviceMappings, func(bdm *v1beta1.BlockDeviceMapping, _ int) *BlockDeviceMapping {
		return &BlockDeviceMapping{
			DeviceName: bdm.DeviceName,
			RootVolume: bdm.RootVolume,
			EBS:        (*BlockDevice)(bdm.EBS),
		}
	})
}

func (in *EC2NodeClassStatus) convertFrom(v1beta1enc *v1beta1.EC2NodeClassStatus) {
	in.Subnets = lo.Map(v1beta1enc.Subnets, func(subnet v1beta1.Subnet, _ int) Subnet {
		return Subnet{
			ID:     subnet.ID,
			Zone:   subnet.Zone,
			ZoneID: subnet.ZoneID,
		}
	})
	in.SecurityGroups = lo.Map(v1beta1enc.SecurityGroups, func(sg v1beta1.SecurityGroup, _ int) SecurityGroup {
		return SecurityGroup{
			ID:   sg.ID,
			Name: sg.Name,
		}
	})
	in.AMIs = lo.Map(v1beta1enc.AMIs, func(ami v1beta1.AMI, _ int) AMI {
		return AMI{
			ID:           ami.ID,
			Name:         ami.Name,
			Requirements: ami.Requirements,
		}
	})
	in.InstanceProfile = v1beta1enc.InstanceProfile
	in.Conditions = v1beta1enc.Conditions
}
