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
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/resources"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/providers/instancetype"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
)

func init() {
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": v1.LabelTopologyZone})
	coreapis.Settings = append(coreapis.Settings, apis.Settings...)
}

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	instanceTypeProvider  *instancetype.Provider
	instanceProvider      *instance.Provider
	kubeClient            client.Client
	amiProvider           *amifamily.Provider
	securityGroupProvider *securitygroup.Provider
	subnetProvider        *subnet.Provider

}

func New(instanceTypeProvider *instancetype.Provider, instanceProvider *instance.Provider,
	kubeClient client.Client, amiProvider *amifamily.Provider, securityGroupProvider *securitygroup.Provider, subnetProvider *subnet.Provider) *CloudProvider {
	return &CloudProvider{
		instanceTypeProvider:  instanceTypeProvider,
		instanceProvider:      instanceProvider,
		kubeClient:            kubeClient,
		amiProvider:           amiProvider,
		securityGroupProvider: securityGroupProvider,
		subnetProvider: subnetProvider,
	}
}

// Create a machine given the constraints.
func (c *CloudProvider) Create(ctx context.Context, machine *v1alpha5.Machine) (*v1alpha5.Machine, error) {
	nodeTemplate, err := c.resolveNodeTemplate(ctx, []byte(machine.
		Annotations[v1alpha5.ProviderCompatabilityAnnotationKey]), machine.
		Spec.MachineTemplateRef)
	if err != nil {
		return nil, fmt.Errorf("resolving node template, %w", err)
	}
	instanceTypes, err := c.resolveInstanceTypes(ctx, machine, nodeTemplate)
	if err != nil {
		return nil, fmt.Errorf("resolving instance types, %w", err)
	}
	if len(instanceTypes) == 0 {
		return nil, cloudprovider.NewInsufficientCapacityError(fmt.Errorf("all requested instance types were unavailable during launch"))
	}
	instance, err := c.instanceProvider.Create(ctx, nodeTemplate, machine, instanceTypes)
	if err != nil {
		return nil, fmt.Errorf("creating instance, %w", err)
	}
	instanceType, _ := lo.Find(instanceTypes, func(i *cloudprovider.InstanceType) bool {
		return i.Name == instance.Type
	})
	return c.instanceToMachine(instance, instanceType), nil
}

// Link adds a tag to the cloudprovider machine to tell the cloudprovider that it's now owned by a Machine
func (c *CloudProvider) Link(ctx context.Context, machine *v1alpha5.Machine) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("machine", machine.Name))
	id, err := utils.ParseInstanceID(machine.Status.ProviderID)
	if err != nil {
		return fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("id", id))
	return c.instanceProvider.Link(ctx, id, machine.Labels[v1alpha5.ProvisionerNameLabelKey])
}

func (c *CloudProvider) List(ctx context.Context) ([]*v1alpha5.Machine, error) {
	instances, err := c.instanceProvider.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instances, %w", err)
	}
	var machines []*v1alpha5.Machine
	for _, instance := range instances {
		instanceType, err := c.resolveInstanceTypeFromInstance(ctx, instance)
		if err != nil {
			return nil, fmt.Errorf("resolving instance type, %w", err)
		}
		machines = append(machines, c.instanceToMachine(instance, instanceType))
	}
	return machines, nil
}

func (c *CloudProvider) Get(ctx context.Context, providerID string) (*v1alpha5.Machine, error) {
	id, err := utils.ParseInstanceID(providerID)
	if err != nil {
		return nil, fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("id", id))
	instance, err := c.instanceProvider.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting instance, %w", err)
	}
	instanceType, err := c.resolveInstanceTypeFromInstance(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("resolving instance type, %w", err)
	}
	return c.instanceToMachine(instance, instanceType), nil
}

func (c *CloudProvider) LivenessProbe(req *http.Request) error {
	if err := c.instanceTypeProvider.LivenessProbe(req); err != nil {
		return err
	}
	return nil
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]*cloudprovider.InstanceType, error) {
	if provisioner == nil {
		return c.instanceTypeProvider.List(ctx, &v1alpha5.KubeletConfiguration{}, &v1alpha1.AWSNodeTemplate{})
	}
	var rawProvider []byte
	if provisioner.Spec.Provider != nil {
		rawProvider = provisioner.Spec.Provider.Raw
	}
	nodeTemplate, err := c.resolveNodeTemplate(ctx, rawProvider, provisioner.Spec.ProviderRef)
	if err != nil {
		return nil, err
	}
	// TODO, break this coupling
	instanceTypes, err := c.instanceTypeProvider.List(ctx, provisioner.Spec.KubeletConfiguration, nodeTemplate)
	if err != nil {
		return nil, err
	}
	return instanceTypes, nil
}

func (c *CloudProvider) Delete(ctx context.Context, machine *v1alpha5.Machine) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("machine", machine.Name))

	providerID := lo.Ternary(machine.Status.ProviderID != "", machine.Status.ProviderID, machine.Annotations[v1alpha5.MachineLinkedAnnotationKey])
	id, err := utils.ParseInstanceID(providerID)
	if err != nil {
		return fmt.Errorf("getting instance ID, %w", err)
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("id", id))
	return c.instanceProvider.Delete(ctx, id)
}

