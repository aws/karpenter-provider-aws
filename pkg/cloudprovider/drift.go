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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

const (
	AMIDrift               cloudprovider.DriftReason = "AMIDrift"
	StaleInstanceTypeDrift cloudprovider.DriftReason = "StaleInstanceTypeDrift"
	SubnetDrift            cloudprovider.DriftReason = "SubnetDrift"
	SecurityGroupDrift     cloudprovider.DriftReason = "SecurityGroupDrift"
	NodeClassDrift         cloudprovider.DriftReason = "NodeClassDrift"
)

func (c *CloudProvider) isNodeClassDrifted(ctx context.Context, nodeClaim *karpv1.NodeClaim, nodePool *karpv1.NodePool, nodeClass *v1.EC2NodeClass) (cloudprovider.DriftReason, error) {
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

func (c *CloudProvider) isAMIDrifted(ctx context.Context, nodeClaim *karpv1.NodeClaim, nodePool *karpv1.NodePool,
	instance *instance.Instance, nodeClass *v1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	instanceTypes, err := c.GetInstanceTypes(ctx, nodePool)
	if err != nil {
		return "", fmt.Errorf("getting instanceTypes, %w", err)
	}

	if _, ok := nodeClaim.Labels[corev1.LabelInstanceTypeStable]; !ok {
		return "", fmt.Errorf("nodeclaim doesn't have instance type label")
	}
	nodeInstanceType, found := lo.Find(instanceTypes, func(instType *cloudprovider.InstanceType) bool {
		return instType.Name == nodeClaim.Labels[corev1.LabelInstanceTypeStable]
	})
	// If we can't find the instance type, that means that we previously thought this was a valid instance type, but no longer is.
	// This should only happen when the result of the DescribeInstanceTypes API changes over time, and an existing node has an instance type
	// that should no longer be used.
	if !found {
		log.FromContext(ctx).V(1).Info(fmt.Sprintf("failed to find node instance type %q, continuing to drift", nodeClaim.Labels[corev1.LabelInstanceTypeStable]))
		return StaleInstanceTypeDrift, nil
	}
	if len(nodeClass.Status.AMIs) == 0 {
		return "", fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := amifamily.MapToInstanceTypes([]*cloudprovider.InstanceType{nodeInstanceType}, nodeClass.Status.AMIs)
	if !lo.Contains(lo.Keys(mappedAMIs), instance.ImageID) {
		return AMIDrift, nil
	}
	return "", nil
}

// Checks if the security groups are drifted, by comparing the subnet returned from the subnetProvider
// to the ec2 instance subnets
func (c *CloudProvider) isSubnetDrifted(instance *instance.Instance, nodeClass *v1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	// subnets need to be found to check for drift
	if len(nodeClass.Status.Subnets) == 0 {
		return "", fmt.Errorf("no subnets are discovered")
	}

	_, found := lo.Find(nodeClass.Status.Subnets, func(subnet v1.Subnet) bool {
		return subnet.ID == instance.SubnetID
	})

	if !found {
		return SubnetDrift, nil
	}
	return "", nil
}

// Checks if the security groups are drifted, by comparing the security groups returned from the SecurityGroupProvider
// to the ec2 instance security groups
func (c *CloudProvider) areSecurityGroupsDrifted(ec2Instance *instance.Instance, nodeClass *v1.EC2NodeClass) (cloudprovider.DriftReason, error) {
	securityGroupIds := sets.New(lo.Map(nodeClass.Status.SecurityGroups, func(sg v1.SecurityGroup, _ int) string { return sg.ID })...)
	if len(securityGroupIds) == 0 {
		return "", fmt.Errorf("no security groups are present in the status")
	}

	if !securityGroupIds.Equal(sets.New(ec2Instance.SecurityGroupIDs...)) {
		return SecurityGroupDrift, nil
	}
	return "", nil
}

func (c *CloudProvider) areStaticFieldsDrifted(nodeClaim *karpv1.NodeClaim, nodeClass *v1.EC2NodeClass) cloudprovider.DriftReason {
	nodeClassHash, foundNodeClassHash := nodeClass.Annotations[v1.AnnotationEC2NodeClassHash]
	nodeClassHashVersion, foundNodeClassHashVersion := nodeClass.Annotations[v1.AnnotationEC2NodeClassHashVersion]
	nodeClaimHash, foundNodeClaimHash := nodeClaim.Annotations[v1.AnnotationEC2NodeClassHash]
	nodeClaimHashVersion, foundNodeClaimHashVersion := nodeClaim.Annotations[v1.AnnotationEC2NodeClassHashVersion]

	if !foundNodeClassHash || !foundNodeClaimHash || !foundNodeClassHashVersion || !foundNodeClaimHashVersion {
		return ""
	}
	// validate that the hash version for the EC2NodeClass is the same as the NodeClaim before evaluating for static drift
	if nodeClassHashVersion != nodeClaimHashVersion {
		return ""
	}
	return lo.Ternary(nodeClassHash != nodeClaimHash, NodeClassDrift, "")
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
