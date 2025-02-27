/*
Copyright The Kubernetes Authors.

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
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

var (
	SpotRequirement     = scheduling.NewRequirements(scheduling.NewRequirement(v1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, v1.CapacityTypeSpot))
	OnDemandRequirement = scheduling.NewRequirements(scheduling.NewRequirement(v1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, v1.CapacityTypeOnDemand))
)

type DriftReason string

type RepairPolicy struct {
	// ConditionType of unhealthy state that is found on the node
	ConditionType corev1.NodeConditionType
	// ConditionStatus condition when a node is unhealthy
	ConditionStatus corev1.ConditionStatus
	// TolerationDuration is the duration the controller will wait
	// before force terminating nodes that are unhealthy.
	TolerationDuration time.Duration
}

// CloudProvider interface is implemented by cloud providers to support provisioning.
type CloudProvider interface {
	// Create launches a NodeClaim with the given resource requests and requirements and returns a hydrated
	// NodeClaim back with resolved NodeClaim labels for the launched NodeClaim
	Create(context.Context, *v1.NodeClaim) (*v1.NodeClaim, error)
	// Delete removes a NodeClaim from the cloudprovider by its provider id. Delete should return
	// NodeClaimNotFoundError if the cloudProvider instance is already terminated and nil if deletion was triggered.
	// Karpenter will keep retrying until Delete returns a NodeClaimNotFound error.
	Delete(context.Context, *v1.NodeClaim) error
	// Get retrieves a NodeClaim from the cloudprovider by its provider id
	Get(context.Context, string) (*v1.NodeClaim, error)
	// List retrieves all NodeClaims from the cloudprovider
	List(context.Context) ([]*v1.NodeClaim, error)
	// GetInstanceTypes returns instance types supported by the cloudprovider.
	// Availability of types or zone may vary by nodepool or over time.  Regardless of
	// availability, the GetInstanceTypes method should always return all instance types,
	// even those with no offerings available.
	GetInstanceTypes(context.Context, *v1.NodePool) ([]*InstanceType, error)
	// IsDrifted returns whether a NodeClaim has drifted from the provisioning requirements
	// it is tied to.
	IsDrifted(context.Context, *v1.NodeClaim) (DriftReason, error)
	// RepairPolicy is for CloudProviders to define a set Unhealthy condition for Karpenter
	// to monitor on the node.
	RepairPolicies() []RepairPolicy
	// Name returns the CloudProvider implementation name.
	Name() string
	// GetSupportedNodeClasses returns CloudProvider NodeClass that implements status.Object
	// NOTE: It returns a list where the first element should be the default NodeClass
	GetSupportedNodeClasses() []status.Object
}

// InstanceType describes the properties of a potential node (either concrete attributes of an instance of this type
// or supported options in the case of arrays)
type InstanceType struct {
	// Name of the instance type, must correspond to corev1.LabelInstanceTypeStable
	Name string
	// Requirements returns a flexible set of properties that may be selected
	// for scheduling. Must be defined for every well known label, even if empty.
	Requirements scheduling.Requirements
	// Note that though this is an array it is expected that all the Offerings are unique from one another
	Offerings Offerings
	// Resources are the full resource capacities for this instance type
	Capacity corev1.ResourceList
	// Overhead is the amount of resource overhead expected to be used by kubelet and any other system daemons outside
	// of Kubernetes.
	Overhead *InstanceTypeOverhead

	once        sync.Once
	allocatable corev1.ResourceList
}

type InstanceTypes []*InstanceType

// precompute is used to ensure we only compute the allocatable resources onces as its called many times
// and the operation is fairly expensive.
func (i *InstanceType) precompute() {
	i.allocatable = resources.Subtract(i.Capacity, i.Overhead.Total())
}

func (i *InstanceType) Allocatable() corev1.ResourceList {
	i.once.Do(i.precompute)
	return i.allocatable
}

func (its InstanceTypes) OrderByPrice(reqs scheduling.Requirements) InstanceTypes {
	// Order instance types so that we get the cheapest instance types of the available offerings
	sort.Slice(its, func(i, j int) bool {
		iPrice := math.MaxFloat64
		jPrice := math.MaxFloat64
		if ofs := its[i].Offerings.Available().Compatible(reqs); len(ofs) > 0 {
			iPrice = ofs.Cheapest().Price
		}
		if ofs := its[j].Offerings.Available().Compatible(reqs); len(ofs) > 0 {
			jPrice = ofs.Cheapest().Price
		}
		if iPrice == jPrice {
			return its[i].Name < its[j].Name
		}
		return iPrice < jPrice
	})
	return its
}

// Compatible returns the list of instanceTypes based on the supported capacityType and zones in the requirements
func (its InstanceTypes) Compatible(requirements scheduling.Requirements) InstanceTypes {
	var filteredInstanceTypes []*InstanceType
	for _, instanceType := range its {
		if instanceType.Offerings.Available().HasCompatible(requirements) {
			filteredInstanceTypes = append(filteredInstanceTypes, instanceType)
		}
	}
	return filteredInstanceTypes
}

// SatisfiesMinValues validates whether the InstanceTypes satisfies the minValues requirements
// It returns the minimum number of needed instance types to satisfy the minValues requirement and an error
// that indicates whether the InstanceTypes satisfy the passed-in requirements
// This minNeededInstanceTypes value is dependent on the ordering of instance types, so relying on this value in a
// deterministic way implies that the instance types are sorted ahead of using this method
// For example:
// Requirements:
//   - key: node.kubernetes.io/instance-type
//     operator: In
//     values: ["c4.large","c4.xlarge","c5.large","c5.xlarge","m4.large","m4.xlarge"]
//     minValues: 3
//   - key: karpenter.kwok.sh/instance-family
//     operator: In
//     values: ["c4","c5","m4"]
//     minValues: 3
//
// InstanceTypes: ["c4.large","c5.xlarge","m4.2xlarge"], it PASSES the requirements
//
//		we get the map as : {
//			node.kubernetes.io/instance-type:  ["c4.large","c5.xlarge","m4.2xlarge"],
//			karpenter.k8s.aws/instance-family: ["c4","c5","m4"]
//		}
//	 so it returns 3 and a nil error to indicate a minimum of 3 instance types were required to fulfill the minValues requirements
//
// And if InstanceTypes: ["c4.large","c4.xlarge","c5.2xlarge"], it FAILS the requirements
//
//		we get the map as : {
//			node.kubernetes.io/instance-type:  ["c4.large","c4.xlarge","c5.2xlarge"],
//			karpenter.k8s.aws/instance-family: ["c4","c5"] // minimum requirement failed for this.
//		}
//	  so it returns 3 and a non-nil error to indicate that the instance types weren't able to fulfill the minValues requirements
func (its InstanceTypes) SatisfiesMinValues(requirements scheduling.Requirements) (minNeededInstanceTypes int, err error) {
	if !requirements.HasMinValues() {
		return 0, nil
	}
	valuesForKey := map[string]sets.Set[string]{}
	// We validate if sorting by price and truncating the number of instance types to minItems breaks the minValue requirement.
	// If minValue requirement fails, we return an error that indicates the first requirement key that couldn't be satisfied.
	var incompatibleKey string
	for i, it := range its {
		for _, req := range requirements {
			if req.MinValues != nil {
				if _, ok := valuesForKey[req.Key]; !ok {
					valuesForKey[req.Key] = sets.New[string]()
				}
				valuesForKey[req.Key] = valuesForKey[req.Key].Insert(it.Requirements.Get(req.Key).Values()...)
			}
		}
		incompatibleKey = func() string {
			for k, v := range valuesForKey {
				// Break if any of the MinValues of requirement is not honored
				if len(v) < lo.FromPtr(requirements.Get(k).MinValues) {
					return k
				}
			}
			return ""
		}()
		if incompatibleKey == "" {
			return i + 1, nil
		}
	}
	if incompatibleKey != "" {
		return len(its), fmt.Errorf("minValues requirement is not met for %q", incompatibleKey)
	}
	return len(its), nil
}

// Truncate truncates the InstanceTypes based on the passed-in requirements
// It returns an error if it isn't possible to truncate the instance types on maxItems without violating minValues
func (its InstanceTypes) Truncate(requirements scheduling.Requirements, maxItems int) (InstanceTypes, error) {
	truncatedInstanceTypes := lo.Slice(its.OrderByPrice(requirements), 0, maxItems)
	// Only check for a validity of NodeClaim if its requirement has minValues in it.
	if requirements.HasMinValues() {
		if _, err := truncatedInstanceTypes.SatisfiesMinValues(requirements); err != nil {
			return its, fmt.Errorf("validating minValues, %w", err)
		}
	}
	return truncatedInstanceTypes, nil
}

type InstanceTypeOverhead struct {
	// KubeReserved returns the default resources allocated to kubernetes system daemons by default
	KubeReserved corev1.ResourceList
	// SystemReserved returns the default resources allocated to the OS system daemons by default
	SystemReserved corev1.ResourceList
	// EvictionThreshold returns the resources used to maintain a hard eviction threshold
	EvictionThreshold corev1.ResourceList
}

func (i InstanceTypeOverhead) Total() corev1.ResourceList {
	return resources.Merge(i.KubeReserved, i.SystemReserved, i.EvictionThreshold)
}

// An Offering describes where an InstanceType is available to be used, with the expectation that its properties
// may be tightly coupled (e.g. the availability of an instance type in some zone is scoped to a capacity type) and
// these properties are captured with labels in Requirements.
// Requirements are required to contain the keys v1.CapacityTypeLabelKey and corev1.LabelTopologyZone
type Offering struct {
	Requirements scheduling.Requirements
	Price        float64
	// Available is added so that Offerings can return all offerings that have ever existed for an instance type,
	// so we can get historical pricing data for calculating savings in consolidation
	Available bool
}

type Offerings []Offering

// Available filters the available offerings from the returned offerings
func (ofs Offerings) Available() Offerings {
	return lo.Filter(ofs, func(o Offering, _ int) bool {
		return o.Available
	})
}

// Compatible returns the offerings based on the passed requirements
func (ofs Offerings) Compatible(reqs scheduling.Requirements) Offerings {
	return lo.Filter(ofs, func(offering Offering, _ int) bool {
		return reqs.IsCompatible(offering.Requirements, scheduling.AllowUndefinedWellKnownLabels)
	})
}

// HasCompatible returns whether there is a compatible offering based on the passed requirements
func (ofs Offerings) HasCompatible(reqs scheduling.Requirements) bool {
	for _, of := range ofs {
		if reqs.IsCompatible(of.Requirements, scheduling.AllowUndefinedWellKnownLabels) {
			return true
		}
	}
	return false
}

// Cheapest returns the cheapest offering from the returned offerings
func (ofs Offerings) Cheapest() Offering {
	return lo.MinBy(ofs, func(a, b Offering) bool {
		return a.Price < b.Price
	})
}

// MostExpensive returns the most expensive offering from the return offerings
func (ofs Offerings) MostExpensive() Offering {
	return lo.MaxBy(ofs, func(a, b Offering) bool {
		return a.Price > b.Price
	})
}

// WorstLaunchPrice gets the worst-case launch price from the offerings that are offered
// on an instance type. If the instance type has a spot offering available, then it uses the spot offering
// to get the launch price; else, it uses the on-demand launch price
func (ofs Offerings) WorstLaunchPrice(reqs scheduling.Requirements) float64 {
	// We prefer to launch spot offerings, so we will get the worst price based on the node requirements
	if reqs.Get(v1.CapacityTypeLabelKey).Has(v1.CapacityTypeSpot) {
		spotOfferings := ofs.Compatible(reqs).Compatible(SpotRequirement)
		if len(spotOfferings) > 0 {
			return spotOfferings.MostExpensive().Price
		}
	}
	if reqs.Get(v1.CapacityTypeLabelKey).Has(v1.CapacityTypeOnDemand) {
		onDemandOfferings := ofs.Compatible(reqs).Compatible(OnDemandRequirement)
		if len(onDemandOfferings) > 0 {
			return onDemandOfferings.MostExpensive().Price
		}
	}
	return math.MaxFloat64
}

// NodeClaimNotFoundError is an error type returned by CloudProviders when the reason for failure is NotFound
type NodeClaimNotFoundError struct {
	error
}

func NewNodeClaimNotFoundError(err error) *NodeClaimNotFoundError {
	return &NodeClaimNotFoundError{
		error: err,
	}
}

func (e *NodeClaimNotFoundError) Error() string {
	return fmt.Sprintf("nodeclaim not found, %s", e.error)
}

func (e *NodeClaimNotFoundError) Unwrap() error {
	return e.error
}

func IsNodeClaimNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var ncnfErr *NodeClaimNotFoundError
	return errors.As(err, &ncnfErr)
}

func IgnoreNodeClaimNotFoundError(err error) error {
	if IsNodeClaimNotFoundError(err) {
		return nil
	}
	return err
}

// InsufficientCapacityError is an error type returned by CloudProviders when a launch fails due to a lack of capacity from NodeClaim requirements
type InsufficientCapacityError struct {
	error
}

func NewInsufficientCapacityError(err error) *InsufficientCapacityError {
	return &InsufficientCapacityError{
		error: err,
	}
}

func (e *InsufficientCapacityError) Error() string {
	return fmt.Sprintf("insufficient capacity, %s", e.error)
}

func (e *InsufficientCapacityError) Unwrap() error {
	return e.error
}

func IsInsufficientCapacityError(err error) bool {
	if err == nil {
		return false
	}
	var icErr *InsufficientCapacityError
	return errors.As(err, &icErr)
}

// NodeClassNotReadyError is an error type returned by CloudProviders when a NodeClass that is used by the launch process doesn't have all its resolved fields
type NodeClassNotReadyError struct {
	error
}

func NewNodeClassNotReadyError(err error) *NodeClassNotReadyError {
	return &NodeClassNotReadyError{
		error: err,
	}
}

func (e *NodeClassNotReadyError) Error() string {
	return fmt.Sprintf("NodeClassRef not ready, %s", e.error)
}

func (e *NodeClassNotReadyError) Unwrap() error {
	return e.error
}

func IsNodeClassNotReadyError(err error) bool {
	if err == nil {
		return false
	}
	var nrError *NodeClassNotReadyError
	return errors.As(err, &nrError)
}

// CreateError is an error type returned by CloudProviders when instance creation fails
type CreateError struct {
	error
	ConditionReason  string
	ConditionMessage string
}

func NewCreateError(err error, reason, message string) *CreateError {
	return &CreateError{
		error:            err,
		ConditionReason:  reason,
		ConditionMessage: message,
	}
}

func (e *CreateError) Error() string {
	return fmt.Sprintf("creating nodeclaim, %s", e.error)
}

func (e *CreateError) Unwrap() error {
	return e.error
}
