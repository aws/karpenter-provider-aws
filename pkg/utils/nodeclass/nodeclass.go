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
	nodetemplateutil "github.com/aws/karpenter/pkg/utils/nodetemplate"
)

type Key struct {
	Name           string
	IsNodeTemplate bool
}

func New(nodeTemplate *v1alpha1.AWSNodeTemplate) *v1beta1.EC2NodeClass {
	return &v1beta1.EC2NodeClass{
		TypeMeta:   nodeTemplate.TypeMeta,
		ObjectMeta: nodeTemplate.ObjectMeta,
		Spec: v1beta1.EC2NodeClassSpec{
			SubnetSelectorTerms:           NewSubnetSelectorTerms(nodeTemplate.Spec.SubnetSelector),
			OriginalSubnetSelector:        nodeTemplate.Spec.SubnetSelector,
			SecurityGroupSelectorTerms:    NewSecurityGroupSelectorTerms(nodeTemplate.Spec.SecurityGroupSelector),
			OriginalSecurityGroupSelector: nodeTemplate.Spec.SecurityGroupSelector,
			AMISelectorTerms:              NewAMISelectorTerms(nodeTemplate.Spec.AMISelector),
			OriginalAMISelector:           nodeTemplate.Spec.AMISelector,
			AMIFamily:                     nodeTemplate.Spec.AMIFamily,
			UserData:                      nodeTemplate.Spec.UserData,
			Tags:                          nodeTemplate.Spec.Tags,
			BlockDeviceMappings:           NewBlockDeviceMappings(nodeTemplate.Spec.BlockDeviceMappings),
			DetailedMonitoring:            nodeTemplate.Spec.DetailedMonitoring,
			MetadataOptions:               NewMetadataOptions(nodeTemplate.Spec.MetadataOptions),
			Context:                       nodeTemplate.Spec.Context,
			LaunchTemplateName:            nodeTemplate.Spec.LaunchTemplateName,
			InstanceProfile:               nodeTemplate.Spec.InstanceProfile,
		},
		Status: v1beta1.EC2NodeClassStatus{
			Subnets:        NewSubnets(nodeTemplate.Status.Subnets),
			SecurityGroups: NewSecurityGroups(nodeTemplate.Status.SecurityGroups),
			AMIs:           NewAMIs(nodeTemplate.Status.AMIs),
		},
		IsNodeTemplate: true,
	}
}

func NewSubnetSelectorTerms(subnetSelector map[string]string) (terms []v1beta1.SubnetSelectorTerm) {
	if len(subnetSelector) == 0 {
		return nil
	}
	// Each of these slices needs to be pre-populated with the "0" element so that we can properly generate permutations
	ids := []string{""}
	tags := map[string]string{}
	for k, v := range subnetSelector {
		switch k {
		case "aws-ids", "aws::ids":
			ids = lo.Map(strings.Split(v, ","), func(s string, _ int) string { return strings.Trim(s, " ") })
		default:
			tags[k] = v
		}
	}
	// If there are some "special" keys used, we have to represent the old selector as multiple terms
	for _, id := range ids {
		terms = append(terms, v1beta1.SubnetSelectorTerm{
			Tags: tags,
			ID:   id,
		})
	}
	return terms
}

