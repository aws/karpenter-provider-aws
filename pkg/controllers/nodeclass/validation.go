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
	"strings"
	"time"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	requeueAfterTime                              = 10 * time.Minute
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

// validationCacheEntry stores a failed validation result with both the condition reason and the
// enriched message derived from the AWS error, so cached results can reproduce the same condition.
type validationCacheEntry struct {
	reason  string
	message string
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
	// A NodeClass that uses AL2023 requires the cluster CIDR for launching nodes.
	// To allow Karpenter to be used for Non-EKS clusters, resolving the Cluster CIDR
	// will not be done at startup but instead in a reconcile loop.
	if nodeClass.AMIFamily() == v1.AMIFamilyAL2023 {
		if err := v.launchTemplateProvider.ResolveClusterCIDR(ctx); err != nil {
			if awserrors.IsServerError(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			nodeClass.StatusConditions().SetFalse(
				v1.ConditionTypeValidationSucceeded,
				"ClusterCIDRResolutionFailed",
				"Failed to detect the cluster CIDR",
			)
			return reconcile.Result{}, fmt.Errorf("failed to detect the cluster CIDR, %w", err)
		}
	}

	if _, ok := lo.Find(v.requiredConditions(), func(cond string) bool {
		return nodeClass.StatusConditions().Get(cond).IsFalse()
	}); ok {
		// If any of the required status conditions are false, we know validation will fail regardless of the other values.
		nodeClass.StatusConditions().SetFalse(
			v1.ConditionTypeValidationSucceeded,
			ConditionReasonDependenciesNotReady,
			"Awaiting AMI, Instance Profile, Security Group, and Subnet resolution",
		)
		return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
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
		return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
	}

	nodeClaim := &karpv1.NodeClaim{
		Spec: karpv1.NodeClaimSpec{
			NodeClassRef: &karpv1.NodeClassReference{
				Name: nodeClass.Name,
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
		entry := val.(validationCacheEntry)
		if entry.reason == "" {
			nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		} else {
			nodeClass.StatusConditions().SetFalse(
				v1.ConditionTypeValidationSucceeded,
				entry.reason,
				entry.message,
			)
		}
		return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
	}

	if v.dryRunDisabled {
		nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		v.cache.SetDefault(v.cacheKey(nodeClass, tags), validationCacheEntry{})
		return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
	}

	launchTemplate, result, err := v.validateCreateLaunchTemplateAuthorization(ctx, nodeClass, nodeClaim, tags)
	if err != nil || !lo.IsEmpty(result) {
		return result, err
	}

	result, err = v.validateCreateFleetAuthorization(ctx, nodeClass, tags, launchTemplate)
	if err != nil || !lo.IsEmpty(result) {
		return result, err
	}

	result, err = v.validateRunInstancesAuthorization(ctx, nodeClass, tags, launchTemplate)
	if err != nil || !lo.IsEmpty(result) {
		return result, err
	}

	v.cache.SetDefault(v.cacheKey(nodeClass, tags), validationCacheEntry{})
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
	return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
}

func (v *Validation) updateCacheOnFailure(nodeClass *v1.EC2NodeClass, tags map[string]string, failureReason string, reasonMessage string) {
	message := fmt.Sprintf("%s: %s", ValidationConditionMessages[failureReason], reasonMessage)
	v.cache.SetDefault(v.cacheKey(nodeClass, tags), validationCacheEntry{
		reason:  failureReason,
		message: message,
	})
	nodeClass.StatusConditions().SetFalse(
		v1.ConditionTypeValidationSucceeded,
		failureReason,
		message,
	)
}

func (v *Validation) validateCreateLaunchTemplateAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	tags map[string]string,
) (launchTemplate *launchtemplate.LaunchTemplate, result reconcile.Result, err error) {
	nodePools, err := nodepoolutils.ListManaged(ctx, v.kubeClient, v.cloudProvider, nodepoolutils.ForNodeClass(nodeClass))
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("listing nodepools for nodeclass, %w", err)
	}
	instanceTypes, err := v.getPrioritizedInstanceTypes(ctx, nodeClass, nodePools)
	if err != nil {
		return nil, reconcile.Result{}, fmt.Errorf("generating options, %w", err)
	}
	// pass 1 instance type in EnsureAll to only create 1 launch template
	tenancyType := getTenancyType(nodePools)

	launchTemplates, err := v.launchTemplateProvider.EnsureAll(ctx, nodeClass, nodeClaim, instanceTypes[:1], karpv1.CapacityTypeOnDemand, tags, string(tenancyType))
	if err != nil {
		if awserrors.IsRateLimitedError(err) || awserrors.IsServerError(err) {
			return nil, reconcile.Result{Requeue: true}, nil
		}
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// We should only ever receive UnauthorizedOperation so if we receive any other error it would be an unexpected state
			return nil, reconcile.Result{}, fmt.Errorf("validating ec2:CreateLaunchTemplate authorization, %w", err)
		}
		log.FromContext(ctx).Error(err, "unauthorized to call ec2:CreateLaunchTemplate")
		_, reasonMessage := awserrors.ToReasonMessage(err)
		v.updateCacheOnFailure(nodeClass, tags, ConditionReasonCreateLaunchTemplateAuthFailed, reasonMessage)
		return nil, reconcile.Result{RequeueAfter: requeueAfterTime}, nil
	}
	// this case should never occur as we ensure instance types are compatible with AMI
	if len(launchTemplates) == 0 {
		return nil, reconcile.Result{}, fmt.Errorf("no compatible launch templates created")
	}
	return launchTemplates[0], reconcile.Result{}, nil
}

