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
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Validation struct {
	ec2api sdk.EC2API

	amiProvider amifamily.Provider
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

	createFleetInput := &ec2.CreateFleetInput{
		DryRun:  lo.ToPtr(true),
		Type:    ec2types.FleetTypeInstant,
		Context: nodeClass.Spec.Context,
		LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateId: aws.String("lt-1234567890abcdef0"),
					Version:          aws.String("1"),
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
		},
		TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: karpv1.CapacityTypeOnDemand,
			TotalTargetCapacity:       aws.Int32(1),
		},
		TagSpecifications: []ec2types.TagSpecification{
			{ResourceType: ec2types.ResourceTypeInstance, Tags: utils.MergeTags(tags)},
			{ResourceType: ec2types.ResourceTypeVolume, Tags: utils.MergeTags(tags)},
			{ResourceType: ec2types.ResourceTypeFleet, Tags: utils.MergeTags(tags)},
		},
	}

	if _, err := n.ec2api.CreateFleet(ctx, createFleetInput); awserrors.IsUnauthorizedError(err) {
		errs = append(errs, fmt.Errorf("create fleet"))
	}

	networkInterfaces := []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{
		{
			DeviceIndex: aws.Int32(0),
			SubnetId:    aws.String(nodeClass.Status.Subnets[0].ID),
			Groups: lo.Map(nodeClass.Status.SecurityGroups, func(sg v1.SecurityGroup, _ int) string {
				return sg.ID
			}),
			DeleteOnTermination:      aws.Bool(true),
			AssociatePublicIpAddress: aws.Bool(true),
			NetworkCardIndex:         aws.Int32(0),
			InterfaceType:            aws.String(string(ec2types.NetworkInterfaceTypeEfa)),
		},
	}

	launchTemplateDataTags := []ec2types.LaunchTemplateTagSpecificationRequest{
		{ResourceType: ec2types.ResourceTypeNetworkInterface, Tags: utils.MergeTags(tags)},
	}

	var userData *string
	if nodeClass.Spec.UserData != nil {
		encoded := base64.StdEncoding.EncodeToString([]byte(*nodeClass.Spec.UserData))
		userData = &encoded
	}

	createLaunchTemplateInput := &ec2.CreateLaunchTemplateInput{
		//this one is not a dry run because we need a launch template anyways
		LaunchTemplateName: lo.ToPtr(fmt.Sprintf("lt-%d", time.Now().UnixNano())),
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			BlockDeviceMappings: blockDeviceMappings(nodeClass.Spec.BlockDeviceMappings),
			IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(nodeClass.Status.InstanceProfile),
			},
			Monitoring: &ec2types.LaunchTemplatesMonitoringRequest{
				Enabled: nodeClass.Spec.DetailedMonitoring,
			},

			SecurityGroupIds: lo.Ternary(networkInterfaces != nil, nil, lo.Map(nodeClass.Status.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID })),
			UserData:         userData,
			ImageId:          aws.String(nodeClaim.Status.ImageID),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptionsRequest{
				HttpEndpoint:     ec2types.LaunchTemplateInstanceMetadataEndpointState(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPEndpoint)),
				HttpProtocolIpv6: ec2types.LaunchTemplateInstanceMetadataProtocolIpv6(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)),
				//Will be removed when we update options.MetadataOptions.HTTPPutResponseHopLimit type to be int32
				//nolint: gosec
				HttpPutResponseHopLimit: lo.ToPtr(int32(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit))),
				HttpTokens:              ec2types.LaunchTemplateHttpTokensState(lo.FromPtr(nodeClass.Spec.MetadataOptions.HTTPTokens)),
				InstanceMetadataTags:    ec2types.LaunchTemplateInstanceMetadataTagsStateDisabled,
			},
			NetworkInterfaces: networkInterfaces,
			TagSpecifications: launchTemplateDataTags,
		},
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags:         utils.MergeTags(tags),
			},
		},
	}

	describeLaunchTemplatesInput := &ec2.DescribeLaunchTemplatesInput{
		DryRun:              lo.ToPtr(true),
		LaunchTemplateNames: []string{"mock-lt-name"},
	}

	if _, err := n.ec2api.DescribeLaunchTemplates(ctx, describeLaunchTemplatesInput); awserrors.IsUnauthorizedError(err) {
		errs = append(errs, fmt.Errorf("describe launch template"))
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("unauthorized operation %v", errors.Join(errs...)))
		//returning here because run instances depends on being able to create a launch template and delete launch template needs describe launch template
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", errors.Join(errs...))
	}

	lt, err := n.ec2api.CreateLaunchTemplate(ctx, createLaunchTemplateInput)

	if awserrors.IsUnauthorizedError(err) {
		errs = append(errs, fmt.Errorf("create launch template"))
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("unauthorized operation %v", errors.Join(errs...)))
		//returning here because run instances depends on being able to create a launch template
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", errors.Join(errs...))
	}

	imageOutput, err := n.ec2api.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amis[0].AmiID},
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	describeInstanceTypesInput := &ec2.DescribeInstanceTypesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("processor-info.supported-architecture"),
				Values: []string{string(imageOutput.Images[0].Architecture)},
			},
		},
	}
	instancetypes, err := n.ec2api.DescribeInstanceTypes(ctx, describeInstanceTypesInput)
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
		},
		InstanceType: instancetypes.InstanceTypes[0].InstanceType,
		IamInstanceProfile: &ec2types.IamInstanceProfileSpecification{
			Name: aws.String(nodeClass.Status.InstanceProfile),
		},
		LaunchTemplate: &ec2types.LaunchTemplateSpecification{
			LaunchTemplateName: lt.LaunchTemplate.LaunchTemplateName,
			Version:            aws.String("1"),
		},
	}
	if _, err := n.ec2api.RunInstances(ctx, runInstancesInput); awserrors.IsUnauthorizedError(err) {
		errs = append(errs, fmt.Errorf("run instances"))
	}

	deleteLaunchTemplateInput := ec2.DeleteLaunchTemplateInput{
		LaunchTemplateName: lt.LaunchTemplate.LaunchTemplateName,
	}

	_, err = n.ec2api.DeleteLaunchTemplate(ctx, &deleteLaunchTemplateInput)
	if err != nil {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("delete launch template: %v", err))
		return reconcile.Result{}, fmt.Errorf("delete launch template: %w", err)

	}

	if errors.Join(errs...) != nil {
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, "NodeClassNotReady", fmt.Sprintf("unauthorized operation %v", errors.Join(errs...)))
		return reconcile.Result{}, fmt.Errorf("unauthorized operation %w", errors.Join(errs...))
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{}, nil
}

