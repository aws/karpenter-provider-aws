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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
)

// CloudProvider interface is implemented by cloud providers to support provisioning.
type CloudProvider interface {
	// Create a set of nodes for each of the given constraints. This API uses a
	// callback pattern to enable cloudproviders to batch capacity creation
	// requests. The callback must be called with a theoretical node object that
	// is fulfilled by the cloud providers capacity creation request. This API
	// is called in parallel and then waits for all channels to return nil or error.
	Create(context.Context, *v1alpha3.Provisioner, *Packing, func(*v1.Node) error) chan error
	// GetInstanceTypes returns the instance types supported by the cloud
	// provider limited by the provided constraints and daemons.
	GetInstanceTypes(context.Context) ([]InstanceType, error)
	// GetZones returns the zones supported by the cloud provider and the specified provisioner
	GetZones(context.Context, *v1alpha3.Provisioner) ([]string, error)
	// ValidateSpec is a hook for additional spec validation logic specific to the cloud provider.
	// Note, implementations should not validate constraints resp. call `ValidateConstraints`
	// from whithin this method as constraints are validated separately.
	ValidateSpec(context.Context, *v1alpha3.ProvisionerSpec) *apis.FieldError
	// ValidateConstraints is a hook for additional constraint validation logic specific to the cloud provider.
	// This method is not only called during Provisioner CRD validation, it is also used at provisioning time
	// to ensure that pods are provisionable for the specified provisioner. For that reasons constraint
	// validation has its own valdiation method and is not conducted as part of `ValidateSpec(...)`.
	ValidateConstraints(context.Context, *v1alpha3.Constraints) *apis.FieldError
	// Terminate node in cloudprovider
	Terminate(context.Context, *v1.Node) error
}

// Packing is a binpacking solution of equivalently schedulable pods to a set of
// viable instance types upon which they fit. All pods in the packing are
// within the specified constraints (e.g., labels, taints).
type Packing struct {
	Pods                []*v1.Pod
	InstanceTypeOptions []InstanceType
	Constraints         *v1alpha3.Constraints
}

// PackedNode is a node object and the pods that should be bound to it. It is
// expected that the pods in a cloudprovider.Packing will be equivalent to the
// pods in a cloudprovider.PackedNode.
type PackedNode struct {
	*v1.Node
	Pods []*v1.Pod
}

// Options are injected into cloud providers' factories
type Options struct {
	ClientSet *kubernetes.Clientset
}

// InstanceType describes the properties of a potential node
type InstanceType interface {
	Name() string
	Zones() []string
	Architectures() []string
	OperatingSystems() []string
	CPU() *resource.Quantity
	Memory() *resource.Quantity
	Pods() *resource.Quantity
	NvidiaGPUs() *resource.Quantity
	AMDGPUs() *resource.Quantity
	AWSNeurons() *resource.Quantity
	Overhead() v1.ResourceList
}
