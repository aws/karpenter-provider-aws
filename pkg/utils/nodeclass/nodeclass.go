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

package nodeclass

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

type Key struct {
	Name           string
	IsNodeTemplate bool
}

func New(nodeTemplate *v1alpha1.AWSNodeTemplate) *v1beta1.NodeClass {
	return &v1beta1.NodeClass{
		TypeMeta:   nodeTemplate.TypeMeta,
		ObjectMeta: nodeTemplate.ObjectMeta,
		Spec: v1beta1.NodeClassSpec{
			SubnetSelectorTerms:        NewSubnetSelectorTerms(nodeTemplate.Spec.SubnetSelector),
			SecurityGroupSelectorTerms: NewSecurityGroupSelectorTerms(nodeTemplate.Spec.SecurityGroupSelector),
			AMISelectorTerms:           NewAMISelectorTerms(nodeTemplate.Spec.AMISelector),
			AMIFamily:                  nodeTemplate.Spec.AMIFamily,
			UserData:                   nodeTemplate.Spec.UserData,
			Tags:                       nodeTemplate.Spec.Tags,
			BlockDeviceMappings: lo.Map(nodeTemplate.Spec.BlockDeviceMappings, func(bdm *v1alpha1.BlockDeviceMapping, _ int) *v1beta1.BlockDeviceMapping {
				return NewBlockDeviceMapping(bdm)
			}),
			DetailedMonitoring: nodeTemplate.Spec.DetailedMonitoring,
			MetadataOptions:    NewMetadataOptions(nodeTemplate.Spec.MetadataOptions),
			Context:            nodeTemplate.Spec.Context,
			LaunchTemplateName: nodeTemplate.Spec.LaunchTemplateName,
			InstanceProfile:    nodeTemplate.Spec.InstanceProfile,
		},
		Status: v1beta1.NodeClassStatus{
			Subnets: lo.Map(nodeTemplate.Status.Subnets, func(s v1alpha1.Subnet, _ int) v1beta1.Subnet {
				return v1beta1.Subnet{
					ID:   s.ID,
					Zone: s.Zone,
				}
			}),
			SecurityGroups: lo.Map(nodeTemplate.Status.SecurityGroups, func(s v1alpha1.SecurityGroup, _ int) v1beta1.SecurityGroup {
				return v1beta1.SecurityGroup{
					ID:   s.ID,
					Name: s.Name,
				}
			}),
			AMIs: lo.Map(nodeTemplate.Status.AMIs, func(a v1alpha1.AMI, _ int) v1beta1.AMI {
				return v1beta1.AMI{
					ID:           a.ID,
					Name:         a.Name,
					Requirements: a.Requirements,
				}
			}),
		},
		IsNodeTemplate: true,
	}
}

func NewSubnetSelectorTerms(subnetSelector map[string]string) []v1beta1.SubnetSelectorTerm {
	if len(subnetSelector) == 0 {
		return nil
	}
	return []v1beta1.SubnetSelectorTerm{
		{
			Tags: subnetSelector,
		},
	}
}

func NewSecurityGroupSelectorTerms(securityGroupSelector map[string]string) []v1beta1.SecurityGroupSelectorTerm {
	if len(securityGroupSelector) == 0 {
		return nil
	}
	return []v1beta1.SecurityGroupSelectorTerm{
		{
			Tags: securityGroupSelector,
		},
	}
}

func NewAMISelectorTerms(amiSelector map[string]string) (terms []v1beta1.AMISelectorTerm) {
	if len(amiSelector) == 0 {
		return nil
	}
	// Each of these slices needs to be pre-populated with the "0" element so that we can properly generate permutations
	ids := []string{""}
	names := []string{""}
	owners := []string{""}
	tags := map[string]string{}
	for k, v := range amiSelector {
		switch k {
		case "aws-ids", "aws::ids":
			ids = strings.Split(strings.Trim(v, " "), ",")
		case "aws::name":
			names = strings.Split(strings.Trim(v, " "), ",")
		case "aws::owners":
			owners = strings.Split(strings.Trim(v, " "), ",")
		default:
			tags[k] = v
		}
	}
	// If there are some "special" keys used, we have to represent the old selector as multiple terms
	for _, owner := range owners {
		for _, id := range ids {
			for _, name := range names {
				terms = append(terms, v1beta1.AMISelectorTerm{
					Tags:  tags,
					ID:    id,
					Name:  name,
					Owner: owner,
				})
			}
		}
	}
	return terms
}

func NewBlockDeviceMapping(bdm *v1alpha1.BlockDeviceMapping) *v1beta1.BlockDeviceMapping {
	if bdm == nil {
		return nil
	}
	return &v1beta1.BlockDeviceMapping{
		DeviceName: bdm.DeviceName,
		EBS:        NewBlockDevice(bdm.EBS),
	}
}

func NewBlockDevice(bd *v1alpha1.BlockDevice) *v1beta1.BlockDevice {
	if bd == nil {
		return nil
	}
	return &v1beta1.BlockDevice{
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

func NewMetadataOptions(mo *v1alpha1.MetadataOptions) *v1beta1.MetadataOptions {
	if mo == nil {
		return nil
	}
	return &v1beta1.MetadataOptions{
		HTTPEndpoint:            mo.HTTPEndpoint,
		HTTPProtocolIPv6:        mo.HTTPProtocolIPv6,
		HTTPPutResponseHopLimit: mo.HTTPPutResponseHopLimit,
		HTTPTokens:              mo.HTTPTokens,
	}
}

func Get(ctx context.Context, c client.Client, key Key) (*v1beta1.NodeClass, error) {
	if key.IsNodeTemplate {
		nodeTemplate := &v1alpha1.AWSNodeTemplate{}
		if err := c.Get(ctx, types.NamespacedName{Name: key.Name}, nodeTemplate); err != nil {
			return nil, err
		}
		return New(nodeTemplate), nil
	}
	nodeClass := &v1beta1.NodeClass{}
	if err := c.Get(ctx, types.NamespacedName{Name: key.Name}, nodeClass); err != nil {
		return nil, err
	}
	return nodeClass, nil
}
