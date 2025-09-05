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
	stderrors "errors"
	"fmt"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeoverlay"

	"sigs.k8s.io/controller-runtime/pkg/log"
	coreapis "sigs.k8s.io/karpenter/pkg/apis"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	karpoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudproviderevents "github.com/aws/karpenter-provider-aws/pkg/cloudprovider/events"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	kubeClient client.Client
	recorder   events.Recorder

	instanceTypeProvider        instancetype.Provider
	instanceProvider            instance.Provider
	amiProvider                 amifamily.Provider
	securityGroupProvider       securitygroup.Provider
	capacityReservationProvider capacityreservation.Provider
	instanceTypeStore           *nodeoverlay.InstanceTypeStore
}

func New(
	instanceTypeProvider instancetype.Provider,
	instanceProvider instance.Provider,
	recorder events.Recorder,
	kubeClient client.Client,
	amiProvider amifamily.Provider,
	securityGroupProvider securitygroup.Provider,
	capacityReservationProvider capacityreservation.Provider,
	store *nodeoverlay.InstanceTypeStore,
) *CloudProvider {
	return &CloudProvider{
		instanceTypeProvider:        instanceTypeProvider,
		instanceProvider:            instanceProvider,
		kubeClient:                  kubeClient,
		amiProvider:                 amiProvider,
		securityGroupProvider:       securityGroupProvider,
		capacityReservationProvider: capacityReservationProvider,
		recorder:                    recorder,
		instanceTypeStore:           store,
	}
}

// Create a NodeClaim given the constraints.
//
//nolint:gocyclo
func (c *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	nodeClass, err := c.resolveNodeClassFromNodeClaim(ctx, nodeClaim)
	if err != nil {
		if errors.IsNotFound(err) {
			// We treat a failure to resolve the NodeClass as an ICE since this means there is no capacity possibilities for this NodeClaim
			c.recorder.Publish(cloudproviderevents.NodeClaimFailedToResolveNodeClass(nodeClaim))
			return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("resolving nodeclass, %w", err))
		}
		// Transient error when resolving the NodeClass
		return nil, fmt.Errorf("resolving nodeclass, %w", err)
	}

	nodeClassReady := nodeClass.StatusConditions().Get(status.ConditionReady)
	if nodeClassReady.IsFalse() {
		return nil, cloudprovider.NewNodeClassNotReadyError(stderrors.New(nodeClassReady.Message))
	}
	if nodeClassReady.IsUnknown() {
		return nil, cloudprovider.NewCreateError(fmt.Errorf("resolving nodeclass readiness, nodeclass is in Ready=Unknown, %s", nodeClassReady.Message), "NodeClassReadinessUnknown", "NodeClass is in Ready=Unknown")
	}
	if nodeClassReady != nil && nodeClassReady.ObservedGeneration != nodeClass.Generation {
		return nil, cloudprovider.NewNodeClassNotReadyError(fmt.Errorf("nodeclass status has not been reconciled against the latest spec"))
	}
	tags, err := utils.GetTags(nodeClass, nodeClaim, options.FromContext(ctx).ClusterName)
	if err != nil {
		return nil, cloudprovider.NewNodeClassNotReadyError(err)
	}
	instanceTypes, err := c.instanceTypeProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, cloudprovider.NewCreateError(fmt.Errorf("resolving instance types, %w", err), "InstanceTypeResolutionFailed", "Error resolving instance types")
	}
	if karpoptions.FromContext(ctx).FeatureGates.NodeOverlay {
		instanceTypes, err = c.instanceTypeStore.ApplyAll(nodeClaim.Labels[v1.NodePoolTagKey], instanceTypes)
		if err != nil {
			return nil, fmt.Errorf("creating instance, %w", err)
		}
	}
	instance, err := c.instanceProvider.Create(ctx, nodeClass, nodeClaim, tags, instanceTypes)
	if err != nil {
		return nil, fmt.Errorf("creating instance, %w", err)
	}
	if instance.CapacityType == karpv1.CapacityTypeReserved {
		c.capacityReservationProvider.MarkLaunched(*instance.CapacityReservationID)
	}
	instanceType, _ := lo.Find(instanceTypes, func(i *cloudprovider.InstanceType) bool {
		return i.Name == string(instance.Type)
	})
	nc := c.instanceToNodeClaim(instance, instanceType, nodeClass)
	nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
		v1.AnnotationEC2NodeClassHash:        nodeClass.Hash(),
		v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
		v1.AnnotationInstanceProfile:         nodeClass.Status.InstanceProfile,
	})
	return nc, nil
}

