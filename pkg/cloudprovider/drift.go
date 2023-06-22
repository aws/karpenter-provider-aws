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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/sets"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/utils"
)

func (c *CloudProvider) isNodeTemplateDrifted(ctx context.Context, machine *v1alpha5.Machine, provisioner *v1alpha5.Provisioner, nodeTemplate *v1alpha1.AWSNodeTemplate) (bool, error) {
	// nodeTemplate selectors can be nil if the user is using a launchTemplateName to define their subnets, security groups, and AMIs.
	// Karpenter will not drift on changes if launchTemplateName is used.
	if nodeTemplate.Spec.LaunchTemplateName != nil {
		return false, nil
	}
	instance, err := c.getInstance(ctx, machine.Status.ProviderID)
	if err != nil {
		return false, err
	}
	amiDrifted, err := c.isAMIDrifted(ctx, machine, provisioner, instance, nodeTemplate)
	if err != nil {
		return false, fmt.Errorf("calculating ami drift, %w", err)
	}
	securitygroupDrifted, err := c.areSecurityGroupsDrifted(instance, nodeTemplate)
	if err != nil {
		return false, fmt.Errorf("calculating securitygroup drift, %w", err)
	}
	subnetDrifted, err := c.isSubnetDrifted(instance, nodeTemplate)
	if err != nil {
		return false, fmt.Errorf("calculating subnet drift, %w", err)
	}

	return amiDrifted || securitygroupDrifted || subnetDrifted, nil
}

func (c *CloudProvider) isAMIDrifted(ctx context.Context, machine *v1alpha5.Machine, provisioner *v1alpha5.Provisioner,
	instance *instance.Instance, nodeTemplate *v1alpha1.AWSNodeTemplate) (bool, error) {
	instanceTypes, err := c.GetInstanceTypes(ctx, provisioner)
	if err != nil {
		return false, fmt.Errorf("getting instanceTypes, %w", err)
	}
	nodeInstanceType, found := lo.Find(instanceTypes, func(instType *cloudprovider.InstanceType) bool {
		return instType.Name == machine.Labels[v1.LabelInstanceTypeStable]
	})
	if !found {
		return false, fmt.Errorf(`finding node instance type "%s"`, machine.Labels[v1.LabelInstanceTypeStable])
	}
	amis, err := c.amiProvider.Get(ctx, nodeTemplate, &amifamily.Options{})
	if err != nil {
		return false, fmt.Errorf("getting amis, %w", err)
	}
	if len(amis) == 0 {
		return false, fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := amifamily.MapInstanceTypes(amis, []*cloudprovider.InstanceType{nodeInstanceType})
	if len(mappedAMIs) == 0 {
		return false, fmt.Errorf("no instance types satisfy requirements of amis %v,", amis)
	}
	return !lo.Contains(lo.Keys(mappedAMIs), instance.ImageID), nil
}

func (c *CloudProvider) isSubnetDrifted(instance *instance.Instance, nodeTemplate *v1alpha1.AWSNodeTemplate) (bool, error) {
	// If the node template status does not have subnets, wait for the subnets to be populated before continuing
	if nodeTemplate.Status.Subnets == nil {
		return false, fmt.Errorf("AWSNodeTemplate has no subnets")
	}
	_, found := lo.Find(nodeTemplate.Status.Subnets, func(subnet v1alpha1.Subnet) bool {
		return subnet.ID == instance.SubnetID
	})
	return !found, nil
}

// Checks if the security groups are drifted, by comparing the AWSNodeTemplate.Status.SecurityGroups
// to the ec2 instance security groups
func (c *CloudProvider) areSecurityGroupsDrifted(ec2Instance *instance.Instance, nodeTemplate *v1alpha1.AWSNodeTemplate) (bool, error) {
	securityGroupIds := sets.New(lo.Map(nodeTemplate.Status.SecurityGroups, func(sg v1alpha1.SecurityGroup, _ int) string { return sg.ID })...)
	if len(securityGroupIds) == 0 {
		return false, fmt.Errorf("no security groups exist in the AWSNodeTemplate Status")
	}
	return !securityGroupIds.Equal(sets.New(ec2Instance.SecurityGroupIDs...)), nil
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
