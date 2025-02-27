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

package v1

import (
	"fmt"
	"math"
	"strconv"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/clock"
)

// NodePoolSpec is the top level nodepool specification. Nodepools
// launch nodes in response to pods that are unschedulable. A single nodepool
// is capable of managing a diverse set of nodes. Node properties are determined
// from a combination of nodepool and pod scheduling constraints.
type NodePoolSpec struct {
	// Template contains the template of possibilities for the provisioning logic to launch a NodeClaim with.
	// NodeClaims launched from this NodePool will often be further constrained than the template specifies.
	// +required
	Template NodeClaimTemplate `json:"template"`
	// Disruption contains the parameters that relate to Karpenter's disruption logic
	// +kubebuilder:default:={consolidateAfter: "0s"}
	// +optional
	Disruption Disruption `json:"disruption"`
	// Limits define a set of bounds for provisioning capacity.
	// +optional
	Limits Limits `json:"limits,omitempty"`
	// Weight is the priority given to the nodepool during scheduling. A higher
	// numerical weight indicates that this nodepool will be ordered
	// ahead of other nodepools with lower weights. A nodepool with no weight
	// will be treated as if it is a nodepool with a weight of 0.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=100
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

type Disruption struct {
	// ConsolidateAfter is the duration the controller will wait
	// before attempting to terminate nodes that are underutilized.
	// Refer to ConsolidationPolicy for how underutilization is considered.
	// +kubebuilder:validation:Pattern=`^(([0-9]+(s|m|h))+)|(Never)$`
	// +kubebuilder:validation:Type="string"
	// +kubebuilder:validation:Schemaless
	// +required
	ConsolidateAfter NillableDuration `json:"consolidateAfter"`
	// ConsolidationPolicy describes which nodes Karpenter can disrupt through its consolidation
	// algorithm. This policy defaults to "WhenEmptyOrUnderutilized" if not specified
	// +kubebuilder:default:="WhenEmptyOrUnderutilized"
	// +kubebuilder:validation:Enum:={WhenEmpty,WhenEmptyOrUnderutilized}
	// +optional
	ConsolidationPolicy ConsolidationPolicy `json:"consolidationPolicy,omitempty"`
	// Budgets is a list of Budgets.
	// If there are multiple active budgets, Karpenter uses
	// the most restrictive value. If left undefined,
	// this will default to one budget with a value to 10%.
	// +kubebuilder:validation:XValidation:message="'schedule' must be set with 'duration'",rule="self.all(x, has(x.schedule) == has(x.duration))"
	// +kubebuilder:default:={{nodes: "10%"}}
	// +kubebuilder:validation:MaxItems=50
	// +optional
	Budgets []Budget `json:"budgets,omitempty" hash:"ignore"`
}

// Budget defines when Karpenter will restrict the
// number of Node Claims that can be terminating simultaneously.
type Budget struct {
	// Reasons is a list of disruption methods that this budget applies to. If Reasons is not set, this budget applies to all methods.
	// Otherwise, this will apply to each reason defined.
	// allowed reasons are Underutilized, Empty, and Drifted.
	// +optional
	Reasons []DisruptionReason `json:"reasons,omitempty"`
	// Nodes dictates the maximum number of NodeClaims owned by this NodePool
	// that can be terminating at once. This is calculated by counting nodes that
	// have a deletion timestamp set, or are actively being deleted by Karpenter.
	// This field is required when specifying a budget.
	// This cannot be of type intstr.IntOrString since kubebuilder doesn't support pattern
	// checking for int nodes for IntOrString nodes.
	// Ref: https://github.com/kubernetes-sigs/controller-tools/blob/55efe4be40394a288216dab63156b0a64fb82929/pkg/crd/markers/validation.go#L379-L388
	// +kubebuilder:validation:Pattern:="^((100|[0-9]{1,2})%|[0-9]+)$"
	// +kubebuilder:default:="10%"
	Nodes string `json:"nodes" hash:"ignore"`
	// Schedule specifies when a budget begins being active, following
	// the upstream cronjob syntax. If omitted, the budget is always active.
	// Timezones are not supported.
	// This field is required if Duration is set.
	// +kubebuilder:validation:Pattern:=`^(@(annually|yearly|monthly|weekly|daily|midnight|hourly))|((.+)\s(.+)\s(.+)\s(.+)\s(.+))$`
	// +optional
	Schedule *string `json:"schedule,omitempty" hash:"ignore"`
	// Duration determines how long a Budget is active since each Schedule hit.
	// Only minutes and hours are accepted, as cron does not work in seconds.
	// If omitted, the budget is always active.
	// This is required if Schedule is set.
	// This regex has an optional 0s at the end since the duration.String() always adds
	// a 0s at the end.
	// +kubebuilder:validation:Pattern=`^((([0-9]+(h|m))|([0-9]+h[0-9]+m))(0s)?)$`
	// +kubebuilder:validation:Type="string"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty" hash:"ignore"`
}

type ConsolidationPolicy string

const (
	ConsolidationPolicyWhenEmpty                ConsolidationPolicy = "WhenEmpty"
	ConsolidationPolicyWhenEmptyOrUnderutilized ConsolidationPolicy = "WhenEmptyOrUnderutilized"
)

// DisruptionReason defines valid reasons for disruption budgets.
// +kubebuilder:validation:Enum={Underutilized,Empty,Drifted}
type DisruptionReason string

const (
	DisruptionReasonUnderutilized DisruptionReason = "Underutilized"
	DisruptionReasonEmpty         DisruptionReason = "Empty"
	DisruptionReasonDrifted       DisruptionReason = "Drifted"
)

type Limits v1.ResourceList

func (l Limits) ExceededBy(resources v1.ResourceList) error {
	if l == nil {
		return nil
	}
	for resourceName, usage := range resources {
		if limit, ok := l[resourceName]; ok {
			if usage.Cmp(limit) > 0 {
				return fmt.Errorf("%s resource usage of %v exceeds limit of %v", resourceName, usage.AsDec(), limit.AsDec())
			}
		}
	}
	return nil
}

type NodeClaimTemplate struct {
	ObjectMeta `json:"metadata,omitempty"`
	// +required
	Spec NodeClaimTemplateSpec `json:"spec"`
}

// NodeClaimTemplateSpec describes the desired state of the NodeClaim in the Nodepool
// NodeClaimTemplateSpec is used in the NodePool's NodeClaimTemplate, with the resource requests omitted since
// users are not able to set resource requests in the NodePool.
type NodeClaimTemplateSpec struct {
	// Taints will be applied to the NodeClaim's node.
	// +optional
	Taints []v1.Taint `json:"taints,omitempty"`
	// StartupTaints are taints that are applied to nodes upon startup which are expected to be removed automatically
	// within a short period of time, typically by a DaemonSet that tolerates the taint. These are commonly used by
	// daemonsets to allow initialization and enforce startup ordering.  StartupTaints are ignored for provisioning
	// purposes in that pods are not required to tolerate a StartupTaint in order to have nodes provisioned for them.
	// +optional
	StartupTaints []v1.Taint `json:"startupTaints,omitempty"`
	// Requirements are layered with GetLabels and applied to every node.
	// +kubebuilder:validation:XValidation:message="requirements with operator 'In' must have a value defined",rule="self.all(x, x.operator == 'In' ? x.values.size() != 0 : true)"
	// +kubebuilder:validation:XValidation:message="requirements operator 'Gt' or 'Lt' must have a single positive integer value",rule="self.all(x, (x.operator == 'Gt' || x.operator == 'Lt') ? (x.values.size() == 1 && int(x.values[0]) >= 0) : true)"
	// +kubebuilder:validation:XValidation:message="requirements with 'minValues' must have at least that many values specified in the 'values' field",rule="self.all(x, (x.operator == 'In' && has(x.minValues)) ? x.values.size() >= x.minValues : true)"
	// +kubebuilder:validation:MaxItems:=100
	// +required
	Requirements []NodeSelectorRequirementWithMinValues `json:"requirements" hash:"ignore"`
	// NodeClassRef is a reference to an object that defines provider specific configuration
	// +kubebuilder:validation:XValidation:rule="self.group == oldSelf.group",message="nodeClassRef.group is immutable"
	// +kubebuilder:validation:XValidation:rule="self.kind == oldSelf.kind",message="nodeClassRef.kind is immutable"
	// +required
	NodeClassRef *NodeClassReference `json:"nodeClassRef"`
	// TerminationGracePeriod is the maximum duration the controller will wait before forcefully deleting the pods on a node, measured from when deletion is first initiated.
	//
	// Warning: this feature takes precedence over a Pod's terminationGracePeriodSeconds value, and bypasses any blocked PDBs or the karpenter.sh/do-not-disrupt annotation.
	//
	// This field is intended to be used by cluster administrators to enforce that nodes can be cycled within a given time period.
	// When set, drifted nodes will begin draining even if there are pods blocking eviction. Draining will respect PDBs and the do-not-disrupt annotation until the TGP is reached.
	//
	// Karpenter will preemptively delete pods so their terminationGracePeriodSeconds align with the node's terminationGracePeriod.
	// If a pod would be terminated without being granted its full terminationGracePeriodSeconds prior to the node timeout,
	// that pod will be deleted at T = node timeout - pod terminationGracePeriodSeconds.
	//
	// The feature can also be used to allow maximum time limits for long-running jobs which can delay node termination with preStop hooks.
	// If left undefined, the controller will wait indefinitely for pods to be drained.
	// +kubebuilder:validation:Pattern=`^([0-9]+(s|m|h))+$`
	// +kubebuilder:validation:Type="string"
	// +optional
	TerminationGracePeriod *metav1.Duration `json:"terminationGracePeriod,omitempty"`
	// ExpireAfter is the duration the controller will wait
	// before terminating a node, measured from when the node is created. This
	// is useful to implement features like eventually consistent node upgrade,
	// memory leak protection, and disruption testing.
	// +kubebuilder:default:="720h"
	// +kubebuilder:validation:Pattern=`^(([0-9]+(s|m|h))+)|(Never)$`
	// +kubebuilder:validation:Type="string"
	// +kubebuilder:validation:Schemaless
	// +optional
	ExpireAfter NillableDuration `json:"expireAfter,omitempty"`
}

// This is used to convert between the NodeClaim's NodeClaimSpec to the Nodepool NodeClaimTemplate's NodeClaimSpec.
func (in *NodeClaimTemplate) ToNodeClaim() *NodeClaim {
	return &NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      in.ObjectMeta.Labels,
			Annotations: in.ObjectMeta.Annotations,
		},
		Spec: NodeClaimSpec{
			Taints:                 in.Spec.Taints,
			StartupTaints:          in.Spec.StartupTaints,
			Requirements:           in.Spec.Requirements,
			NodeClassRef:           in.Spec.NodeClassRef,
			TerminationGracePeriod: in.Spec.TerminationGracePeriod,
			ExpireAfter:            in.Spec.ExpireAfter,
		},
	}
}

