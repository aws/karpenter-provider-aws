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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// CloudProvider interface is implemented by cloud providers to support provisioning.
type CloudProvider interface {
	// Create a set of nodes for each of the given constraints. This API uses a
	// callback pattern to enable cloudproviders to batch capacity creation
	// requests. The callback must be called with a theoretical node object that
	// is fulfilled by the cloud providers capacity creation request.
	Create(context.Context, *v1alpha5.Constraints, []InstanceType, int, func(*v1.Node) error) error
	// Delete node in cloudprovider
	Delete(context.Context, *v1.Node) error
	// GetInstanceTypes returns instance types supported by the cloudprovider.
	// Availability of types or zone may vary by provisioner or over time.
	GetInstanceTypes(context.Context, *v1alpha5.Provider) ([]InstanceType, error)
	// Default is a hook for additional defaulting logic at webhook time.
	Default(context.Context, *v1alpha5.Constraints)
	// Validate is a hook for additional validation logic at webhook time.
	Validate(context.Context, *v1alpha5.Constraints) *apis.FieldError
	// Name returns the CloudProvider implementation name.
	Name() string
}

// Options are injected into cloud providers' factories
type Options struct {
	ClientSet *kubernetes.Clientset
}

// InstanceType describes the properties of a potential node (either concrete attributes of an instance of this type
// or supported options in the case of arrays)
type InstanceType interface {
	Name() string
	// Note that though this is an array it is expected that all the Offerings are unique from one another
	Offerings() []Offering
	Architecture() string
	OperatingSystems() sets.String
	CPU() *resource.Quantity
	Memory() *resource.Quantity
	Pods() *resource.Quantity
	NvidiaGPUs() *resource.Quantity
	AMDGPUs() *resource.Quantity
	AWSNeurons() *resource.Quantity
	AWSPodENI() *resource.Quantity
	Overhead() v1.ResourceList
}

// An Offering describes where an InstanceType is available to be used, with the expectation that its properties
// may be tightly coupled (e.g. the availability of an instance type in some zone is scoped to a capacity type)
type Offering struct {
	CapacityType string
	Zone         string
}
