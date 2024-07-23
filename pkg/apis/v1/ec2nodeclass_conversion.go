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
	"fmt"
	"strings"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

func (in *EC2NodeClass) ConvertTo(ctx context.Context, to apis.Convertible) error {
	v1beta1enc := to.(*v1beta1.EC2NodeClass)
	v1beta1enc.ObjectMeta = in.ObjectMeta

	v1beta1enc.Spec.AMIFamily = lo.ToPtr(in.AMIFamily())
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
	v1beta1enc.AMISelectorTerms = lo.FilterMap(in.AMISelectorTerms, func(term AMISelectorTerm, _ int) (v1beta1.AMISelectorTerm, bool) {
		if term.Alias != "" {
			return v1beta1.AMISelectorTerm{}, false
		}
		return v1beta1.AMISelectorTerm{
			ID:    term.ID,
			Name:  term.Name,
			Owner: term.Owner,
			Tags:  term.Tags,
		}, true
	})
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

	// TODO: jmdeal@ remove before v1
	// Temporarily fail closed when trying to convert EC2NodeClasses with the Ubuntu AMI family since compatibility support isn't yet integrated.
	// This check can be removed once it's added.
	if lo.FromPtr(v1beta1enc.Spec.AMIFamily) == v1beta1.AMIFamilyUbuntu && len(v1beta1enc.Spec.AMISelectorTerms) == 0 {
		return fmt.Errorf("failed to convert v1beta1 EC2NodeClass to v1, conversion for Ubuntu AMIFamily without AMISelectorTerms is currently unsupported")
	}

	switch lo.FromPtr(v1beta1enc.Spec.AMIFamily) {
	case AMIFamilyAL2, AMIFamilyAL2023, AMIFamilyBottlerocket, AMIFamilyWindows2019, AMIFamilyWindows2022:
		// If no amiSelectorTerms are specified, we can create an alias and don't need to specify amiFamily. Otherwise,
		// we'll carry over the amiSelectorTerms and amiFamily.
		if len(v1beta1enc.Spec.AMISelectorTerms) == 0 {
			in.Spec.AMIFamily = nil
			in.Spec.AMISelectorTerms = []AMISelectorTerm{{
				Alias: fmt.Sprintf("%s@latest", strings.ToLower(lo.FromPtr(v1beta1enc.Spec.AMIFamily))),
			}}
		} else {
			in.Spec.AMIFamily = v1beta1enc.Spec.AMIFamily
		}
	case AMIFamilyUbuntu:
		// If there are no amiSelectorTerms specified, we need to use the compatibility annotation so we can discover
		// amis at runtime. Otherwise, we can carry over the amiSelectorTerms and use the AL2 family for bootstrapping
		// (they have the same UserData). In this case, we'll have to override AL2's default block device mappings.
		if len(v1beta1enc.Spec.AMISelectorTerms) == 0 {
			in.Annotations = lo.Assign(in.Annotations, map[string]string{
				AnnotationAMIFamilyCompatibility: AMIFamilyUbuntu,
			})
		} else {
			in.Spec.AMIFamily = lo.ToPtr(AMIFamilyAL2)
			if v1beta1enc.Spec.BlockDeviceMappings == nil {
				in.Spec.BlockDeviceMappings = []*BlockDeviceMapping{{
					DeviceName: lo.ToPtr("/dev/sda1"),
					EBS: &BlockDevice{
						Encrypted:  lo.ToPtr(true),
						VolumeType: lo.ToPtr(ec2.VolumeTypeGp3),
						VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
					},
				}}
			}
		}
	default:
		// The amiFamily is custom or undefined (shouldn't be possible via validation). We'll treat it as custom
		// regardless.
		in.Spec.AMIFamily = lo.ToPtr(AMIFamilyCustom)
	}

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
	in.AMISelectorTerms = append(in.AMISelectorTerms, lo.Map(v1beta1enc.AMISelectorTerms, func(ami v1beta1.AMISelectorTerm, _ int) AMISelectorTerm {
		return AMISelectorTerm{
			ID:    ami.ID,
			Name:  ami.Name,
			Owner: ami.Owner,
			Tags:  ami.Tags,
		}
	})...)
	in.AssociatePublicIPAddress = v1beta1enc.AssociatePublicIPAddress
	in.Context = v1beta1enc.Context
	in.DetailedMonitoring = v1beta1enc.DetailedMonitoring
	in.Role = v1beta1enc.Role
	in.InstanceProfile = v1beta1enc.InstanceProfile
	in.InstanceStorePolicy = (*InstanceStorePolicy)(v1beta1enc.InstanceStorePolicy)
	in.Tags = v1beta1enc.Tags
	in.UserData = v1beta1enc.UserData
	in.MetadataOptions = (*MetadataOptions)(v1beta1enc.MetadataOptions)
	if v1beta1enc.BlockDeviceMappings != nil {
		in.BlockDeviceMappings = lo.Map(v1beta1enc.BlockDeviceMappings, func(bdm *v1beta1.BlockDeviceMapping, _ int) *BlockDeviceMapping {
			return &BlockDeviceMapping{
				DeviceName: bdm.DeviceName,
				RootVolume: bdm.RootVolume,
				EBS:        (*BlockDevice)(bdm.EBS),
			}
		})
	}
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