func (c *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	instances, err := c.instanceProvider.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instances, %w", err)
	}
	var nodeClaims []*karpv1.NodeClaim
	for _, it := range instances {
		instanceType, err := c.resolveInstanceTypeFromInstance(ctx, it)
		if err != nil {
			return nil, fmt.Errorf("resolving instance type, %w", err)
		}
		nc, err := c.resolveNodeClassFromInstance(ctx, it)
		if client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("resolving nodeclass, %w", err)
		}
		nodeClaims = append(nodeClaims, c.instanceToNodeClaim(it, instanceType, nc))
	}
	return nodeClaims, nil
}

func (c *CloudProvider) Get(ctx context.Context, providerID string) (*karpv1.NodeClaim, error) {
	id, err := utils.ParseInstanceID(providerID)
	if err != nil {
		return nil, fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("id", id))
	instance, err := c.instanceProvider.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting instance, %w", err)
	}
	instanceType, err := c.resolveInstanceTypeFromInstance(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("resolving instance type, %w", err)
	}
	nc, err := c.resolveNodeClassFromInstance(ctx, instance)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("resolving nodeclass, %w", err)
	}
	return c.instanceToNodeClaim(instance, instanceType, nc), nil
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, nodePool *karpv1.NodePool) ([]*cloudprovider.InstanceType, error) {
	nodeClass, err := c.resolveNodeClassFromNodePool(ctx, nodePool)
	if err != nil {
		if errors.IsNotFound(err) {
			// If we can't resolve the NodeClass, then it's impossible for us to resolve the instance types
			c.recorder.Publish(cloudproviderevents.NodePoolFailedToResolveNodeClass(nodePool))
			return nil, nil
		}
		return nil, fmt.Errorf("resolving nodeclass, %w", err)
	}
	// TODO, break this coupling
	instanceTypes, err := c.instanceTypeProvider.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	return instanceTypes, nil
}

// getInstanceType returns a specific instance type to avoid re-constructing all InstanceTypes
func (c *CloudProvider) getInstanceType(ctx context.Context, nodePool *karpv1.NodePool, name ec2types.InstanceType) (*cloudprovider.InstanceType, error) {
	nodeClass, err := c.resolveNodeClassFromNodePool(ctx, nodePool)
	if err != nil {
		if errors.IsNotFound(err) {
			// If we can't resolve the NodeClass, then it's impossible for us to resolve the instance types
			c.recorder.Publish(cloudproviderevents.NodePoolFailedToResolveNodeClass(nodePool))
			return nil, nil
		}
		return nil, fmt.Errorf("resolving nodeclass, %w", err)
	}
	it, err := c.instanceTypeProvider.Get(ctx, nodeClass, name)
	if err != nil {
		return nil, fmt.Errorf("resolving instancetype, %w", err)
	}
	if karpoptions.FromContext(ctx).FeatureGates.NodeOverlay {
		it, err = c.instanceTypeStore.Apply(nodePool.Name, it)
		if err != nil {
			return nil, fmt.Errorf("resolving instancetype, %w", err)
		}
	}

	return it, err
}

func (c *CloudProvider) Delete(ctx context.Context, nodeClaim *karpv1.NodeClaim) error {
	id, err := utils.ParseInstanceID(nodeClaim.Status.ProviderID)
	if err != nil {
		return fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("id", id))
	err = c.instanceProvider.Delete(ctx, id)
	if id := nodeClaim.Labels[cloudprovider.ReservationIDLabel]; id != "" && cloudprovider.IsNodeClaimNotFoundError(err) {
		c.capacityReservationProvider.MarkTerminated(id)
	}
	return err
}

func (c *CloudProvider) DisruptionReasons() []karpv1.DisruptionReason {
	return nil
}

