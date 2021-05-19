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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates the cloud provider's resources
type Factory interface {
	// Capacity returns a provisioner for the provider to create instances
	CapacityFor(provisioner *v1alpha1.Provisioner) Capacity
}

// Capacity provisions a set of nodes that fulfill a set of constraints.
type Capacity interface {
	// Create a set of nodes for each of the given constraints.
	Create(context.Context, []*Packing) ([]*PackedNode, error)
	// Delete nodes in cloudprovider
	Delete(context.Context, []*v1.Node) error
	// GetInstanceTypes returns the instance types supported by the cloud
	// provider limited by the provided constraints and daemons.
	GetInstanceTypes(ctx context.Context) ([]InstanceType, error)
	// GetZones returns the zones supported by the cloud provider.
	GetZones(context.Context) ([]string, error)
	// GetArchitectures returns the architectures supported by the cloud provider.
	GetArchitectures(context.Context) ([]string, error)
	// GetOperatingSystems returns the operating systems supported by the cloud provider.
	GetOperatingSystems(context.Context) ([]string, error)
	// Validate cloud provider specific components of the cluster spec
	Validate(context.Context) error
}

// Packing is a binpacking solution of equivalently schedulable pods to a set of
// viable instance types upon which they fit. All pods in the packing are
// within the specified constraints (e.g., labels, taints).
type Packing struct {
	Pods                []*v1.Pod
	InstanceTypeOptions []InstanceType
	Constraints         *v1alpha1.Constraints
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
	Client    client.Client
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