func (c *CloudProvider) IsMachineDrifted(ctx context.Context, machine *v1alpha5.Machine) (bool, error) {
	// Not needed when GetInstanceTypes removes provisioner dependency
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: machine.Labels[v1alpha5.ProvisionerNameLabelKey]}, provisioner); err != nil {
		return false, client.IgnoreNotFound(fmt.Errorf("getting provisioner, %w", err))
	}
	if provisioner.Spec.ProviderRef == nil {
		return false, nil
	}
	nodeTemplate, err := c.resolveNodeTemplate(ctx, nil, provisioner.Spec.ProviderRef)
	if err != nil {
		return false, client.IgnoreNotFound(fmt.Errorf("resolving node template, %w", err))
	}
	drifted, err := c.isNodeTemplateDrifted(ctx, machine, provisioner, nodeTemplate)
	if err != nil {
		return false, err
	}
	return drifted, nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}

func (c *CloudProvider) resolveNodeTemplate(ctx context.Context, raw []byte, objRef *v1alpha5.MachineTemplateRef) (*v1alpha1.AWSNodeTemplate, error) {
	nodeTemplate := &v1alpha1.AWSNodeTemplate{}
	if objRef != nil {
		if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: objRef.Name}, nodeTemplate); err != nil {
			return nil, fmt.Errorf("getting providerRef, %w", err)
		}
		return nodeTemplate, nil
	}
	aws, err := v1alpha1.DeserializeProvider(raw)
	if err != nil {
		return nil, err
	}
	nodeTemplate.Spec.AWS = lo.FromPtr(aws)
	return nodeTemplate, nil
}

func (c *CloudProvider) resolveInstanceTypes(ctx context.Context, machine *v1alpha5.Machine, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]*cloudprovider.InstanceType, error) {
	instanceTypes, err := c.instanceTypeProvider.List(ctx, machine.Spec.Kubelet, nodeTemplate)
	if err != nil {
		return nil, fmt.Errorf("getting instance types, %w", err)
	}
	reqs := scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...)
	return lo.Filter(instanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
		return reqs.Compatible(i.Requirements) == nil &&
			len(i.Offerings.Requirements(reqs).Available()) > 0 &&
			resources.Fits(machine.Spec.Resources.Requests, i.Allocatable())
	}), nil
}

func (c *CloudProvider) resolveInstanceTypeFromInstance(ctx context.Context, instance *instance.Instance) (*cloudprovider.InstanceType, error) {
	provisioner, err := c.resolveProvisionerFromInstance(ctx, instance)
	if err != nil {
		// If we can't resolve the provisioner, we fall back to not getting instance type info
		return nil, client.IgnoreNotFound(fmt.Errorf("resolving provisioner, %w", err))
	}
	instanceTypes, err := c.GetInstanceTypes(ctx, provisioner)
	if err != nil {
		// If we can't resolve the provisioner, we fall back to not getting instance type info
		return nil, client.IgnoreNotFound(fmt.Errorf("resolving node template, %w", err))
	}
	instanceType, _ := lo.Find(instanceTypes, func(i *cloudprovider.InstanceType) bool {
		return i.Name == instance.Type
	})
	return instanceType, nil
}

func (c *CloudProvider) resolveProvisionerFromInstance(ctx context.Context, instance *instance.Instance) (*v1alpha5.Provisioner, error) {
	provisioner := &v1alpha5.Provisioner{}
	provisionerName, ok := instance.Tags[v1alpha5.ProvisionerNameLabelKey]
	if !ok {
		return nil, errors.NewNotFound(schema.GroupResource{Group: v1alpha5.Group, Resource: "Provisioner"}, "")
	}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return nil, err
	}
	return provisioner, nil
}

func (c *CloudProvider) instanceToMachine(i *instance.Instance, instanceType *cloudprovider.InstanceType) *v1alpha5.Machine {
	machine := &v1alpha5.Machine{}
	labels := map[string]string{}
	annotations := map[string]string{}

	if instanceType != nil {
		for key, req := range instanceType.Requirements {
			if req.Len() == 1 {
				labels[key] = req.Values()[0]
			}
		}
		machine.Status.Capacity = functional.FilterMap(instanceType.Capacity, func(_ v1.ResourceName, v resource.Quantity) bool { return !resources.IsZero(v) })
		machine.Status.Allocatable = functional.FilterMap(instanceType.Allocatable(), func(_ v1.ResourceName, v resource.Quantity) bool { return !resources.IsZero(v) })
	}
	labels[v1.LabelTopologyZone] = i.Zone
	labels[v1alpha5.LabelCapacityType] = i.CapacityType
	if v, ok := i.Tags[v1alpha5.ProvisionerNameLabelKey]; ok {
		labels[v1alpha5.ProvisionerNameLabelKey] = v
	}
	if v, ok := i.Tags[v1alpha5.MachineManagedByAnnotationKey]; ok {
		annotations[v1alpha5.MachineManagedByAnnotationKey] = v
	}
	machine.Labels = labels
	machine.Annotations = annotations
	machine.CreationTimestamp = metav1.Time{Time: i.LaunchTime}
	// Set the deletionTimestamp to be the current time if the instance is currently terminating
	if i.State == ec2.InstanceStateNameShuttingDown || i.State == ec2.InstanceStateNameTerminated {
		machine.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	machine.Status.ProviderID = fmt.Sprintf("aws:///%s/%s", i.Zone, i.ID)
	return machine
}