type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NodePool is the Schema for the NodePools API
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=nodepools,scope=Cluster,categories=karpenter
// +kubebuilder:printcolumn:name="NodeClass",type="string",JSONPath=".spec.template.spec.nodeClassRef.name",description=""
// +kubebuilder:printcolumn:name="Nodes",type="string",JSONPath=".status.resources.nodes",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Weight",type="integer",JSONPath=".spec.weight",priority=1,description=""
// +kubebuilder:printcolumn:name="CPU",type="string",JSONPath=".status.resources.cpu",priority=1,description=""
// +kubebuilder:printcolumn:name="Memory",type="string",JSONPath=".status.resources.memory",priority=1,description=""
// +kubebuilder:subresource:status
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   NodePoolSpec   `json:"spec"`
	Status NodePoolStatus `json:"status,omitempty"`
}

// We need to bump the NodePoolHashVersion when we make an update to the NodePool CRD under these conditions:
// 1. A field changes its default value for an existing field that is already hashed
// 2. A field is added to the hash calculation with an already-set value
// 3. A field is removed from the hash calculations
const NodePoolHashVersion = "v3"

func (in *NodePool) Hash() string {
	return fmt.Sprint(lo.Must(hashstructure.Hash(in.Spec.Template, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})))
}

// NodePoolList contains a list of NodePool
// +kubebuilder:object:root=true
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool `json:"items"`
}

