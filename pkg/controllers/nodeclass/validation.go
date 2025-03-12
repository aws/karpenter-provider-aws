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
	"math"
	"strings"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
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
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

const (
	ConditionReasonCreateFleetAuthFailed          = "CreateFleetAuthCheckFailed"
	ConditionReasonCreateLaunchTemplateAuthFailed = "CreateLaunchTemplateAuthCheckFailed"
	ConditionReasonRunInstancesAuthFailed         = "RunInstancesAuthCheckFailed"
	ConditionReasonDependenciesNotReady           = "DependenciesNotReady"
	ConditionReasonTagValidationFailed            = "TagValidationFailed"
)

var ValidationConditionMessages = map[string]string{
	ConditionReasonCreateFleetAuthFailed:          "Controller isn't authorized to call ec2:CreateFleet",
	ConditionReasonCreateLaunchTemplateAuthFailed: "Controller isn't authorized to call ec2:CreateLaunchTemplate",
	ConditionReasonRunInstancesAuthFailed:         "Controller isn't authorized to call ec2:RunInstances",
}

type Validation struct {
	ec2api                 sdk.EC2API
	amiResolver            amifamily.Resolver
	launchTemplateProvider launchtemplate.Provider
	instanceProvider       instance.Provider
	instanceTypeProvider   instancetype.Provider
	cache                  *cache.Cache
}

func NewValidationReconciler(ec2api sdk.EC2API, amiResolver amifamily.Resolver, launchTemplateProvider launchtemplate.Provider, instanceProvider instance.Provider, instanceTypeProvider instancetype.Provider, cache *cache.Cache) *Validation {
	return &Validation{
		ec2api:                 ec2api,
		amiResolver:            amiResolver,
		launchTemplateProvider: launchTemplateProvider,
		instanceProvider:       instanceProvider,
		instanceTypeProvider:   instanceTypeProvider,
		cache:                  cache,
	}
}

// nolint:gocyclo
func (v *Validation) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if _, ok := lo.Find(v.requiredConditions(), func(cond string) bool {
		return nodeClass.StatusConditions().Get(cond).IsFalse()
	}); ok {
		// If any of the required status conditions are false, we know validation will fail regardless of the other values.
		nodeClass.StatusConditions().SetFalse(
			v1.ConditionTypeValidationSucceeded,
			ConditionReasonDependenciesNotReady,
			"Awaiting AMI, Instance Profile, Security Group, and Subnet resolution",
		)
		return reconcile.Result{}, nil
	}
	if _, ok := lo.Find(v.requiredConditions(), func(cond string) bool {
		return nodeClass.StatusConditions().Get(cond).IsUnknown()
	}); ok {
		// If none of the status conditions are false, but at least one is unknown, we should also consider the validation
		// state to be unknown. Once all required conditions collapse to a true or false state, we can test validation.
		nodeClass.StatusConditions().SetUnknownWithReason(
			v1.ConditionTypeValidationSucceeded,
			ConditionReasonDependenciesNotReady,
			"Awaiting AMI, Instance Profile, Security Group, and Subnet resolution",
		)
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
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeValidationSucceeded, ConditionReasonTagValidationFailed, err.Error())
		return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("validating tags, %w", err))
	}

	if val, ok := v.cache.Get(v.cacheKey(nodeClass, tags)); ok {
		// We still update the status condition even if it's cached since we may have had a conflict error previously
		if val == "" {
			nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		} else {
			nodeClass.StatusConditions().SetFalse(
				v1.ConditionTypeValidationSucceeded,
				val.(string),
				ValidationConditionMessages[val.(string)],
			)
		}
		return reconcile.Result{}, nil
	}
	for _, isValid := range []validatorFunc{
		v.validateCreateFleetAuthorization,
		v.validateCreateLaunchTemplateAuthorization,
		v.validateRunInstancesAuthorization,
	} {
		if failureReason, requeue, err := isValid(ctx, nodeClass, nodeClaim, tags); err != nil {
			return reconcile.Result{}, err
		} else if requeue {
			return reconcile.Result{Requeue: true}, nil
		} else if failureReason != "" {
			v.cache.SetDefault(v.cacheKey(nodeClass, tags), failureReason)
			nodeClass.StatusConditions().SetFalse(
				v1.ConditionTypeValidationSucceeded,
				failureReason,
				ValidationConditionMessages[failureReason],
			)
			return reconcile.Result{}, nil
		}
	}

	v.cache.SetDefault(v.cacheKey(nodeClass, tags), "")
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{}, nil
}

type validatorFunc func(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim, map[string]string) (string, bool, error)

