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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/awslabs/operatorpkg/status"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
	ConditionReasonDryRunDisabled                 = "DryRunDisabled"
)

var ValidationConditionMessages = map[string]string{
	ConditionReasonCreateFleetAuthFailed:          "Controller isn't authorized to call ec2:CreateFleet",
	ConditionReasonCreateLaunchTemplateAuthFailed: "Controller isn't authorized to call ec2:CreateLaunchTemplate",
	ConditionReasonRunInstancesAuthFailed:         "Controller isn't authorized to call ec2:RunInstances",
}

type validationContext struct {
	instanceTypes     []*cloudprovider.InstanceType
	launchTemplate    *launchtemplate.LaunchTemplate
	runInstancesInput *ec2.RunInstancesInput
}

type Validation struct {
	kubeClient             client.Client
	cloudProvider          cloudprovider.CloudProvider
	ec2api                 sdk.EC2API
	amiResolver            amifamily.Resolver
	instanceTypeProvider   instancetype.Provider
	launchTemplateProvider launchtemplate.Provider
	cache                  *cache.Cache
	dryRunDisabled         bool
}

func NewValidationReconciler(
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
	ec2api sdk.EC2API,
	amiResolver amifamily.Resolver,
	instanceTypeProvider instancetype.Provider,
	launchTemplateProvider launchtemplate.Provider,
	cache *cache.Cache,
	dryRunDisabled bool,
) *Validation {
	return &Validation{
		kubeClient:             kubeClient,
		cloudProvider:          cloudProvider,
		ec2api:                 ec2api,
		amiResolver:            amiResolver,
		instanceTypeProvider:   instanceTypeProvider,
		launchTemplateProvider: launchTemplateProvider,
		cache:                  cache,
		dryRunDisabled:         dryRunDisabled,
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
	// If CIDR has not been resolved, we know validation will fail regardless of the other values.
	readyCondition := nodeClass.StatusConditions().Get(status.ConditionReady)
	if readyCondition.Reason == "NodeClassNotReady" && readyCondition.Message == "Failed to detect the cluster CIDR" {
		nodeClass.StatusConditions().SetFalse(
			v1.ConditionTypeValidationSucceeded,
			ConditionReasonDependenciesNotReady,
			"Awaiting CIDR resolution",
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

	if v.dryRunDisabled {
		nodeClass.StatusConditions().SetTrueWithReason(
			v1.ConditionTypeValidationSucceeded,
			ConditionReasonDryRunDisabled,
			"Dry run is disabled",
		)
		v.cache.SetDefault(v.cacheKey(nodeClass, tags), "")
		return reconcile.Result{}, nil
	}
	validationCtx := &validationContext{}

	for _, isValid := range []validatorFunc{
		v.validateCreateLaunchTemplateAuthorization,
		v.validateCreateFleetAuthorization,
		v.validateRunInstancesAuthorization,
	} {
		if failureReason, requeue, err := isValid(ctx, nodeClass, nodeClaim, tags, validationCtx); err != nil {
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

type validatorFunc func(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim, map[string]string, *validationContext) (string, bool, error)

func (v *Validation) validateCreateLaunchTemplateAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	tags map[string]string,
	validationCtx *validationContext,
) (reason string, requeue bool, err error) {
	opts, err := v.getLaunchTemplateOptions(ctx, nodeClaim, nodeClass, tags)
	if err != nil {
		return "", false, fmt.Errorf("generating options, %w", err)
	}
	// this case should never occur
	if len(opts.InstanceTypes) == 0 {
		return "", false, fmt.Errorf("no instance types available")
	}
	// we only want to create 1 launch template so we only pass 1 instance type to EnsureAll
	instanceTypes := opts.InstanceTypes[:1]
	launchTemplates, err := v.launchTemplateProvider.EnsureAll(ctx, nodeClass, nodeClaim, instanceTypes, karpv1.CapacityTypeOnDemand, tags)
	if err != nil {
		if awserrors.IsRateLimitedError(err) {
			return "", true, nil
		}
		return ConditionReasonCreateLaunchTemplateAuthFailed, false, nil
	}
	// this case should never occur
	if len(launchTemplates) == 0 {
		return "", false, fmt.Errorf("no launch templates created")
	}
	// update validation context
	runInstancesInput := &ec2.RunInstancesInput{}
	raw, err := json.Marshal(launchtemplate.NewCreateLaunchTemplateInputBuilder(opts, corev1.IPv4Protocol, "").Build(ctx).LaunchTemplateData)
	if err != nil {
		return "", false, fmt.Errorf("converting launch template input to run instances input, %w", err)
	}
	if err = json.Unmarshal(raw, runInstancesInput); err != nil {
		return "", false, fmt.Errorf("converting launch template input to run instances input, %w", err)
	}
	validationCtx.instanceTypes = instanceTypes
	validationCtx.launchTemplate = launchTemplates[0]
	validationCtx.runInstancesInput = runInstancesInput
	return "", false, nil
}

func (v *Validation) validateCreateFleetAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	_ *karpv1.NodeClaim,
	tags map[string]string,
	validationCtx *validationContext,
) (reason string, requeue bool, err error) {
	fleetLaunchTemplateConfig := getFleetLaunchTemplateConfig(nodeClass, validationCtx.instanceTypes, validationCtx.launchTemplate)
	createFleetInput := instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, tags, fleetLaunchTemplateConfig).Build()
	createFleetInput.DryRun = lo.ToPtr(true)
	// Adding NopRetryer to avoid aggressive retry when rate limited
	if _, err := v.ec2api.CreateFleet(ctx, createFleetInput, func(o *ec2.Options) {
		o.Retryer = aws.NopRetryer{}
	}); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IsRateLimitedError(err) {
			return "", true, nil
		}
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return "", false, fmt.Errorf("validating ec2:CreateFleet authorization, %w", err)
		}
		return ConditionReasonCreateFleetAuthFailed, false, nil
	}
	return "", false, nil
}

func (v *Validation) validateRunInstancesAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	_ *karpv1.NodeClaim,
	tags map[string]string,
	validationCtx *validationContext,
) (reason string, requeue bool, err error) {
	runInstancesInput := validationCtx.runInstancesInput
	// Ensure we set specific values for things that are typically overridden in the CreateFleet call
	runInstancesInput.DryRun = lo.ToPtr(true)
	runInstancesInput.MaxCount = lo.ToPtr[int32](1)
	runInstancesInput.MinCount = lo.ToPtr[int32](1)
	runInstancesInput.NetworkInterfaces[0].SubnetId = lo.ToPtr(nodeClass.Status.Subnets[0].ID)
	runInstancesInput.InstanceType = ec2types.InstanceType(validationCtx.instanceTypes[0].Name)
	runInstancesInput.TagSpecifications = append(runInstancesInput.TagSpecifications,
		ec2types.TagSpecification{
			ResourceType: ec2types.ResourceTypeInstance,
			Tags:         runInstancesInput.TagSpecifications[0].Tags,
		},
		ec2types.TagSpecification{
			ResourceType: ec2types.ResourceTypeVolume,
			Tags:         runInstancesInput.TagSpecifications[0].Tags,
		},
	)
	// Adding NopRetryer to avoid aggressive retry when rate limited
	if _, err = v.ec2api.RunInstances(ctx, runInstancesInput, func(o *ec2.Options) {
		o.Retryer = aws.NopRetryer{}
	}); awserrors.IgnoreDryRunError(err) != nil {
		// If we get InstanceProfile NotFound, but we have a resolved instance profile in the status,
		// this means there is most likely an eventual consistency issue and we just need to requeue
		if awserrors.IsInstanceProfileNotFound(err) || awserrors.IsRateLimitedError(err) {
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

func getFleetLaunchTemplateConfig(
	nodeClass *v1.EC2NodeClass,
	instanceTypes []*cloudprovider.InstanceType,
	launchTemplate *launchtemplate.LaunchTemplate,
) []ec2types.FleetLaunchTemplateConfigRequest {
	var overrides []ec2types.FleetLaunchTemplateOverridesRequest
	for _, instanceType := range instanceTypes {
		for _, subnet := range nodeClass.Status.Subnets {
			overrides = append(overrides,
				ec2types.FleetLaunchTemplateOverridesRequest{
					InstanceType: ec2types.InstanceType(instanceType.Name),
					SubnetId:     lo.ToPtr(subnet.ID),
				},
			)
		}
	}
	return []ec2types.FleetLaunchTemplateConfigRequest{
		{
			LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: lo.ToPtr(launchTemplate.Name),
				Version:            lo.ToPtr("$Latest"),
			},
			Overrides: overrides,
		},
	}
}

func (v *Validation) getLaunchTemplateOptions(
	ctx context.Context,
	nodeClaim *karpv1.NodeClaim,
	nodeClass *v1.EC2NodeClass,
	tags map[string]string,
) (*amifamily.LaunchTemplate, error) {
	amiOptions, err := v.launchTemplateProvider.CreateAMIOptions(
		ctx,
		nodeClass,
		lo.Assign(
			nodeClaim.Labels,
			scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...).Labels(), // Inject single-value requirements into userData
			map[string]string{karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand},
		),
		tags,
	)
	if err != nil {
		return nil, err
	}

	// Select an instance type to use for validation. If NodePools exist for this NodeClass, we'll use an instance type
	// selected by one of those NodePools. We should also prioritize an InstanceType which will launch with a non-GPU
	// (VariantStandard) AMI, since GPU AMIs may have a larger snapshot size than that supported by the NodeClass'
	// blockDeviceMappings.
	// Historical Issue: https://github.com/aws/karpenter-provider-aws/issues/7928
	instanceTypes, err := v.getInstanceTypesForNodeClass(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	amiMap := amifamily.MapToInstanceTypes(instanceTypes, nodeClass.Status.AMIs)
	var selectedInstanceTypes []*cloudprovider.InstanceType
	for _, ami := range nodeClass.Status.AMIs {
		if len(amiMap[ami.ID]) == 0 {
			continue
		}
		amiRequirements := scheduling.NewNodeSelectorRequirements(ami.Requirements...)
		if amiRequirements.IsCompatible(amifamily.VariantStandard.Requirements()) {
			selectedInstanceTypes = append(selectedInstanceTypes, amiMap[ami.ID]...)
		}
	}
	// If we fail to find an instance type compatible with a standard AMI, fallback
	if len(selectedInstanceTypes) == 0 && len(amiMap) != 0 {
		selectedInstanceTypes = lo.Flatten(lo.Values(amiMap))
	}

	// If there weren't any matching instance types, we should fallback to some defaults. There's an instance type included
	// for both x86_64 and arm64 architectures, ensuring that there will be a matching AMI. We also fallback to the default
	// instance types if the AMI family is Windows. Karpenter currently incorrectly marks certain instance types as Windows
	// compatible, and dynamic instance type resolution may choose those instance types for the dry-run, even if they
	// wouldn't be chosen due to cost in practice. This ensures the behavior matches that on Karpenter v1.3, preventing a
	// potential regression for Windows users.
	// Tracking issue: https://github.com/aws/karpenter-provider-aws/issues/7985
	if len(selectedInstanceTypes) == 0 || lo.ContainsBy([]string{
		v1.AMIFamilyWindows2019,
		v1.AMIFamilyWindows2022,
	}, func(family string) bool {
		return family == nodeClass.AMIFamily()
	}) {
		selectedInstanceTypes = []*cloudprovider.InstanceType{
			{
				Name: string(ec2types.InstanceTypeM5Large),
				Requirements: scheduling.NewRequirements(append(
					lo.Values(amifamily.VariantStandard.Requirements()),
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureAmd64),
					scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpExists),
					scheduling.NewRequirement(corev1.LabelWindowsBuild, corev1.NodeSelectorOpExists),
				)...),
			},
			{
				Name: string(ec2types.InstanceTypeM6gLarge),
				Requirements: scheduling.NewRequirements(append(
					lo.Values(amifamily.VariantStandard.Requirements()),
					scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, karpv1.ArchitectureArm64),
					scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpExists),
					scheduling.NewRequirement(corev1.LabelWindowsBuild, corev1.NodeSelectorOpExists),
				)...),
			},
		}
	}
	opts, err := v.amiResolver.Resolve(nodeClass, nodeClaim, selectedInstanceTypes, karpv1.CapacityTypeOnDemand, amiOptions)
	if err != nil {
		return nil, err
	}
	return opts[0], nil
}

