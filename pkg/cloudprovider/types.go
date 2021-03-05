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

	autoscalingv1alpha1 "github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	provisioningv1alpha1 "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates the cloud provider's resources
type Factory interface {
	// NodeGroupFor returns a node group for the provided spec
	NodeGroupFor(sng *autoscalingv1alpha1.ScalableNodeGroupSpec) NodeGroup
	// QueueFor returns a queue for the provided spec
	QueueFor(queue *autoscalingv1alpha1.QueueSpec) Queue
	// Capacity returns a provisioner for the provider to create instances
	CapacityFor(spec *provisioningv1alpha1.ProvisionerSpec) Capacity
}

// Queue abstracts all provider specific behavior for Queues
type Queue interface {
	// Name returns the name of the queue
	Name() string
	// Length returns the length of the queue
	Length() (int64, error)
	// OldestMessageAge returns the age of the oldest message
	OldestMessageAgeSeconds() (int64, error)
}

// NodeGroup abstracts all provider specific behavior for NodeGroups.
// It is meant to be used by controllers.
type NodeGroup interface {
	// SetReplicas sets the desired replicas for the node group
	SetReplicas(count int32) error
	// GetReplicas returns the number of schedulable replicas in the node group
	GetReplicas() (int32, error)
	// Stabilized returns true if a node group is not currently adding or
	// removing replicas. Otherwise, returns false with a message.
	Stabilized() (bool, string, error)
}

// Capacity provisions a set of nodes that fulfill a set of constraints.
type Capacity interface {
	// Create a set of nodes to fulfill the desired capacity given constraints.
	Create(context.Context, *Constraints) ([]Packing, error)
	// Delete nodes in cloudprovider
	Delete(context.Context, []*v1.Node) error
}

// Constraints for an efficient binpacking solution of pods onto nodes, given
// overhead and node constraints.
type Constraints struct {
	provisioningv1alpha1.Constraints
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