func (v *Validation) validateCreateFleetAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	tags map[string]string,
	launchTemplate *launchtemplate.LaunchTemplate,
) (result reconcile.Result, err error) {
	fleetLaunchTemplateConfig := getFleetLaunchTemplateConfig(nodeClass, launchTemplate)
	createFleetInput := instance.NewCreateFleetInputBuilder(karpv1.CapacityTypeOnDemand, tags, fleetLaunchTemplateConfig).Build()
	createFleetInput.DryRun = lo.ToPtr(true)
	// Adding NopRetryer to avoid aggressive retry when rate limited
	if _, err := v.ec2api.CreateFleet(ctx, createFleetInput, func(o *ec2.Options) {
		o.Retryer = aws.NopRetryer{}
	}); awserrors.IgnoreDryRunError(err) != nil {
		if awserrors.IsRateLimitedError(err) || awserrors.IsServerError(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		if awserrors.IgnoreUnauthorizedOperationError(err) != nil {
			// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
			// it would be an unexpected state
			return reconcile.Result{}, fmt.Errorf("validating ec2:CreateFleet authorization, %w", err)
		}
		log.FromContext(ctx).Error(err, "unauthorized to call ec2:CreateFleet")
		_, reasonMessage := awserrors.ToReasonMessage(err)
		v.updateCacheOnFailure(nodeClass, tags, ConditionReasonCreateFleetAuthFailed, reasonMessage)
		return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
	}
	return reconcile.Result{}, nil
}

func (v *Validation) validateRunInstancesAuthorization(
	ctx context.Context,
	nodeClass *v1.EC2NodeClass,
	tags map[string]string,
	launchTemplate *launchtemplate.LaunchTemplate,
) (result reconcile.Result, err error) {
	// We use the first subnet's error to determine the outcome. Mixed-error scenarios across subnets
	// are unlikely in practice since authorization policies are not subnet-specific, and transient
	// failures on individual subnets are already handled by the early-exit-on-success pattern.
	var firstSubnetErr error
	for i, subnet := range nodeClass.Status.Subnets {
		runInstancesInput := getRunInstancesInput(tags, launchTemplate, nodeClass.NetworkInterfaces(), &subnet)
		if _, err = v.ec2api.RunInstances(ctx, runInstancesInput, func(o *ec2.Options) {
			// Adding NopRetryer to avoid aggressive retry when rate limited
			o.Retryer = aws.NopRetryer{}
		}); awserrors.IgnoreDryRunError(err) != nil {
			if i == 0 {
				firstSubnetErr = err
			}
		} else {
			// if any of them succeed, we can exit early
			return reconcile.Result{}, nil
		}
	}

	// If we get InstanceProfile NotFound, but we have a resolved instance profile in the status,
	// this means there is most likely an eventual consistency issue and we just need to requeue
	if awserrors.IsInstanceProfileNotFound(firstSubnetErr) || awserrors.IsRateLimitedError(firstSubnetErr) || awserrors.IsServerError(firstSubnetErr) {
		return reconcile.Result{Requeue: true}, nil
	}
	if awserrors.IgnoreUnauthorizedOperationError(firstSubnetErr) != nil {
		// Dry run should only ever return UnauthorizedOperation or DryRunOperation so if we receive any other error
		// it would be an unexpected state
		return reconcile.Result{}, fmt.Errorf("validating ec2:RunInstances authorization, %w", firstSubnetErr)
	}
	log.FromContext(ctx).Error(firstSubnetErr, "unauthorized to call ec2:RunInstances")
	_, reasonMessage := awserrors.ToReasonMessage(firstSubnetErr)
	v.updateCacheOnFailure(nodeClass, tags, ConditionReasonRunInstancesAuthFailed, reasonMessage)
	return reconcile.Result{RequeueAfter: requeueAfterTime}, nil
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
	hash := lo.Must(hashstructure.Hash([]any{
		nodeClass.Status.Subnets,
		nodeClass.Status.SecurityGroups,
		nodeClass.Status.AMIs,
		nodeClass.Status.InstanceProfile,
		nodeClass.Spec,
		nodeClass.Annotations,
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

func getRunInstancesInput(
	tags map[string]string,
	launchTemplate *launchtemplate.LaunchTemplate,
	networkInterfaces []*v1.NetworkInterface,
	subnet *v1.Subnet,
) *ec2.RunInstancesInput {
	return &ec2.RunInstancesInput{
		DryRun:   lo.ToPtr(true),
		MaxCount: lo.ToPtr[int32](1),
		MinCount: lo.ToPtr[int32](1),
		LaunchTemplate: &ec2types.LaunchTemplateSpecification{
			LaunchTemplateName: lo.ToPtr(launchTemplate.Name),
			Version:            lo.ToPtr("$Latest"),
		},
		InstanceType:      ec2types.InstanceType(launchTemplate.InstanceTypes[0].Name),
		NetworkInterfaces: getNetworkInterfacesInput(networkInterfaces, subnet),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags:         utils.EC2MergeTags(tags),
			},
			{
				ResourceType: ec2types.ResourceTypeVolume,
				Tags:         utils.EC2MergeTags(tags),
			},
			{
				ResourceType: ec2types.ResourceTypeNetworkInterface,
				Tags:         utils.EC2MergeTags(tags),
			},
		},
	}
}

func getFleetLaunchTemplateConfig(
	nodeClass *v1.EC2NodeClass,
	launchTemplate *launchtemplate.LaunchTemplate,
) []ec2types.FleetLaunchTemplateConfigRequest {
	var overrides []ec2types.FleetLaunchTemplateOverridesRequest
	for _, instanceType := range launchTemplate.InstanceTypes {
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

// getPrioritizedInstanceTypes returns the set of instances which could be launched using this NodeClass based on the
// requirements of linked NodePools. If no NodePools exist for the given NodeClass, this function returns two default
// instance types (one x86_64 and one arm64). If the 2 default instance types are not compatible with the NodeClass,
// this function we'll use an instance type that could be selected with an open NodePool.
func (v *Validation) getPrioritizedInstanceTypes(ctx context.Context, nodeClass *v1.EC2NodeClass, nodePools []*karpv1.NodePool) ([]*cloudprovider.InstanceType, error) {
	// We should prioritize an InstanceType which will launch with a non-GPU (VariantStandard) AMI, since GPU
	// AMIs may have a larger snapshot size than that supported by the NodeClass' blockDeviceMappings.
	// Historical Issue: https://github.com/aws/karpenter-provider-aws/issues/7928
	instanceTypes, err := v.instanceTypeProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, fmt.Errorf("listing instance types for nodeclass, %w", err)
	}
	instanceTypes = v.instanceTypeProvider.FilterForNodeClass(ctx, instanceTypes, nodeClass)
	compatibleInstanceTypes := getNodePoolCompatibleInstanceTypes(instanceTypes, nodePools)

	// If there weren't any matching instance types, we should fallback to some defaults. There's an instance type included
	// for both x86_64 and arm64 architectures, ensuring that there will be a matching AMI. We also fallback to the default
	// instance types if the AMI family is Windows. Karpenter currently incorrectly marks certain instance types as Windows
	// compatible, and dynamic instance type resolution may choose those instance types for the dry-run, even if they
	// wouldn't be chosen due to cost in practice. This ensures the behavior matches that on Karpenter v1.3, preventing a
	// potential regression for Windows users.
	// Tracking issue: https://github.com/aws/karpenter-provider-aws/issues/7985
	if len(compatibleInstanceTypes) == 0 || lo.ContainsBy([]string{
		v1.AMIFamilyWindows2019,
		v1.AMIFamilyWindows2022,
		v1.AMIFamilyWindows2025,
	}, func(family string) bool {
		return family == nodeClass.AMIFamily()
	}) {
		compatibleInstanceTypes = v.getFallbackInstanceTypes(instanceTypes)
	}
	return getAMICompatibleInstanceTypes(compatibleInstanceTypes, nodeClass), nil
}

func (v *Validation) getFallbackInstanceTypes(instanceTypes []*cloudprovider.InstanceType) []*cloudprovider.InstanceType {
	fallbackInstanceTypes := []*cloudprovider.InstanceType{
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
	fallbackInstanceTypes = lo.Filter(fallbackInstanceTypes, func(itFallback *cloudprovider.InstanceType, _ int) bool {
		return lo.ContainsBy(instanceTypes, func(it *cloudprovider.InstanceType) bool {
			return it.Name == itFallback.Name
		})
	})
	return lo.Ternary(len(fallbackInstanceTypes) == 0, instanceTypes, fallbackInstanceTypes)
}

func getTenancyType(nodePools []*karpv1.NodePool) ec2types.Tenancy {
	for _, np := range nodePools {
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(np.Spec.Template.Spec.Requirements...)
		if reqs.Has(v1.LabelInstanceTenancy) && reqs.Get(v1.LabelInstanceTenancy).Has(string(ec2types.TenancyDedicated)) {
			return ec2types.TenancyDedicated
		}
	}
	return ec2types.TenancyDefault
}

func getNodePoolCompatibleInstanceTypes(instanceTypes []*cloudprovider.InstanceType, nodePools []*karpv1.NodePool) []*cloudprovider.InstanceType {
	var compatibleInstanceTypes []*cloudprovider.InstanceType
	names := sets.New[string]()
	for _, np := range nodePools {
		reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(np.Spec.Template.Spec.Requirements...)
		if np.Spec.Template.Labels != nil {
			reqs.Add(lo.Values(scheduling.NewLabelRequirements(np.Spec.Template.Labels))...)
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
	return compatibleInstanceTypes
}

func getAMICompatibleInstanceTypes(instanceTypes []*cloudprovider.InstanceType, nodeClass *v1.EC2NodeClass) []*cloudprovider.InstanceType {
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

	return selectedInstanceTypes
}

func getNetworkInterfacesInput(ncNetworkInterfaces []*v1.NetworkInterface, subnet *v1.Subnet) []ec2types.InstanceNetworkInterfaceSpecification {
	defaultInterface := []ec2types.InstanceNetworkInterfaceSpecification{
		{
			DeviceIndex: lo.ToPtr[int32](0),
			SubnetId:    lo.ToPtr(subnet.ID),
		},
	}
	networkInterfaces := lo.Ternary(ncNetworkInterfaces == nil,
		defaultInterface,
		lo.Map(ncNetworkInterfaces, func(networkInterface *v1.NetworkInterface, _ int) ec2types.InstanceNetworkInterfaceSpecification {
			return ec2types.InstanceNetworkInterfaceSpecification{
				NetworkCardIndex: lo.ToPtr(networkInterface.NetworkCardIndex),
				DeviceIndex:      lo.ToPtr(networkInterface.DeviceIndex),
				InterfaceType:    lo.ToPtr(string(networkInterface.InterfaceType)),
				SubnetId:         lo.ToPtr(subnet.ID),
			}
		}))
	return networkInterfaces
}