// MustGetAllowedDisruptions calls GetAllowedDisruptionsByReason if the error is not nil. This reduces the
// amount of state that the disruption controller must reconcile, while allowing the GetAllowedDisruptionsByReason()
// to bubble up any errors in validation.
func (in *NodePool) MustGetAllowedDisruptions(c clock.Clock, numNodes int, reason DisruptionReason) int {
	allowedDisruptions, err := in.GetAllowedDisruptionsByReason(c, numNodes, reason)
	if err != nil {
		return 0
	}
	return allowedDisruptions
}

// GetAllowedDisruptionsByReason returns the minimum allowed disruptions across all disruption budgets, for all disruption methods for a given nodepool
func (in *NodePool) GetAllowedDisruptionsByReason(c clock.Clock, numNodes int, reason DisruptionReason) (int, error) {
	allowedNodes := math.MaxInt32
	var multiErr error
	for _, budget := range in.Spec.Disruption.Budgets {
		val, err := budget.GetAllowedDisruptions(c, numNodes)
		if err != nil {
			multiErr = multierr.Append(multiErr, err)
		}
		if budget.Reasons == nil || lo.Contains(budget.Reasons, reason) {
			allowedNodes = lo.Min([]int{allowedNodes, val})
		}
	}
	return allowedNodes, multiErr
}