func (c *CloudProvider) IsDrifted(ctx context.Context, nodeClaim *karpv1.NodeClaim) (cloudprovider.DriftReason, error) {
	// Not needed when GetInstanceTypes removes nodepool dependency
	nodePoolName, ok := nodeClaim.Labels[karpv1.NodePoolLabelKey]
	if !ok {
		return "", nil
	}
	nodePool := &karpv1.NodePool{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodePoolName}, nodePool); err != nil {
		return "", client.IgnoreNotFound(err)
	}
	if nodePool.Spec.Template.Spec.NodeClassRef == nil {
		return "", nil
	}
	nodeClass, err := c.resolveNodeClassFromNodePool(ctx, nodePool)
	if err != nil {
		if errors.IsNotFound(err) {
			// We can't determine the drift status for the NodeClaim if we can no longer resolve the NodeClass
			c.recorder.Publish(cloudproviderevents.NodePoolFailedToResolveNodeClass(nodePool))
			return "", nil
		}
		return "", fmt.Errorf("resolving nodeclass, %w", err)
	}
	driftReason, err := c.isNodeClassDrifted(ctx, nodeClaim, nodePool, nodeClass)
	if err != nil {
		return "", err
	}
	return driftReason, nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}

func (c *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1.EC2NodeClass{}}
}

func (c *CloudProvider) RepairPolicies() []cloudprovider.RepairPolicy {
	return []cloudprovider.RepairPolicy{
		// Supported Kubelet Node Conditions
		{
			ConditionType:      corev1.NodeReady,
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
		{
			ConditionType:      corev1.NodeReady,
			ConditionStatus:    corev1.ConditionUnknown,
			TolerationDuration: 30 * time.Minute,
		},
		// Support Node Monitoring Agent Conditions
		//
		{
			ConditionType:      "AcceleratedHardwareReady",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 10 * time.Minute,
		},
		{
			ConditionType:      "StorageReady",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
		{
			ConditionType:      "NetworkingReady",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
		{
			ConditionType:      "KernelReady",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
		{
			ConditionType:      "ContainerRuntimeReady",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
	}
}

func (c *CloudProvider) resolveNodeClassFromNodeClaim(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*v1.EC2NodeClass, error) {
	nodeClass := &v1.EC2NodeClass{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodeClaim.Spec.NodeClassRef.Name}, nodeClass); err != nil {
		return nil, err
	}
	// For the purposes of NodeClass CloudProvider resolution, we treat deleting NodeClasses as NotFound
	if !nodeClass.DeletionTimestamp.IsZero() {
		// For the purposes of NodeClass CloudProvider resolution, we treat deleting NodeClasses as NotFound,
		// but we return a different error message to be clearer to users
		return nil, newTerminatingNodeClassError(nodeClass.Name)
	}
	return nodeClass, nil
}

func (c *CloudProvider) resolveNodeClassFromNodePool(ctx context.Context, nodePool *karpv1.NodePool) (*v1.EC2NodeClass, error) {
	nodeClass := &v1.EC2NodeClass{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodePool.Spec.Template.Spec.NodeClassRef.Name}, nodeClass); err != nil {
		return nil, err
	}
	if !nodeClass.DeletionTimestamp.IsZero() {
		// For the purposes of NodeClass CloudProvider resolution, we treat deleting NodeClasses as NotFound,
		// but we return a different error message to be clearer to users
		return nil, newTerminatingNodeClassError(nodeClass.Name)
	}
	return nodeClass, nil
}

func (c *CloudProvider) resolveInstanceTypeFromInstance(ctx context.Context, instance *instance.Instance) (*cloudprovider.InstanceType, error) {
	nodePool, err := c.resolveNodePoolFromInstance(ctx, instance)
	if err != nil {
		// If we can't resolve the NodePool, we fall back to not getting instance type info
		return nil, client.IgnoreNotFound(fmt.Errorf("resolving nodepool, %w", err))
	}
	instanceType, err := c.getInstanceType(ctx, nodePool, instance.Type)
	if err != nil {
		// If we can't resolve the NodePool, we fall back to not getting instance type info
		return nil, client.IgnoreNotFound(fmt.Errorf("resolving instance type, %w", err))
	}
	return instanceType, nil
}

func (c *CloudProvider) resolveNodeClassFromInstance(ctx context.Context, instance *instance.Instance) (*v1.EC2NodeClass, error) {
	name, ok := instance.Tags[v1.NodeClassTagKey]
	if !ok {
		return nil, errors.NewNotFound(schema.GroupResource{Group: apis.Group, Resource: "ec2nodeclasses"}, "")
	}
	nc := &v1.EC2NodeClass{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: name}, nc); err != nil {
		return nil, fmt.Errorf("resolving ec2nodeclass, %w", err)
	}
	if !nc.DeletionTimestamp.IsZero() {
		// For the purposes of NodeClass CloudProvider resolution, we treat deleting NodeClasses as NotFound,
		// but we return a different error message to be clearer to users
		return nil, newTerminatingNodeClassError(nc.Name)
	}
	return nc, nil
}

func (c *CloudProvider) resolveNodePoolFromInstance(ctx context.Context, instance *instance.Instance) (*karpv1.NodePool, error) {
	if nodePoolName, ok := instance.Tags[karpv1.NodePoolLabelKey]; ok {
		nodePool := &karpv1.NodePool{}
		if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodePoolName}, nodePool); err != nil {
			return nil, err
		}
		return nodePool, nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{Group: coreapis.Group, Resource: "nodepools"}, "")
}

