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

package scheduling

import (
	"fmt"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

// DefaultTerminationGracePeriod is used as runtime defaulting for TerminationGracePeriod on the NodeClaim
// This would be a mechanism to allow cloud providers to enforce a TerminationGracePeriod on all node
// provisioned by Karpenter
var DefaultTerminationGracePeriod *metav1.Duration = nil

// MaxInstanceTypes is a constant that restricts the number of instance types to be sent for launch. Note that this
// is intentionally changed to var just to help in testing the code.
var MaxInstanceTypes = 600

// NodeClaimTemplate encapsulates the fields required to create a node and mirrors
// the fields in NodePool. These structs are maintained separately in order
// for fields like Requirements to be able to be stored more efficiently.
type NodeClaimTemplate struct {
	v1.NodeClaim

	NodePoolName        string
	NodePoolUUID        types.UID
	NodePoolWeight      int32
	InstanceTypeOptions cloudprovider.InstanceTypes
	Requirements        scheduling.Requirements
	IsStaticNodeClaim   bool
}

func NewNodeClaimTemplate(nodePool *v1.NodePool) *NodeClaimTemplate {
	nct := &NodeClaimTemplate{
		NodeClaim:         *nodePool.Spec.Template.ToNodeClaim(),
		NodePoolName:      nodePool.Name,
		NodePoolUUID:      nodePool.UID,
		NodePoolWeight:    lo.FromPtr(nodePool.Spec.Weight),
		Requirements:      scheduling.NewRequirements(),
		IsStaticNodeClaim: nodePool.Spec.Replicas != nil,
	}
	nct.Annotations = lo.Assign(nct.Annotations, map[string]string{
		v1.NodePoolHashAnnotationKey:        nodePool.Hash(),
		v1.NodePoolHashVersionAnnotationKey: v1.NodePoolHashVersion,
	})
	nct.Labels = lo.Assign(nct.Labels, map[string]string{
		v1.NodePoolLabelKey: nodePool.Name,
		v1.NodeClassLabelKey(nodePool.Spec.Template.Spec.NodeClassRef.GroupKind()): nodePool.Spec.Template.Spec.NodeClassRef.Name,
	})
	nct.Requirements.Add(scheduling.NewNodeSelectorRequirementsWithMinValues(nct.Spec.Requirements...).Values()...)
	nct.Requirements.Add(scheduling.NewLabelRequirements(nct.Labels).Values()...)
	return nct
}

func (i *NodeClaimTemplate) ToNodeClaim() *v1.NodeClaim {
	// Inject instanceType requirements for NodeClaims belonging to dynamic NodePool
	// For static we let cloudprovider.Create()
	if !i.IsStaticNodeClaim {
		// Order the instance types by price and only take up to MaxInstanceTypes of them to decrease the instance type size in the requirements
		instanceTypes := lo.Slice(i.InstanceTypeOptions.OrderByPrice(i.Requirements), 0, MaxInstanceTypes)
		i.Requirements.Add(scheduling.NewRequirementWithFlexibility(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, i.Requirements.Get(corev1.LabelInstanceTypeStable).MinValues, lo.Map(instanceTypes, func(i *cloudprovider.InstanceType, _ int) string {
			return i.Name
		})...))
		if foundPriceOverlay := lo.ContainsBy(instanceTypes, func(it *cloudprovider.InstanceType) bool { return it.IsPricingOverlayApplied() }); foundPriceOverlay {
			i.Annotations = lo.Assign(i.Annotations, map[string]string{
				v1alpha1.PriceOverlayAppliedAnnotationKey: "true",
			})
		}
		if foundCapacityOverlay := lo.ContainsBy(instanceTypes, func(it *cloudprovider.InstanceType) bool { return it.IsCapacityOverlayApplied() }); foundCapacityOverlay {
			i.Annotations = lo.Assign(i.Annotations, map[string]string{
				v1alpha1.CapacityOverlayAppliedAnnotationKey: "true",
			})
		}
	}

	nc := &v1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", i.NodePoolName),
			Annotations:  i.Annotations,
			Labels:       i.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         object.GVK(&v1.NodePool{}).GroupVersion().String(),
					Kind:               object.GVK(&v1.NodePool{}).Kind,
					Name:               i.NodePoolName,
					UID:                i.NodePoolUUID,
					BlockOwnerDeletion: lo.ToPtr(true),
				},
			},
		},
		Spec: i.Spec,
	}
	nc.Spec.Requirements = i.Requirements.NodeSelectorRequirements()
	if nc.Spec.TerminationGracePeriod == nil {
		nc.Spec.TerminationGracePeriod = DefaultTerminationGracePeriod
	}

	return nc
}
