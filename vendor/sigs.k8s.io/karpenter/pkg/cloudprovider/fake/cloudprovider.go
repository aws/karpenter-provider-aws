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

package fake

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

func init() {
	v1.WellKnownLabels = v1.WellKnownLabels.Insert(v1alpha1.LabelReservationID)
	cloudprovider.ReservationIDLabel = v1alpha1.LabelReservationID
	cloudprovider.ReservedCapacityLabels.Insert(v1alpha1.LabelReservationID)
}

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	InstanceTypes            []*cloudprovider.InstanceType
	InstanceTypesForNodePool map[string][]*cloudprovider.InstanceType
	ErrorsForNodePool        map[string]error

	mu sync.RWMutex
	// CreateCalls contains the arguments for every create call that was made since it was cleared
	CreateCalls        []*v1.NodeClaim
	AllowedCreateCalls int
	NextCreateErr      error
	NextGetErr         error
	NextDeleteErr      error
	DeleteCalls        []*v1.NodeClaim
	GetCalls           []string

	CreatedNodeClaims         map[string]*v1.NodeClaim
	Drifted                   cloudprovider.DriftReason
	NodeClassGroupVersionKind []schema.GroupVersionKind
	RepairPolicy              []cloudprovider.RepairPolicy
}

func NewCloudProvider() *CloudProvider {
	return &CloudProvider{
		AllowedCreateCalls:       math.MaxInt,
		CreatedNodeClaims:        map[string]*v1.NodeClaim{},
		InstanceTypesForNodePool: map[string][]*cloudprovider.InstanceType{},
		ErrorsForNodePool:        map[string]error{},
	}
}

// Reset is for BeforeEach calls in testing to reset the tracking of CreateCalls
func (c *CloudProvider) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CreateCalls = nil
	c.CreatedNodeClaims = map[string]*v1.NodeClaim{}
	c.InstanceTypes = nil
	c.InstanceTypesForNodePool = map[string][]*cloudprovider.InstanceType{}
	c.ErrorsForNodePool = map[string]error{}
	c.AllowedCreateCalls = math.MaxInt
	c.NextCreateErr = nil
	c.NextDeleteErr = nil
	c.NextGetErr = nil
	c.DeleteCalls = []*v1.NodeClaim{}
	c.GetCalls = nil
	c.Drifted = ""
	c.NodeClassGroupVersionKind = []schema.GroupVersionKind{
		{
			Group:   "",
			Version: "",
			Kind:    "",
		},
	}
	c.RepairPolicy = []cloudprovider.RepairPolicy{
		{
			ConditionType:      "BadNode",
			ConditionStatus:    corev1.ConditionFalse,
			TolerationDuration: 30 * time.Minute,
		},
	}
}

//nolint:gocyclo
func (c *CloudProvider) Create(ctx context.Context, nodeClaim *v1.NodeClaim) (*v1.NodeClaim, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.NextCreateErr != nil {
		temp := c.NextCreateErr
		c.NextCreateErr = nil
		return nil, temp
	}

	c.CreateCalls = append(c.CreateCalls, nodeClaim)
	if len(c.CreateCalls) > c.AllowedCreateCalls {
		return &v1.NodeClaim{}, fmt.Errorf("erroring as number of AllowedCreateCalls has been exceeded")
	}
	reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...)
	np := &v1.NodePool{ObjectMeta: metav1.ObjectMeta{Name: nodeClaim.Labels[v1.NodePoolLabelKey]}}
	instanceTypes := lo.Filter(lo.Must(c.GetInstanceTypes(ctx, np)), func(i *cloudprovider.InstanceType, _ int) bool {
		if !reqs.IsCompatible(i.Requirements, scheduling.AllowUndefinedWellKnownLabels) {
			return false
		}
		if !i.Offerings.Available().HasCompatible(reqs) {
			return false
		}
		if !resources.Fits(nodeClaim.Spec.Resources.Requests, i.Allocatable()) {
			return false
		}
		return true
	})
	// Order instance types so that we get the cheapest instance types of the available offerings
	sort.Slice(instanceTypes, func(i, j int) bool {
		iOfferings := instanceTypes[i].Offerings.Available().Compatible(reqs)
		jOfferings := instanceTypes[j].Offerings.Available().Compatible(reqs)
		return iOfferings.Cheapest().Price < jOfferings.Cheapest().Price
	})
	instanceType := instanceTypes[0]
	// Labels
	labels := map[string]string{}
	for key, requirement := range instanceType.Requirements {
		if requirement.Operator() == corev1.NodeSelectorOpIn {
			labels[key] = requirement.Values()[0]
		}
	}
	// Find offering, prioritizing reserved instances
	var offering *cloudprovider.Offering
	offerings := instanceType.Offerings.Available().Compatible(reqs)
	lo.Must0(len(offerings) != 0, "created nodeclaim with no available offerings")
	for _, o := range offerings {
		if o.CapacityType() == v1.CapacityTypeReserved {
			o.ReservationCapacity -= 1
			if o.ReservationCapacity == 0 {
				o.Available = false
			}
			offering = o
			break
		}
	}
	if offering == nil {
		offering = offerings[0]
	}
	// Propagate labels dictated by offering requirements - e.g. zone, capacity-type, and reservation-id
	for _, req := range offering.Requirements {
		labels[req.Key] = req.Any()
	}

	created := &v1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nodeClaim.Name,
			Labels:      lo.Assign(labels, nodeClaim.Labels),
			Annotations: nodeClaim.Annotations,
		},
		Spec: *nodeClaim.Spec.DeepCopy(),
		Status: v1.NodeClaimStatus{
			ProviderID:  test.RandomProviderID(),
			Capacity:    lo.PickBy(instanceType.Capacity, func(_ corev1.ResourceName, v resource.Quantity) bool { return !resources.IsZero(v) }),
			Allocatable: lo.PickBy(instanceType.Allocatable(), func(_ corev1.ResourceName, v resource.Quantity) bool { return !resources.IsZero(v) }),
		},
	}
	c.CreatedNodeClaims[created.Status.ProviderID] = created
	return created, nil
}