func NewSecurityGroupSelectorTerms(securityGroupSelector map[string]string) (terms []v1beta1.SecurityGroupSelectorTerm) {
	if len(securityGroupSelector) == 0 {
		return nil
	}
	// Each of these slices needs to be pre-populated with the "0" element so that we can properly generate permutations
	ids := []string{""}
	tags := map[string]string{}
	for k, v := range securityGroupSelector {
		switch k {
		case "aws-ids", "aws::ids":
			ids = lo.Map(strings.Split(v, ","), func(s string, _ int) string { return strings.Trim(s, " ") })
		default:
			tags[k] = v
		}
	}
	// If there are some "special" keys used, we have to represent the old selector as multiple terms
	for _, id := range ids {
		terms = append(terms, v1beta1.SecurityGroupSelectorTerm{
			Tags: tags,
			ID:   id,
		})
	}
	return terms
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
			ids = lo.Map(strings.Split(v, ","), func(s string, _ int) string { return strings.Trim(s, " ") })
		case "aws::name":
			names = lo.Map(strings.Split(v, ","), func(s string, _ int) string { return strings.Trim(s, " ") })
		case "aws::owners":
			owners = lo.Map(strings.Split(v, ","), func(s string, _ int) string { return strings.Trim(s, " ") })
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

func NewBlockDeviceMappings(bdms []*v1alpha1.BlockDeviceMapping) []*v1beta1.BlockDeviceMapping {
	if bdms == nil {
		return nil
	}
	return lo.Map(bdms, func(bdm *v1alpha1.BlockDeviceMapping, _ int) *v1beta1.BlockDeviceMapping {
		return NewBlockDeviceMapping(bdm)
	})
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

func NewSubnets(subnets []v1alpha1.Subnet) []v1beta1.Subnet {
	if subnets == nil {
		return nil
	}
	return lo.Map(subnets, func(s v1alpha1.Subnet, _ int) v1beta1.Subnet {
		return v1beta1.Subnet{
			ID:   s.ID,
			Zone: s.Zone,
		}
	})
}

func NewSecurityGroups(securityGroups []v1alpha1.SecurityGroup) []v1beta1.SecurityGroup {
	if securityGroups == nil {
		return nil
	}
	return lo.Map(securityGroups, func(s v1alpha1.SecurityGroup, _ int) v1beta1.SecurityGroup {
		return v1beta1.SecurityGroup{
			ID:   s.ID,
			Name: s.Name,
		}
	})
}

func NewAMIs(amis []v1alpha1.AMI) []v1beta1.AMI {
	if amis == nil {
		return nil
	}
	return lo.Map(amis, func(a v1alpha1.AMI, _ int) v1beta1.AMI {
		return v1beta1.AMI{
			ID:           a.ID,
			Name:         a.Name,
			Requirements: a.Requirements,
		}
	})
}

func Get(ctx context.Context, c client.Client, key Key) (*v1beta1.EC2NodeClass, error) {
	if key.IsNodeTemplate {
		nodeTemplate := &v1alpha1.AWSNodeTemplate{}
		if err := c.Get(ctx, types.NamespacedName{Name: key.Name}, nodeTemplate); err != nil {
			return nil, err
		}
		return New(nodeTemplate), nil
	}
	nodeClass := &v1beta1.EC2NodeClass{}
	if err := c.Get(ctx, types.NamespacedName{Name: key.Name}, nodeClass); err != nil {
		return nil, err
	}
	return nodeClass, nil
}

func Patch(ctx context.Context, c client.Client, stored, nodeClass *v1beta1.EC2NodeClass) error {
	if nodeClass.IsNodeTemplate {
		storedNodeTemplate := nodetemplateutil.New(stored)
		nodeTemplate := nodetemplateutil.New(nodeClass)
		return c.Patch(ctx, nodeTemplate, client.MergeFrom(storedNodeTemplate))
	}
	return c.Patch(ctx, nodeClass, client.MergeFrom(stored))
}

func PatchStatus(ctx context.Context, c client.Client, stored, nodeClass *v1beta1.EC2NodeClass) error {
	if nodeClass.IsNodeTemplate {
		storedNodeTemplate := nodetemplateutil.New(stored)
		nodeTemplate := nodetemplateutil.New(nodeClass)
		return c.Status().Patch(ctx, nodeTemplate, client.MergeFrom(storedNodeTemplate))
	}
	return c.Status().Patch(ctx, nodeClass, client.MergeFrom(stored))
}

func HashAnnotation(nodeClass *v1beta1.EC2NodeClass) map[string]string {
	if nodeClass.IsNodeTemplate {
		nodeTemplate := nodetemplateutil.New(nodeClass)
		return map[string]string{v1alpha1.AnnotationNodeTemplateHash: nodeTemplate.Hash()}
	}
	return map[string]string{v1beta1.AnnotationNodeClassHash: nodeClass.Hash()}
}
