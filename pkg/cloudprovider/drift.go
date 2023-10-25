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

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/utils"
)

const (
	AMIDrift           cloudprovider.DriftReason = "AMIDrift"
	SubnetDrift        cloudprovider.DriftReason = "SubnetDrift"
	SecurityGroupDrift cloudprovider.DriftReason = "SecurityGroupDrift"
	NodeTemplateDrift  cloudprovider.DriftReason = "NodeTemplateDrift"
	NodeClassDrift     cloudprovider.DriftReason = "NodeClassDrift"
)

func (c *CloudProvider) isNodeClassDrifted(ctx context.Context, nodeClaim *corev1beta1.NodeClaim, nodePool *corev1beta1.NodePool, nodeClass *v1beta1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	// First check if the node class is statically drifted to save on API calls.
	if drifted := c.areStaticFieldsDrifted(nodeClaim, nodeClass); drifted != "" {
		return drifted, nil
	}
	instance, err := c.getInstance(ctx, nodeClaim.Status.ProviderID)
	if err != nil {
		return "", err
	}
	amiDrifted, err := c.isAMIDrifted(ctx, nodeClaim, nodePool, instance, nodeClass)
	if err != nil {
		return "", fmt.Errorf("calculating ami drift, %w", err)
	}
	securitygroupDrifted, err := c.areSecurityGroupsDrifted(instance, nodeClass)
	if err != nil {
		return "", fmt.Errorf("calculating securitygroup drift, %w", err)
	}
	subnetDrifted, err := c.isSubnetDrifted(instance, nodeClass)
	if err != nil {
		return "", fmt.Errorf("calculating subnet drift, %w", err)
	}
	drifted := lo.FindOrElse([]cloudprovider.DriftReason{amiDrifted, securitygroupDrifted, subnetDrifted}, "", func(i cloudprovider.DriftReason) bool {
		return string(i) != ""
	})
	return drifted, nil
}

func (c *CloudProvider) isAMIDrifted(ctx context.Context, nodeClaim *corev1beta1.NodeClaim, nodePool *corev1beta1.NodePool,
	instance *instance.Instance, nodeClass *v1beta1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	instanceTypes, err := c.GetInstanceTypes(ctx, nodePool)
	if err != nil {
		return "", fmt.Errorf("getting instanceTypes, %w", err)
	}
	nodeInstanceType, found := lo.Find(instanceTypes, func(instType *cloudprovider.InstanceType) bool {
		return instType.Name == nodeClaim.Labels[v1.LabelInstanceTypeStable]
	})
	if !found {
		return "", fmt.Errorf(`finding node instance type "%s"`, nodeClaim.Labels[v1.LabelInstanceTypeStable])
	}
	if nodeClass.Spec.LaunchTemplateName != nil {
		return "", nil
	}
	amis, err := c.amiProvider.Get(ctx, nodeClass, &amifamily.Options{})
	if err != nil {
		return "", fmt.Errorf("getting amis, %w", err)
	}
	if len(amis) == 0 {
		return "", fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := amis.MapToInstanceTypes([]*cloudprovider.InstanceType{nodeInstanceType}, nodeClaim.IsMachine)
	if !lo.Contains(lo.Keys(mappedAMIs), instance.ImageID) {
		return AMIDrift, nil
	}
	return "", nil
}

// Checks if the security groups are drifted, by comparing the EC2NodeClass.Status.Subnets
// to the ec2 instance subnets
func (c *CloudProvider) isSubnetDrifted(instance *instance.Instance, nodeClass *v1beta1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	// If the node template status does not have subnets, wait for the subnets to be populated before continuing
	if len(nodeClass.Status.Subnets) == 0 {
		return "", fmt.Errorf("no subnets exist in status")
	}
	_, found := lo.Find(nodeClass.Status.Subnets, func(subnet v1beta1.Subnet) bool {
		return subnet.ID == instance.SubnetID
	})
	if !found {
		return SubnetDrift, nil
	}
	return "", nil
}

// Checks if the security groups are drifted, by comparing the EC2NodeClass.Status.SecurityGroups
// to the ec2 instance security groups
func (c *CloudProvider) areSecurityGroupsDrifted(ec2Instance *instance.Instance, nodeClass *v1beta1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	// nodeClass.Spec.SecurityGroupSelector can be nil if the user is using a launchTemplateName to define SecurityGroups
	// Karpenter will not drift on changes to securitygroup in the launchTemplateName
	if nodeClass.Spec.LaunchTemplateName != nil {
		return "", nil
	}
	securityGroupIds := sets.New(lo.Map(nodeClass.Status.SecurityGroups, func(sg v1beta1.SecurityGroup, _ int) string { return sg.ID })...)
	if len(securityGroupIds) == 0 {
		return "", fmt.Errorf("no security groups exist in status")
	}

	if !securityGroupIds.Equal(sets.New(ec2Instance.SecurityGroupIDs...)) {
		return SecurityGroupDrift, nil
	}
	return "", nil
}

func (c *CloudProvider) areStaticFieldsDrifted(nodeClaim *corev1beta1.NodeClaim, nodeClass *v1beta1.EC2NodeClass) cloudprovider.DriftReason {
	var ownerHashKey string
	if nodeClaim.IsMachine {
		ownerHashKey = v1alpha1.AnnotationNodeTemplateHash
	} else {
		ownerHashKey = v1beta1.AnnotationNodeClassHash
	}
	nodeClassHash, foundHashNodeClass := nodeClass.Annotations[ownerHashKey]
	nodeClaimHash, foundHashNodeClaim := nodeClaim.Annotations[ownerHashKey]
	if !foundHashNodeClass || !foundHashNodeClaim {
		return ""
	}
	if nodeClassHash != nodeClaimHash {
		return lo.Ternary(nodeClaim.IsMachine, NodeTemplateDrift, NodeClassDrift)
	}
	return ""
}

func (c *CloudProvider) getInstance(ctx context.Context, providerID string) (*instance.Instance, error) {
	// Get InstanceID to fetch from EC2
	instanceID, err := utils.ParseInstanceID(providerID)
	if err != nil {
		return nil, err
	}
	instance, err := c.instanceProvider.Get(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("getting instance, %w", err)
	}
	return instance, nil
}
