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

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/scheduling"
)

// Context is injected into CloudProvider's factories
type Context struct {
	context.Context

	ClientSet     *kubernetes.Clientset
	KubeClient    client.Client
	EventRecorder record.EventRecorder
	Clock         clock.Clock
	// StartAsync is a channel that is closed when leader election has been won.  This is a signal to start any  async
	// processing that should only occur while the cloud provider is the leader.
	StartAsync <-chan struct{}
	// WebhookOnly is true if the cloud provider is being used for its validation/defaulting only by the webhook. In
	// this case it may not need to perform some initialization and the StartAsync channel will not be closed.
	WebhookOnly bool
}

// CloudProvider interface is implemented by cloud providers to support provisioning.
type CloudProvider interface {
	// Create a node given constraints and instance type options. This API uses a
	// callback pattern to enable cloudproviders to batch capacity creation
	// requests. The callback must be called with a theoretical node object that
	// is fulfilled by the cloud providers capacity creation request.
	Create(context.Context, *NodeRequest) (*v1.Node, error)
	// Delete node in cloudprovider
	Delete(context.Context, *v1.Node) error
	// GetInstanceTypes returns instance types supported by the cloudprovider.
	// Availability of types or zone may vary by provisioner or over time.  Regardless of
	// availability, the GetInstanceTypes method should always return all instance types,
	// even those with no offerings available.
	GetInstanceTypes(context.Context, *v1alpha5.Provisioner) ([]InstanceType, error)
	// Name returns the CloudProvider implementation name.
	Name() string
}

type NodeRequest struct {
	Template            *scheduling.NodeTemplate
	InstanceTypeOptions []InstanceType
}

// InstanceType describes the properties of a potential node (either concrete attributes of an instance of this type
// or supported options in the case of arrays)
type InstanceType interface {
	// Name of the instance type, must correspond to v1.LabelInstanceTypeStable
	Name() string
	// Requirements returns a flexible set of properties that may be selected
	// for scheduling. Must be defined for every well known label, even if empty.
	Requirements() scheduling.Requirements
	// Note that though this is an array it is expected that all the Offerings are unique from one another
	Offerings() []Offering
	// Resources are the full allocatable resource capacities for this instance type
	Resources() v1.ResourceList
	// Overhead is the amount of resource overhead expected to be used by kubelet and any other system daemons outside
	// of Kubernetes.
	Overhead() v1.ResourceList
}

// An Offering describes where an InstanceType is available to be used, with the expectation that its properties
// may be tightly coupled (e.g. the availability of an instance type in some zone is scoped to a capacity type)
type Offering struct {
	CapacityType string
	Zone         string
	Price        float64
	// Available is added so that Offerings() can return all offerings that have ever existed for an instance type
	// so we can get historical pricing data for calculating savings in consolidation
	Available bool
}

// AvailableOfferings filters the offerings on the passed instance type
// and returns the offerings marked as available
func AvailableOfferings(it InstanceType) []Offering {
	return lo.Filter(it.Offerings(), func(o Offering, _ int) bool {
		return o.Available
	})
}

// GetOffering gets the offering from passed instance type that matches the
// passed zone and capacity type
func GetOffering(it InstanceType, ct, zone string) (Offering, bool) {
	return lo.Find(it.Offerings(), func(of Offering) bool {
		return of.CapacityType == ct && of.Zone == zone
	})
}