func (c *CloudProvider) Get(_ context.Context, id string) (*v1.NodeClaim, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.NextGetErr != nil {
		tempError := c.NextGetErr
		c.NextGetErr = nil
		return nil, tempError
	}
	c.GetCalls = append(c.GetCalls, id)
	if nodeClaim, ok := c.CreatedNodeClaims[id]; ok {
		return nodeClaim.DeepCopy(), nil
	}
	return nil, cloudprovider.NewNodeClaimNotFoundError(serrors.Wrap(fmt.Errorf("no nodeclaim exists with id"), "id", id))
}

func (c *CloudProvider) List(_ context.Context) ([]*v1.NodeClaim, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return lo.Map(lo.Values(c.CreatedNodeClaims), func(nc *v1.NodeClaim, _ int) *v1.NodeClaim {
		return nc.DeepCopy()
	}), nil
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, np *v1.NodePool) ([]*cloudprovider.InstanceType, error) {
	if np != nil {
		if err, ok := c.ErrorsForNodePool[np.Name]; ok {
			return nil, err
		}

		if v, ok := c.InstanceTypesForNodePool[np.Name]; ok {
			return v, nil
		}
	}
	if c.InstanceTypes != nil {
		return c.InstanceTypes, nil
	}
	return []*cloudprovider.InstanceType{
		NewInstanceType(InstanceTypeOptions{
			Name: "default-instance-type",
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "small-instance-type",
			Resources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "gpu-vendor-instance-type",
			Resources: map[corev1.ResourceName]resource.Quantity{
				ResourceGPUVendorA: resource.MustParse("2"),
			}}),
		NewInstanceType(InstanceTypeOptions{
			Name: "gpu-vendor-b-instance-type",
			Resources: map[corev1.ResourceName]resource.Quantity{
				ResourceGPUVendorB: resource.MustParse("2"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:             "arm-instance-type",
			Architecture:     "arm64",
			OperatingSystems: sets.New("ios", string(corev1.Linux), string(corev1.Windows), "darwin"),
			Resources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("128Gi"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "single-pod-instance-type",
			Resources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourcePods: resource.MustParse("1"),
			},
		}),
	}, nil
}

func (c *CloudProvider) Delete(_ context.Context, nc *v1.NodeClaim) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.NextDeleteErr != nil {
		tempError := c.NextDeleteErr
		c.NextDeleteErr = nil
		return tempError
	}

	c.DeleteCalls = append(c.DeleteCalls, nc)
	if _, ok := c.CreatedNodeClaims[nc.Status.ProviderID]; ok {
		delete(c.CreatedNodeClaims, nc.Status.ProviderID)
		return nil
	}
	return cloudprovider.NewNodeClaimNotFoundError(serrors.Wrap(fmt.Errorf("no nodeclaim exists with provider id"), "provider-id", nc.Status.ProviderID))
}

func (c *CloudProvider) IsDrifted(context.Context, *v1.NodeClaim) (cloudprovider.DriftReason, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Drifted, nil
}

func (c *CloudProvider) RepairPolicies() []cloudprovider.RepairPolicy {
	return c.RepairPolicy
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "fake"
}

func (c *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1alpha1.TestNodeClass{}}
}
