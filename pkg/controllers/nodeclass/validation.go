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
	"fmt"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Validation struct {
	ec2api sdk.EC2API

	amiProvider amifamily.Provider
}

// nolint:gocyclo
func (n Validation) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	// Tag Validation
	if offendingTag, found := lo.FindKeyBy(nodeClass.Spec.Tags, func(k string, v string) bool {
		for _, exp := range v1.RestrictedTagPatterns {
			if exp.MatchString(k) {
				return true
			}
		}
		return false
	}); found {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "TagValidationFailed",
			fmt.Sprintf("%q tag does not pass tag validation requirements", offendingTag))
		return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("%q tag does not pass tag validation requirements", offendingTag))
	}
	// Auth Validation
	if !nodeClass.StatusConditions().Get(v1.ConditionTypeSecurityGroupsReady).IsTrue() || !nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsTrue() || !nodeClass.StatusConditions().Get(v1.ConditionTypeInstanceProfileReady).IsTrue() || !nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsTrue() {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "DependenciesNotReady", "Waiting for SecurityGroups, AMIs, Subnets and InstanceProfiles to go true")

		return reconcile.Result{}, nil
	}
	nodeClaim := &karpv1.NodeClaim{
		Spec: karpv1.NodeClaimSpec{
			NodeClassRef: &karpv1.NodeClassReference{
				Name: nodeClass.ObjectMeta.Name,
			},
		},
	}
	tags, err := utils.GetTags(nodeClass, nodeClaim, options.FromContext(ctx).ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting tags, %w", err)
	}

	createFleetInput := instance.GetCreateFleetInput(nodeClass, string(karpv1.CapacityTypeOnDemand), tags, mockLaunchTemplateConfig())
	createFleetInput.DryRun = aws.Bool(true)

	if _, err := n.ec2api.CreateFleet(ctx, createFleetInput); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return reconcile.Result{}, fmt.Errorf("unexpected error during CreateFleet validation: %w", err)
		}
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "CreateFleetAuthCheckFailed", "Controller isn't authorized to call CreateFleet")
		return reconcile.Result{}, nil
	}

	createLaunchTemplateInput := launchtemplate.GetCreateLaunchTemplateInput(ctx, mockOptions(*nodeClaim, nodeClass, tags), corev1.IPv4Protocol, "")
	createLaunchTemplateInput.DryRun = aws.Bool(true)

	if _, err := n.ec2api.CreateLaunchTemplate(ctx, createLaunchTemplateInput); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return reconcile.Result{}, fmt.Errorf("unexpected error during CreateLaunchTemplate validation: %w", err)
		}
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "CreateLaunchTemplateAuthCheckFailed", "Controller isn't authorized to call CreateLaunchTemplate")
		return reconcile.Result{}, nil
	}

	// This should never occur as AMIs should already be resolved during the AMI resolution phase
	if len(nodeClass.Status.AMIs) == 0 {
		return reconcile.Result{}, fmt.Errorf("no resolved AMIs in status: %w", err)
	}

	var instanceType ec2types.InstanceType
	requirements := scheduling.NewNodeSelectorRequirements(nodeClass.Status.AMIs[0].Requirements...)

	if requirements.Get(corev1.LabelArchStable).Has(karpv1.ArchitectureAmd64) {
		instanceType = ec2types.InstanceTypeM5Large
	} else if requirements.Get(corev1.LabelArchStable).Has(karpv1.ArchitectureArm64) {
		instanceType = ec2types.InstanceTypeM6gLarge
	}

	runInstancesInput := &ec2.RunInstancesInput{
		DryRun:       lo.ToPtr(true),
		MaxCount:     aws.Int32(1),
		MinCount:     aws.Int32(1),
		InstanceType: instanceType,
		MetadataOptions: &ec2types.InstanceMetadataOptionsRequest{
			HttpEndpoint:     ec2types.InstanceMetadataEndpointState(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPEndpoint)),
			HttpTokens:       ec2types.HttpTokensState(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPTokens)),
			HttpProtocolIpv6: ec2types.InstanceMetadataProtocolState(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)),
			//aws sdk v2 changed this type to *int32 instead of *int64
			//nolint: gosec
			HttpPutResponseHopLimit: aws.Int32(int32(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit))),
		},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags:         utils.MergeTags(tags),
			},
			{
				ResourceType: ec2types.ResourceTypeVolume,
				Tags:         utils.MergeTags(tags),
			},
			{
				ResourceType: ec2types.ResourceTypeNetworkInterface,
				Tags:         utils.MergeTags(tags),
			},
		},
		ImageId: lo.ToPtr(nodeClass.Status.AMIs[0].ID),
	}

	if _, err = n.ec2api.RunInstances(ctx, runInstancesInput); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return reconcile.Result{}, fmt.Errorf("unexpected error during RunInstances validation: %w", err)
		}
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "RunInstancesAuthCheckFailed", "Controller isn't authorized to call RunInstances")
		return reconcile.Result{}, nil
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{}, nil
}

func mockLaunchTemplateConfig() []ec2types.FleetLaunchTemplateConfigRequest {
	return []ec2types.FleetLaunchTemplateConfigRequest{
		{
			LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: aws.String("mock-lt-name"),
				LaunchTemplateId:   aws.String("lt-1234567890abcdef0"),
				Version:            aws.String("1"),
			},
			Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
				{
					InstanceType: ec2types.InstanceTypeT3Micro,
					SubnetId:     aws.String("subnet-1234567890abcdef0"),
				},
				{
					InstanceType: ec2types.InstanceTypeT3Small,
					SubnetId:     aws.String("subnet-1234567890abcdef1"),
				},
			},
		},
	}
}
func mockOptions(nodeClaim karpv1.NodeClaim, nodeClass *v1.EC2NodeClass, tags map[string]string) *amifamily.LaunchTemplate {
	return &amifamily.LaunchTemplate{
		Options: &amifamily.Options{
			Tags:            tags,
			InstanceProfile: nodeClass.Status.InstanceProfile,
			SecurityGroups:  nodeClass.Status.SecurityGroups,
		},
		MetadataOptions: &v1.MetadataOptions{
			HTTPEndpoint:            nodeClass.Spec.MetadataOptions.HTTPEndpoint,
			HTTPTokens:              nodeClass.Spec.MetadataOptions.HTTPTokens,
			HTTPProtocolIPv6:        nodeClass.Spec.MetadataOptions.HTTPProtocolIPv6,
			HTTPPutResponseHopLimit: nodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit,
		},
		AMIID:               nodeClaim.Status.ImageID,
		BlockDeviceMappings: nodeClass.Spec.BlockDeviceMappings,
	}
}
