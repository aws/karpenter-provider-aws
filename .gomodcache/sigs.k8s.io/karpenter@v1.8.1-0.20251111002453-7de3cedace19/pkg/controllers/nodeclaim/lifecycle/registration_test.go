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

package lifecycle_test

import (
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	operatorpkg "github.com/awslabs/operatorpkg/test/expectations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Registration", func() {
	var nodePool *v1.NodePool
	var taints []corev1.Taint
	var startupTaints []corev1.Taint
	BeforeEach(func() {
		nodePool = test.NodePool()
		taints = []corev1.Taint{
			{
				Key:    "custom-taint",
				Effect: corev1.TaintEffectNoSchedule,
				Value:  "custom-value",
			},
			{
				Key:    "other-custom-taint",
				Effect: corev1.TaintEffectNoExecute,
				Value:  "other-custom-value",
			},
		}
		startupTaints = []corev1.Taint{
			{
				Key:    "custom-startup-taint",
				Effect: corev1.TaintEffectNoSchedule,
				Value:  "custom-startup-value",
			},
			{
				Key:    "other-custom-startup-taint",
				Effect: corev1.TaintEffectNoExecute,
				Value:  "other-custom-startup-value",
			},
		}

	})
	DescribeTable(
		"Registration",
		func(isManagedNodeClaim bool) {
			ExpectApplied(ctx, env.Client, nodePool)
			nodeClaimOpts := []v1.NodeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         object.GVK(nodePool).GroupVersion().String(),
							Kind:               object.GVK(nodePool).Kind,
							Name:               nodePool.Name,
							UID:                nodePool.UID,
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			}}
			if !isManagedNodeClaim {
				nodeClaimOpts = append(nodeClaimOpts, v1.NodeClaim{
					Spec: v1.NodeClaimSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "UnmanagedNodeClass",
							Name:  "default",
						},
					},
				})
			}
			nodeClaim := test.NodeClaim(nodeClaimOpts...)
			ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

			node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
			ExpectApplied(ctx, env.Client, node)
			ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

			nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
			if isManagedNodeClaim {
				Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
				Expect(nodeClaim.Status.NodeName).To(Equal(node.Name))
				operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{
					Type:   v1.ConditionTypeNodeRegistrationHealthy,
					Status: metav1.ConditionTrue,
				})
			} else {
				Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsUnknown()).To(BeTrue())
				Expect(nodeClaim.Status.NodeName).To(Equal(""))
			}
		},
		Entry("should match the nodeClaim to the Node when the Node comes online", true),
		Entry("should ignore NodeClaims not managed by this Karpenter instance", false),
	)
	It("should add the owner reference to the Node when the Node comes online", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		node = ExpectExists(ctx, env.Client, node)
		ExpectOwnerReferenceExists(node, nodeClaim)
	})
	It("should not add the owner reference to the Node when the Node already has the owner reference", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodeClaim).GroupVersion().String(),
						Kind:               object.GVK(nodeClaim).Kind,
						Name:               nodeClaim.Name,
						UID:                nodeClaim.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		node = ExpectExists(ctx, env.Client, node)
		ExpectOwnerReferenceExists(node, nodeClaim)
		Expect(lo.CountBy(node.OwnerReferences, func(o metav1.OwnerReference) bool {
			return o.APIVersion == object.GVK(nodeClaim).GroupVersion().String() && o.Kind == object.GVK(nodeClaim).Kind && o.UID == nodeClaim.UID
		})).To(Equal(1))
	})
	It("should sync the karpenter.sh/registered label to the Node and remove the karpenter.sh/unregistered taint when the Node comes online", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)
		Expect(node.Labels).To(HaveKeyWithValue(v1.NodeRegisteredLabelKey, "true"))
		Expect(node.Spec.Taints).To(Not(ContainElement(v1.UnregisteredNoExecuteTaint)))
	})
	It("should succeed registration if the karpenter.sh/unregistered taint is not present and emit an event", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		// Create a node without the unregistered taint
		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		// Verify the NodeClaim is registered
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.Status.NodeName).To(Equal(node.Name))

		// Verify the node is registered
		node = ExpectExists(ctx, env.Client, node)
		Expect(node.Labels).To(HaveKeyWithValue(v1.NodeRegisteredLabelKey, "true"))

		Expect(recorder.Calls(events.UnregisteredTaintMissing)).To(Equal(1))
	})

	It("should sync the labels to the Node when the Node comes online", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:  nodePool.Name,
					"custom-label":       "custom-value",
					"other-custom-label": "other-custom-value",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(nodeClaim.Labels).To(HaveKeyWithValue("other-custom-label", "other-custom-value"))

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		// Expect Node to have all the labels that the nodeClaim has
		for k, v := range nodeClaim.Labels {
			Expect(node.Labels).To(HaveKeyWithValue(k, v))
		}
	})
	It("should sync the annotations to the Node when the Node comes online", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
					"my-custom-annotation":       "my-custom-value",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.DoNotDisruptAnnotationKey, "true"))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue("my-custom-annotation", "my-custom-value"))

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		// Expect Node to have all the annotations that the nodeClaim has
		for k, v := range nodeClaim.Annotations {
			Expect(node.Annotations).To(HaveKeyWithValue(k, v))
		}
	})
	It("should sync the taints to the Node when the Node comes online, if node label do not sync taints is not present", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{Taints: taints},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(ContainElements(taints))
	})
	It("should sync the taints to the Node when the Node comes online, if node label do not sync taints is present but key is not true", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{Taints: taints},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodeDoNotSyncTaintsLabelKey: "false"},
			},
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(ContainElements(taints))
	})
	It("should not sync the taints to the Node when the Node comes online, with node label do not sync taints", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{Taints: taints},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodeDoNotSyncTaintsLabelKey: "true"},
			},
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(HaveLen(1))
		Expect(node.Spec.Taints).To(Not(ContainElements(taints)))
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))
		Expect(node.Spec.Taints).To(ContainElements(corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule}))
	})
	It("should not sync the startupTaints to the Node when the Node comes online, with node label do not sync taints", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Taints:        taints,
				StartupTaints: startupTaints,
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.StartupTaints).To(ContainElements(startupTaints))

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodeDoNotSyncTaintsLabelKey: "true"},
			},
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(HaveLen(1))
		Expect(node.Spec.Taints).To(Not(ContainElements(startupTaints)))
		Expect(nodeClaim.Spec.StartupTaints).To(ContainElements(startupTaints))
		Expect(node.Spec.Taints).To(ContainElements(corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule}))
	})
	It("should sync the startupTaints to the Node when the Node comes online, if node label do not sync taints is not present", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Taints:        taints,
				StartupTaints: startupTaints,
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))
		Expect(nodeClaim.Spec.StartupTaints).To(ContainElements(startupTaints))

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(ContainElements(startupTaints))
	})
	It("should sync the startupTaints to the Node when the Node comes online, if node label do not sync taints is present but key is not true", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Taints:        taints,
				StartupTaints: startupTaints,
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.Spec.Taints).To(ContainElements(taints))
		Expect(nodeClaim.Spec.StartupTaints).To(ContainElements(startupTaints))

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodeDoNotSyncTaintsLabelKey: "false"},
			},
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(ContainElements(startupTaints))
	})
	It("should not re-sync the startupTaints to the Node when the startupTaints are removed", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: v1.NodeClaimSpec{StartupTaints: startupTaints},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{
			ProviderID: nodeClaim.Status.ProviderID,
			Taints:     append(startupTaints, v1.UnregisteredNoExecuteTaint),
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)

		Expect(node.Spec.Taints).To(ContainElements(startupTaints))
		node.Spec.Taints = []corev1.Taint{}
		ExpectApplied(ctx, env.Client, node)

		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		node = ExpectExists(ctx, env.Client, node)
		Expect(node.Spec.Taints).To(HaveLen(0))
	})
	It("should add NodeRegistrationHealthy=true on the nodePool if registration succeeds and if it was previously false", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		nodeClaimOpts := []v1.NodeClaim{{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodePool).GroupVersion().String(),
						Kind:               object.GVK(nodePool).Kind,
						Name:               nodePool.Name,
						UID:                nodePool.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		}}
		nodeClaim := test.NodeClaim(nodeClaimOpts...)
		nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "unhealthy", "unhealthy")
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.Status.NodeName).To(Equal(node.Name))
		operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{
			Type:   v1.ConditionTypeNodeRegistrationHealthy,
			Status: metav1.ConditionTrue,
		})
	})
	It("should not add NodeRegistrationHealthy=true on the nodePool if registration succeeds once and if it had previously failed twice in the registrationHealthBuffer", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		nodeClaim1 := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodePool).GroupVersion().String(),
						Kind:               object.GVK(nodePool).Kind,
						Name:               nodePool.Name,
						UID:                nodePool.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})
		nodeClaim2 := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodePool).GroupVersion().String(),
						Kind:               object.GVK(nodePool).Kind,
						Name:               nodePool.Name,
						UID:                nodePool.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})
		cloudProvider.AllowedCreateCalls = 0 // Don't allow Create() calls to succeed
		ExpectApplied(ctx, env.Client, nodeClaim1, nodeClaim2)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim1)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim2)

		// If the node hasn't registered in the registration timeframe, then we deprovision the nodeClaim
		fakeClock.Step(time.Minute * 6)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim1)
		_ = ExpectObjectReconcileFailed(ctx, env.Client, nodeClaimController, nodeClaim2)

		// NodeClaim registration failed twice which is greater than our threshold so update the NodeRegistrationHealthy status condition
		operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{Type: v1.ConditionTypeNodeRegistrationHealthy, Status: metav1.ConditionFalse})
		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusUnhealthy))
		ExpectFinalizersRemoved(ctx, env.Client, nodeClaim1, nodeClaim2)
		ExpectNotFound(ctx, env.Client, nodeClaim1, nodeClaim2)

		cloudProvider.Reset()
		nodeClaim3 := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodePool).GroupVersion().String(),
						Kind:               object.GVK(nodePool).Kind,
						Name:               nodePool.Name,
						UID:                nodePool.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim3)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim3)
		nodeClaim3 = ExpectExists(ctx, env.Client, nodeClaim3)

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim3.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim3)

		nodeClaim3 = ExpectExists(ctx, env.Client, nodeClaim3)
		Expect(nodeClaim3.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim3.Status.NodeName).To(Equal(node.Name))

		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusUnhealthy))
		operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{
			Type:   v1.ConditionTypeNodeRegistrationHealthy,
			Status: metav1.ConditionFalse,
		})
	})
	It("should not update NodeRegistrationHealthy status condition if nodePool owning the nodeClaim is deleted", func() {
		nodePool = test.NodePool(
			v1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepool",
				},
			},
		)
		ExpectApplied(ctx, env.Client, nodePool)
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         object.GVK(nodePool).GroupVersion().String(),
						Kind:               object.GVK(nodePool).Kind,
						Name:               nodePool.Name,
						UID:                nodePool.UID,
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		// Delete the nodePool referenced on the nodeClaim
		ExpectDeleted(ctx, env.Client, nodePool)
		//Recreate the nodePool so that it will have a different UID
		nodePool = test.NodePool(
			v1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepool",
				},
			},
		)
		ExpectApplied(ctx, env.Client, nodePool)
		operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{Type: v1.ConditionTypeNodeRegistrationHealthy, Status: metav1.ConditionUnknown})

		node := test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID, Taints: []corev1.Taint{v1.UnregisteredNoExecuteTaint}})
		ExpectApplied(ctx, env.Client, node)
		ExpectObjectReconciled(ctx, env.Client, nodeClaimController, nodeClaim)
		operatorpkg.ExpectStatusConditions(ctx, env.Client, 1*time.Minute, nodePool, status.Condition{Type: v1.ConditionTypeNodeRegistrationHealthy, Status: metav1.ConditionUnknown})

		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)
		Expect(nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue()).To(BeTrue())
		Expect(nodeClaim.Status.NodeName).To(Equal(node.Name))
	})
})