// GetAllowedDisruptions returns an intstr.IntOrString that can be used a comparison
// for calculating if a disruption action is allowed. It returns an error if the
// schedule is invalid. This returns MAXINT if the value is unbounded.
func (in *Budget) GetAllowedDisruptions(c clock.Clock, numNodes int) (int, error) {
	active, err := in.IsActive(c)
	// If the budget is misconfigured, fail closed.
	if err != nil {
		return 0, err
	}
	if !active {
		return math.MaxInt32, nil
	}
	// This will round up to the nearest whole number. Therefore, a disruption can
	// sometimes exceed the disruption budget. This is the same as how Kubernetes
	// handles MaxUnavailable with PDBs. Take the case with 5% disruptions, but
	// 10 nodes. Karpenter will opt to allow 1 node to be disrupted, rather than
	// blocking all disruptions for this nodepool.
	res, err := intstr.GetScaledValueFromIntOrPercent(lo.ToPtr(GetIntStrFromValue(in.Nodes)), numNodes, true)
	if err != nil {
		// Should never happen since this is validated when the nodepool is applied
		// If this value is incorrectly formatted, fail closed, since we don't know what
		// they want here.
		return 0, err
	}
	return res, nil
}

// IsActive takes a clock as input and returns if a budget is active.
// It walks back in time the time.Duration associated with the schedule,
// and checks if the next time the schedule will hit is before the current time.
// If the last schedule hit is exactly the duration in the past, this means the
// schedule is active, as any more schedule hits in between would only extend this
// window. This ensures that any previous schedule hits for a schedule are considered.
func (in *Budget) IsActive(c clock.Clock) (bool, error) {
	if in.Schedule == nil && in.Duration == nil {
		return true, nil
	}
	schedule, err := cron.ParseStandard(fmt.Sprintf("TZ=UTC %s", lo.FromPtr(in.Schedule)))
	if err != nil {
		// Should only occur if there's a discrepancy
		// with the validation regex and the cron package.
		return false, fmt.Errorf("invariant violated, invalid cron %s", schedule)
	}
	// Walk back in time for the duration associated with the schedule
	checkPoint := c.Now().UTC().Add(-lo.FromPtr(in.Duration).Duration)
	nextHit := schedule.Next(checkPoint)
	return !nextHit.After(c.Now().UTC()), nil
}

func GetIntStrFromValue(str string) intstr.IntOrString {
	// If err is nil, we treat it as an int.
	if intVal, err := strconv.Atoi(str); err == nil {
		return intstr.FromInt(intVal)
	}
	return intstr.FromString(str)
}