func blockDeviceMappings(blockDeviceMappings []*v1.BlockDeviceMapping) []ec2types.LaunchTemplateBlockDeviceMappingRequest {
	if len(blockDeviceMappings) == 0 {
		return nil
	}
	var blockDeviceMappingsRequest []ec2types.LaunchTemplateBlockDeviceMappingRequest
	for _, blockDeviceMapping := range blockDeviceMappings {
		blockDeviceMappingsRequest = append(blockDeviceMappingsRequest, ec2types.LaunchTemplateBlockDeviceMappingRequest{
			DeviceName: blockDeviceMapping.DeviceName,
			Ebs: &ec2types.LaunchTemplateEbsBlockDeviceRequest{
				DeleteOnTermination: blockDeviceMapping.EBS.DeleteOnTermination,
				Encrypted:           blockDeviceMapping.EBS.Encrypted,
				VolumeType:          ec2types.VolumeType(aws.ToString(blockDeviceMapping.EBS.VolumeType)),
				//Lints here can be removed when we update options.EBS.IOPS and Throughput type to be int32
				//nolint: gosec
				Iops: lo.EmptyableToPtr(int32(lo.FromPtr(blockDeviceMapping.EBS.IOPS))),
				//nolint: gosec
				Throughput: lo.EmptyableToPtr(int32(lo.FromPtr(blockDeviceMapping.EBS.Throughput))),
				KmsKeyId:   blockDeviceMapping.EBS.KMSKeyID,
				SnapshotId: blockDeviceMapping.EBS.SnapshotID,
				VolumeSize: lo.ToPtr(int32(blockDeviceMapping.EBS.VolumeSize.AsApproximateFloat64())),
			},
		})
	}
	return blockDeviceMappingsRequest
}