// getInstanceTypesForNodeClass returns the set of instances which could be launched using this NodeClass based on the
// requirements of linked NodePools. If no NodePools exist for the given NodeClass, this function returns two default
// instance types (one x86_64 and one arm64).
func (v *Validation) getInstanceTypesForNodeClass(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]*cloudprovider.InstanceType, error) {
	instanceTypes, err := v.instanceTypeProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("listing instance types for nodeclass, %w", err)
	}
	nodePools, err := nodepoolutils.ListManaged(ctx, v.kubeClient, v.cloudProvider, nodepoolutils.ForNodeClass(nodeClass))
	if err != nil {
		return nil, fmt.Errorf("listing nodepools for nodeclass, %w", err)
	}
	var compatibleInstanceTypes []*cloudprovider.InstanceType
	names := sets.New[string]()
	for _, np := range nodePools {
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(np.Spec.Template.Spec.Requirements...)
		if np.Spec.Template.ObjectMeta.Labels != nil {
			reqs.Add(lo.Values(scheduling.NewLabelRequirements(np.Spec.Template.ObjectMeta.Labels))...)
		}
		for _, it := range instanceTypes {
			if it.Requirements.Intersects(reqs) != nil {
				continue
			}
			if names.Has(it.Name) {
				continue
			}
			names.Insert(it.Name)
			compatibleInstanceTypes = append(compatibleInstanceTypes, it)
		}
	}
	return compatibleInstanceTypes, nil
}