func (v *Validation) validateCreateFleetAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	_ *karpv1.NodeClaim,
	tags map[string]string,
) (reason string, requeue bool, err error) {
	createFleetInput := instance.GetCreateFleetInput(nodeClass, karpv1.CapacityTypeOnDemand, tags, mockLaunchTemplateConfig())
	createFleetInput.DryRun = lo.ToPtr(true)
	if _, err := v.ec2api.CreateFleet(ctx, createFleetInput); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return "", false, fmt.Errorf("validating ec2:CreateFleet authorization, %w", err)
		}
		return ConditionReasonCreateFleetAuthFailed, false, nil
	}
	return "", false, nil
}

func (v *Validation) validateCreateLaunchTemplateAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	tags map[string]string,
) (reason string, requeue bool, err error) {
	options, err := v.mockOptions(ctx, nodeClaim, nodeClass, tags)
	createLaunchTemplateInput := launchtemplate.GetCreateLaunchTemplateInput(ctx, options[0], corev1.IPv4Protocol, "")
	createLaunchTemplateInput.DryRun = lo.ToPtr(true)
	if _, err := v.ec2api.CreateLaunchTemplate(ctx, createLaunchTemplateInput); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return "", false, fmt.Errorf("validating ec2:CreateLaunchTemplates authorization, %w", err)
		}
		return ConditionReasonCreateLaunchTemplateAuthFailed, false, nil
	}
	return "", false, nil
}

func (v *Validation) validateRunInstancesAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	tags map[string]string,
) (reason string, requeue bool, err error) {
	// NOTE: Since we've already validated the status conditions are true, these should never occur
	if len(nodeClass.Status.AMIs) == 0 {
		return "", false, fmt.Errorf("no resolved amis in status")
	}
	if len(nodeClass.Status.Subnets) == 0 {
		return "", false, fmt.Errorf("no resolved subnets in status")
	}
	if len(nodeClass.Status.SecurityGroups) == 0 {
		return "", false, fmt.Errorf("no resolved security groups in status")
	}
	if nodeClass.Status.InstanceProfile == "" {
		return "", false, fmt.Errorf("no instance profile in status")
	}

	options, err := v.mockOptions(ctx, nodeClaim, nodeClass, tags)
	userdata, err := options[0].UserData.Script()

	runInstancesInput := &ec2.RunInstancesInput{
		DryRun:       lo.ToPtr(true),
		MaxCount:     lo.ToPtr[int32](1),
		MinCount:     lo.ToPtr[int32](1),
		InstanceType: ec2types.InstanceType(options[0].InstanceTypes[0].Name),
		MetadataOptions: &ec2types.InstanceMetadataOptionsRequest{
			HttpEndpoint:     ec2types.InstanceMetadataEndpointState(lo.FromPtr(options[0].MetadataOptions.HTTPEndpoint)),
			HttpTokens:       ec2types.HttpTokensState(lo.FromPtr(options[0].MetadataOptions.HTTPTokens)),
			HttpProtocolIpv6: ec2types.InstanceMetadataProtocolState(lo.FromPtr(options[0].MetadataOptions.HTTPProtocolIPv6)),
			//aws sdk v2 changed this type to *int32 instead of *int64
			//nolint: gosec
			HttpPutResponseHopLimit: lo.ToPtr(int32(lo.FromPtr(options[0].MetadataOptions.HTTPPutResponseHopLimit))),
		},
		Monitoring: &ec2types.RunInstancesMonitoringEnabled{
			// Default Enabled to False if not specified
			Enabled: lo.ToPtr(lo.FromPtr(nodeClass.Spec.DetailedMonitoring)),
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
		ImageId: lo.ToPtr(options[0].AMIID),
		IamInstanceProfile: &ec2types.IamInstanceProfileSpecification{
			Name: lo.ToPtr(nodeClass.Status.InstanceProfile),
		},
		UserData:            lo.ToPtr(userdata),
		BlockDeviceMappings: blockDeviceMappings(options[0].BlockDeviceMappings),
		// EC2 dry-run doesn't validate the number of IPs, so it's safe to take the first subnet here
		// even if that subnet has no more IPv4 or IPv6 addresses to give out
		NetworkInterfaces: []ec2types.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: nodeClass.Spec.AssociatePublicIPAddress,
				DeviceIndex:              lo.ToPtr[int32](0),
				Groups:                   lo.Map(nodeClass.Status.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID }),
				SubnetId:                 lo.ToPtr(nodeClass.Status.Subnets[0].ID),
			},
		},
		SecurityGroupIds: lo.Map(nodeClass.Status.SecurityGroups, func(s v1.SecurityGroup, _ int) string { return s.ID }),
	}

	if _, err = v.ec2api.RunInstances(ctx, runInstancesInput); awserrors.IgnoreDryRunError(err) != nil {
		// If we get InstanceProfile NotFound, but we have a resolved instance profile in the status,
		// this means there is most likely an eventual consistency issue and we just need to requeue
		if awserrors.IsInstanceProfileNotFound(err) {
			return "", true, nil
		}
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return "", false, fmt.Errorf("validating ec2:RunInstances authorization, %w", err)
		}
		return ConditionReasonRunInstancesAuthFailed, false, nil
	}
	return "", false, nil
}

