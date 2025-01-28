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
	"errors"
	"fmt"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

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

	amiProvider            amifamily.Provider
	instanceProvider       instance.Provider
	launchTemplateProvider launchtemplate.Provider
}

//nolint:gocyclo
func (n Validation) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "reconcile", true)

	//Tag Validation
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
	//Auth Validation
	amis, err := n.amiProvider.List(ctx, nodeClass)
	if err != nil {
		return reconcile.Result{}, err
	}
	if nodeClass.StatusConditions().Get(v1.ConditionTypeSubnetsReady).IsFalse() || nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsFalse() {
		return reconcile.Result{}, nil
	}
	nodeClaim := &karpv1.NodeClaim{
		Spec: karpv1.NodeClaimSpec{
			NodeClassRef: &karpv1.NodeClassReference{
				Name: nodeClass.ObjectMeta.Name,
			},
		},
		Status: karpv1.NodeClaimStatus{
			ImageID: amis[0].AmiID,
		},
	}
	tags, err := utils.GetTags(ctx, nodeClass, nodeClaim, options.FromContext(ctx).ClusterName)
	if err != nil {
		return reconcile.Result{}, err
	}
	var errs []error

	createFleetInput := n.instanceProvider.GetCreateFleetInput(nodeClass, string(karpv1.CapacityTypeOnDemand), tags, mockLaunchTemplateConfig(), true)

	if _, err := n.ec2api.CreateFleet(ctx, createFleetInput); awserrors.IsNotDryRunError(err) {
		errs = append(errs, fmt.Errorf("create fleet %w", err))
	}

	createLaunchTemplateInput := n.launchTemplateProvider.GetCreateLaunchTemplateInput(mockOptions(*nodeClaim, nodeClass, tags), corev1.IPv4Protocol, "", true)

	if _, err := n.ec2api.CreateLaunchTemplate(ctx, createLaunchTemplateInput); awserrors.IsNotDryRunError(err) {
		errs = append(errs, fmt.Errorf("create launch template %w", err))
	}

	imageOutput, err := n.ec2api.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amis[0].AmiID},
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	runInstancesInput := &ec2.RunInstancesInput{
		DryRun:   lo.ToPtr(true),
		MaxCount: aws.Int32(1),
		MinCount: aws.Int32(1),
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
		ImageId: imageOutput.Images[0].ImageId,
	}

	if _, err = n.ec2api.RunInstances(ctx, runInstancesInput); awserrors.IsNotDryRunError(err) {
		errs = append(errs, fmt.Errorf("run instances %w", err))
	}

	if errors.Join(errs...) != nil {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("unauthorized operation %v", errors.Join(errs...)))
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", errors.Join(errs...))
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
			InstanceProfile: lo.FromPtr(nodeClass.Spec.InstanceProfile),
			SecurityGroups:  nodeClass.Status.SecurityGroups,
		},
		MetadataOptions: &v1.MetadataOptions{
			HTTPEndpoint:            nodeClass.Spec.MetadataOptions.HTTPEndpoint,
			HTTPTokens:              nodeClass.Spec.MetadataOptions.HTTPTokens,
			HTTPProtocolIPv6:        nodeClass.Spec.MetadataOptions.HTTPProtocolIPv6,
			HTTPPutResponseHopLimit: aws.Int64(1),
		},
		AMIID:               nodeClaim.Status.ImageID,
		BlockDeviceMappings: nodeClass.Spec.BlockDeviceMappings,
	}
}
