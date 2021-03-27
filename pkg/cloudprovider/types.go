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

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates the cloud provider's resources
type Factory interface {
	// Capacity returns a provisioner for the provider to create instances
	CapacityFor(spec *v1alpha1.ProvisionerSpec) Capacity
}

// Capacity provisions a set of nodes that fulfill a set of constraints.
type Capacity interface {
	// Create a set of nodes to fulfill the desired capacity given constraints.
	Create(context.Context, *Constraints) ([]Packing, error)
	// Delete nodes in cloudprovider
	Delete(context.Context, []*v1.Node) error
	// GetInstanceTypes returns the instance types supported by the cloud provider.
	GetInstanceTypes(ctx context.Context) ([]string, error)
	// GetZones returns the zones supported by the cloud provider.
	GetZones(ctx context.Context) ([]string, error)
	// GetArchitectures returns the architectures supported by the cloud provider.
	GetArchitectures(ctx context.Context) ([]string, error)
	// GetOperatingSystems returns the operating systems supported by the cloud provider.
	GetOperatingSystems(ctx context.Context) ([]string, error)
}

// Constraints for an efficient binpacking solution of pods onto nodes, given
// overhead and node constraints.
type Constraints struct {
	v1alpha1.Constraints
	// Pods is a list of equivalently schedulable pods to be binpacked.
	Pods []*v1.Pod
	// Overhead resources per node from daemonsets.
	Overhead v1.ResourceList
}

// Packing is a solution to packing pods onto nodes given constraints.
type Packing struct {
	Node *v1.Node
	Pods []*v1.Pod
}

// Options are injected into cloud providers' factories
type Options struct {
	Client    client.Client
	ClientSet *kubernetes.Clientset
}

type Instance struct {
	// TODO replace w/ generic instance parameters
	ec2.InstanceTypeInfo
	Zones []string
}