//nolint:gocyclo
func (c *CloudProvider) instanceToNodeClaim(i *instance.Instance, instanceType *cloudprovider.InstanceType, nodeClass *v1.EC2NodeClass) *karpv1.NodeClaim {
	nodeClaim := &karpv1.NodeClaim{}
	labels := map[string]string{}
	annotations := map[string]string{}

	if instanceType != nil {
		for key, req := range instanceType.Requirements {
			// We only want to add a label based on the instance type requirements if there is a single value for that
			// requirement. For example, we can't add a label for zone based on this if the requirement is compatible with
			// three. Capacity reservation IDs are a special case since we don't have a way to represent that the label may or
			// may not exist. Since this requirement will be present regardless of the capacity type, we can't insert it here.
			// Otherwise, you may end up with spot and on-demand NodeClaims with a reservation ID label.
			if req.Len() == 1 && !lo.Contains([]string{
				cloudprovider.ReservationIDLabel,
				v1.LabelCapacityReservationType,
			}, req.Key) {
				labels[key] = req.Values()[0]
			}
		}
		resourceFilter := func(n corev1.ResourceName, v resource.Quantity) bool {
			if resources.IsZero(v) {
				return false
			}
			// The nodeclaim should only advertise an EFA resource if it was requested. EFA network interfaces are only
			// added to the launch template if they're requested, otherwise the instance is launched with a normal ENI.
			if n == v1.ResourceEFA {
				return i.EFAEnabled
			}
			return true
		}
		nodeClaim.Status.Capacity = lo.PickBy(instanceType.Capacity, resourceFilter)
		nodeClaim.Status.Allocatable = lo.PickBy(instanceType.Allocatable(), resourceFilter)
	}
	labels[corev1.LabelTopologyZone] = i.Zone
	// Attempt to resolve the zoneID from the instance's EC2NodeClass' status condition.
	// If the EC2NodeClass is nil, we know we're in the List or Get paths, where we don't care about the zone-id value.
	// If we're in the Create path, we've already validated the EC2NodeClass exists. In this case, we resolve the zone-id from the status condition
	// both when creating offerings and when adding the label.
	if nodeClass != nil {
		if subnet, ok := lo.Find(nodeClass.Status.Subnets, func(s v1.Subnet) bool {
			return s.Zone == i.Zone
		}); ok && subnet.ZoneID != "" {
			labels[v1.LabelTopologyZoneID] = subnet.ZoneID
		}
	}
	labels[karpv1.CapacityTypeLabelKey] = i.CapacityType
	if i.CapacityType == karpv1.CapacityTypeReserved {
		labels[cloudprovider.ReservationIDLabel] = *i.CapacityReservationID
		labels[v1.LabelCapacityReservationType] = string(*i.CapacityReservationType)
	}
	if v, ok := i.Tags[karpv1.NodePoolLabelKey]; ok {
		labels[karpv1.NodePoolLabelKey] = v
	}
	nodeClaim.Labels = labels
	nodeClaim.Annotations = annotations
	nodeClaim.CreationTimestamp = metav1.Time{Time: i.LaunchTime}
	// Set the deletionTimestamp to be the current time if the instance is currently terminating
	if i.State == ec2types.InstanceStateNameShuttingDown || i.State == ec2types.InstanceStateNameTerminated {
		nodeClaim.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	nodeClaim.Status.ProviderID = fmt.Sprintf("aws:///%s/%s", i.Zone, i.ID)
	nodeClaim.Status.ImageID = i.ImageID
	return nodeClaim
}

// newTerminatingNodeClassError returns a NotFound error for handling by
func newTerminatingNodeClassError(name string) *errors.StatusError {
	qualifiedResource := schema.GroupResource{Group: apis.Group, Resource: "ec2nodeclasses"}
	err := errors.NewNotFound(qualifiedResource, name)
	err.ErrStatus.Message = fmt.Sprintf("%s %q is terminating, treating as not found", qualifiedResource.String(), name)
	return err
}