func (*Validation) requiredConditions() []string {
	return []string{
		v1.ConditionTypeAMIsReady,
		v1.ConditionTypeInstanceProfileReady,
		v1.ConditionTypeSecurityGroupsReady,
		v1.ConditionTypeSubnetsReady,
	}
}

func (*Validation) cacheKey(nodeClass *v1.EC2NodeClass, tags map[string]string) string {
	hash := lo.Must(hashstructure.Hash([]interface{}{
		nodeClass.Status.Subnets,
		nodeClass.Status.SecurityGroups,
		nodeClass.Status.AMIs,
		nodeClass.Status.InstanceProfile,
		nodeClass.Spec.MetadataOptions,
		nodeClass.Spec.BlockDeviceMappings,
		tags,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	return fmt.Sprintf("%s:%016x", nodeClass.Name, hash)
}

// clearCacheEntries removes all cache entries associated with the given nodeclass from the validation cache
func (v *Validation) clearCacheEntries(nodeClass *v1.EC2NodeClass) {
	var toDelete []string
	for key := range v.cache.Items() {
		parts := strings.Split(key, ":")
		// NOTE: should never occur, indicates malformed cache key
		if len(parts) != 2 {
			continue
		}
		if parts[0] == nodeClass.Name {
			toDelete = append(toDelete, key)
		}
	}
	for _, key := range toDelete {
		v.cache.Delete(key)
	}
}

func mockLaunchTemplateConfig() []ec2types.FleetLaunchTemplateConfigRequest {
	return []ec2types.FleetLaunchTemplateConfigRequest{
		{
			LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: lo.ToPtr("mock-lt-name"),
				LaunchTemplateId:   lo.ToPtr("lt-1234567890abcdef0"),
				Version:            lo.ToPtr("1"),
			},
			Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
				{
					InstanceType: ec2types.InstanceTypeT3Micro,
					SubnetId:     lo.ToPtr("subnet-1234567890abcdef0"),
				},
				{
					InstanceType: ec2types.InstanceTypeT3Small,
					SubnetId:     lo.ToPtr("subnet-1234567890abcdef1"),
				},
			},
		},
	}
}

func (v *Validation) mockOptions(ctx context.Context, nodeClaim *karpv1.NodeClaim, nodeClass *v1.EC2NodeClass, tags map[string]string) ([]*amifamily.LaunchTemplate, error) {
	instancetypes := v.instanceTypeProvider.ResolveInstanceTypes(ctx, nodeClass, lo.Must(hashstructure.Hash(nodeClass.Status.AMIs, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})))
	capacityType := v.instanceProvider.GetCapacityType(nodeClaim, instancetypes)
	amioptions, err := v.launchTemplateProvider.CreateAMIOptions(ctx, nodeClass, lo.Assign(nodeClaim.Labels, map[string]string{karpv1.CapacityTypeLabelKey: capacityType}), tags)
	if err != nil {
		return []*amifamily.LaunchTemplate{}, err
	}
	return v.amiResolver.Resolve(nodeClass, nodeClaim, instancetypes, capacityType, amioptions)
}

func blockDeviceMappings(blockDeviceMappings []*v1.BlockDeviceMapping) []ec2types.BlockDeviceMapping {
	if len(blockDeviceMappings) == 0 {
		// The EC2 API fails with empty slices and expects nil.
		return nil
	}
	var blockDeviceMappingsRequest []ec2types.BlockDeviceMapping
	for _, blockDeviceMapping := range blockDeviceMappings {
		blockDeviceMappingsRequest = append(blockDeviceMappingsRequest, ec2types.BlockDeviceMapping{
			DeviceName: blockDeviceMapping.DeviceName,
			Ebs: &ec2types.EbsBlockDevice{
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
				VolumeSize: volumeSize(blockDeviceMapping.EBS.VolumeSize),
			},
		})
	}
	return blockDeviceMappingsRequest
}

// volumeSize returns a GiB scaled value from a resource quantity or nil if the resource quantity passed in is nil
func volumeSize(quantity *resource.Quantity) *int32 {
	if quantity == nil {
		return nil
	}
	// Converts the value to Gi and rounds up the value to the nearest Gi
	return lo.ToPtr(int32(math.Ceil(quantity.AsApproximateFloat64() / math.Pow(2, 30))))
}
