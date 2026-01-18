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

package provisioning_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	pscheduling "sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/metrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx                 context.Context
	fakeClock           *clock.FakeClock
	cluster             *state.Cluster
	nodeController      *informer.NodeController
	daemonsetController *informer.DaemonSetController
	cloudProvider       *fake.CloudProvider
	prov                *provisioning.Provisioner
	env                 *test.Environment
	instanceTypeMap     map[string]*cloudprovider.InstanceType
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Provisioning")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeController = informer.NewNodeController(env.Client, cluster)
	prov = provisioning.NewProvisioner(env.Client, events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster, fakeClock)
	daemonsetController = informer.NewDaemonSetController(env.Client, cluster)
	instanceTypes, _ := cloudProvider.GetInstanceTypes(ctx, nil)
	instanceTypeMap = map[string]*cloudprovider.InstanceType{}
	for _, it := range instanceTypes {
		instanceTypeMap[it.Name] = it
	}
})

var _ = BeforeEach(func() {
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider.Reset()

	// ensure any waiters on our clock are allowed to proceed before resetting our clock time
	for fakeClock.HasWaiters() {
		fakeClock.Step(1 * time.Minute)
	}
	fakeClock.SetTime(time.Now())
	state.PodSchedulingDecisionSeconds.Reset()
	pscheduling.DefaultTerminationGracePeriod = nil
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	cloudProvider.Reset()
	cluster.Reset()
	pscheduling.IgnoredPodCount.Set(0, nil)
})

var _ = Describe("Provisioning", func() {
	Context("Batcher", func() {
		It("should provision single pod if no other pod is received within the batch idle duration", func() {
			pod := test.UnschedulablePod()
			ExpectApplied(ctx, env.Client, test.NodePool(), pod)
			prov.Trigger(pod.UID)

			ExpectParallelized(
				func() {
					Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second*10).Should(BeTrue())
					fakeClock.Step(time.Second * 11)
				},
				func() {
					result := ExpectSingletonReconciled(ctx, prov)
					Expect(result.RequeueAfter).ToNot(BeNil())
				},
			)
		})
		It("should not extend the timeout if we receive the same pod within the batch idle duration", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				BatchMaxDuration:  lo.ToPtr(10 * time.Second),
				BatchIdleDuration: lo.ToPtr(5 * time.Second),
			}))
			pod := test.UnschedulablePod()
			ExpectApplied(ctx, env.Client, test.NodePool(), pod)

			wg := sync.WaitGroup{}
			wg.Add(1)
			Expect(fakeClock.HasWaiters()).To(BeFalse())
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				// Have a waiter on the first trigger and trigger the batcher
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())
				prov.Trigger(pod.UID)

				time.Sleep(time.Second) // give the process time to make it to the next batching section

				// Fall-through to the second batching section
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())

				// Step the clock by 3 seconds which is within the batch idle duration of 5s and then add the same pod again.
				fakeClock.Step(3 * time.Second)
				// We expect to have waiters on the fakeClock since this is still within the batch idle duration of 5s.
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())
				prov.Trigger(pod.UID)

				time.Sleep(time.Second) // give the process time to iterate on the batching section

				// Step the clock again by 3s to just cross the batch idle duration. We should be able to get out of the
				// provisioning loop because the same pod will not cause the idle duration to reset.
				fakeClock.Step(3 * time.Second)
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeFalse())
			}()
			ExpectSingletonReconciled(ctx, prov)
			wg.Wait()
		})
		It("should extend the timeout if we receive a new pod within the batch idle duration", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				BatchMaxDuration:  lo.ToPtr(10 * time.Second),
				BatchIdleDuration: lo.ToPtr(5 * time.Second),
			}))
			pod := test.UnschedulablePod()
			ExpectApplied(ctx, env.Client, test.NodePool(), pod)

			pod2 := test.UnschedulablePod()
			ExpectApplied(ctx, env.Client, pod2)

			wg := sync.WaitGroup{}
			wg.Add(1)
			Expect(fakeClock.HasWaiters()).To(BeFalse())
			go func() {
				defer GinkgoRecover()
				defer wg.Done()

				// Have a waiter on the first trigger and trigger the batcher
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())
				prov.Trigger(pod.UID)

				time.Sleep(time.Second) // give the process time to make it to the next batching section

				// Fall-through to the second batching section
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())

				// Step the clock by 3 seconds which is within the batch idle duration of 5s and then add a new pod
				fakeClock.Step(3 * time.Second)
				// We expect to have waiters on the fakeClock since this is still within the batch idle duration of 5s.
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())
				prov.Trigger(pod2.UID)

				time.Sleep(time.Second) // give the process time to iterate on the batching section

				// Step the clock by 3s as we expect provisioning to not happen until another 5s because the
				// batch idle duration was reset due to a new pod being added.
				fakeClock.Step(3 * time.Second)
				Consistently(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeTrue())
				// Stepping the clock again by 3s. We should be able to get out of the
				// provisioning loop at this point (since we have exceeded the idle duration)
				fakeClock.Step(3 * time.Second)
				Eventually(func() bool { return fakeClock.HasWaiters() }, time.Second).Should(BeFalse())
			}()
			ExpectSingletonReconciled(ctx, prov)
			wg.Wait()
		})
	})
	It("should provision nodes", func() {
		ExpectApplied(ctx, env.Client, test.NodePool())
		pod := test.UnschedulablePod()
		cluster.AckPods(pod)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		ExpectScheduled(ctx, env.Client, pod)
		ExpectMetricHistogramSampleCountValue("karpenter_pods_scheduling_decision_duration_seconds", 1, nil)
	})
	It("Should provision nodes for multiple pods", func() {
		ExpectApplied(ctx, env.Client, test.NodePool())
		pods := test.UnschedulablePods(test.PodOptions{}, 100)
		cluster.AckPods(pods...)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).ToNot(Equal(0))
		ExpectPodsScheduled(ctx, env.Client, pods...)
		ExpectMetricHistogramSampleCountValue("karpenter_pods_scheduling_decision_duration_seconds", 100, nil)
	})
	It("should expect nodeclaim terminationGracePeriod to be the global value when the nodepool terminationGracePeriod is not set", func() {
		nodePool := test.NodePool()
		pscheduling.DefaultTerminationGracePeriod = &metav1.Duration{Duration: 98 * time.Hour}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		nodeClaims := &v1.NodeClaimList{}
		Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
		Expect(len(nodeClaims.Items)).To(Equal(1))
		Expect(nodeClaims.Items[0].Spec.TerminationGracePeriod.Duration).To(BeNumerically("==", 98*time.Hour))
	})
	It("should expect nodeclaim terminationGracePeriod to be the nil when the nodepool terminationGracePeriod and global terminationGracePeriod is not set", func() {
		nodePool := test.NodePool()
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		nodeClaims := &v1.NodeClaimList{}
		Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
		Expect(len(nodeClaims.Items)).To(Equal(1))
		Expect(nodeClaims.Items[0].Spec.TerminationGracePeriod).To(BeNil())
	})
	It("should respect terminationGracePeriod value set on the nodePool over global terminationGracePeriod", func() {
		nodePool := test.NodePool()
		nodePool.Spec.Template.Spec.TerminationGracePeriod = &metav1.Duration{Duration: 223 * time.Hour}
		pscheduling.DefaultTerminationGracePeriod = &metav1.Duration{Duration: 47 * time.Hour}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		nodeClaims := &v1.NodeClaimList{}
		Expect(env.Client.List(ctx, nodeClaims)).To(Succeed())
		Expect(len(nodeClaims.Items)).To(Equal(1))
		Expect(nodeClaims.Items[0].Spec.TerminationGracePeriod.Duration).To(BeNumerically("==", 223*time.Hour))
	})
	It("should ignore NodePools that are deleting", func() {
		nodePool := test.NodePool()
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectDeletionTimestampSet(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(0))
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should mark pod as unschedulable if there are no valid nodepools", func() {
		nodePool := test.NodePool()
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectDeletionTimestampSet(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		cluster.AckPods(pod)
		nn := client.ObjectKeyFromObject(pod)
		// Provisioning should fail here since there are no valid nodePools to schedule the pod
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
		Expect(cluster.PodSchedulingSuccessTime(nn).IsZero()).To(BeTrue())
		Expect(cluster.PodSchedulingDecisionTime(nn).IsZero()).To(BeFalse())
		ExpectMetricHistogramSampleCountValue("karpenter_pods_scheduling_decision_duration_seconds", 1, nil)
	})
	It("should mark podHealthyNodePoolScheduledTime if it is scheduled against a nodePool with NodeRegistrationHealthy=true", func() {
		nodePool := test.NodePool()
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeClassReady)
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		ExpectScheduled(ctx, env.Client, pod)
		Expect(cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod)).IsZero()).To(BeFalse())
	})
	It("should not mark podHealthyNodePoolScheduledTime if it is scheduled against a nodePool with NodeRegistrationHealthy=False", func() {
		nodePool := test.NodePool()
		nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "unhealthy", "unhealthy")
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeClassReady)
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		ExpectScheduled(ctx, env.Client, pod)
		Expect(cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod)).IsZero()).To(BeTrue())
	})
	It("should provision nodes for pods with supported node selectors", func() {
		nodePool := test.NodePool()
		schedulable := []*corev1.Pod{
			// Constrained by nodepool
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.NodePoolLabelKey: nodePool.Name}}),
			// Constrained by zone
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1"}}),
			// Constrained by instanceType
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "default-instance-type"}}),
			// Constrained by architecture
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelArchStable: "arm64"}}),
			// Constrained by operatingSystem
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelOSStable: string(corev1.Linux)}}),
		}
		unschedulable := []*corev1.Pod{
			// Ignored, matches another nodepool
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.NodePoolLabelKey: "unknown"}}),
			// Ignored, invalid zone
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelTopologyZone: "unknown"}}),
			// Ignored, invalid instance type
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "unknown"}}),
			// Ignored, invalid architecture
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelArchStable: "unknown"}}),
			// Ignored, invalid operating system
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{corev1.LabelOSStable: "unknown"}}),
			// Ignored, invalid capacity type
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.CapacityTypeLabelKey: "unknown"}}),
			// Ignored, label selector does not match
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
		}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, schedulable...)
		for _, pod := range schedulable {
			ExpectScheduled(ctx, env.Client, pod)
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, unschedulable...)
		for _, pod := range unschedulable {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes for pods with supported node affinities", func() {
		nodePool := test.NodePool()
		schedulable := []*corev1.Pod{
			// Constrained by nodepool
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: v1.NodePoolLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{nodePool.Name}}}}),
			// Constrained by zone
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}}),
			// Constrained by instanceType
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"default-instance-type"}}}}),
			// Constrained by architecture
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"arm64"}}}}),
			// Constrained by operatingSystem
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(corev1.Linux)}}}}),
		}
		unschedulable := []*corev1.Pod{
			// Ignored, matches another nodepool
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: v1.NodePoolLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, invalid zone
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, invalid instance type
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, invalid architecture
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, invalid operating system
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, invalid capacity type
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"unknown"}}}}),
			// Ignored, label selector does not match
			test.UnschedulablePod(test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpIn, Values: []string{"bar"}}}}),
		}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, schedulable...)
		for _, pod := range schedulable {
			ExpectScheduled(ctx, env.Client, pod)
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, unschedulable...)
		for _, pod := range unschedulable {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes for accelerators", func() {
		ExpectApplied(ctx, env.Client, test.NodePool())
		pods := []*corev1.Pod{
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Limits: corev1.ResourceList{fake.ResourceGPUVendorA: resource.MustParse("1")}},
			}),
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Limits: corev1.ResourceList{fake.ResourceGPUVendorB: resource.MustParse("1")}},
			}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision multiple nodes when maxPods is set", func() {
		// Kubelet is actually not observed here, the scheduler is relying on the
		// pods resource value which is statically set in the fake cloudprovider
		ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimTemplateSpec{
						Requirements: []v1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      corev1.LabelInstanceTypeStable,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"single-pod-instance-type"},
								},
							},
						},
					},
				},
			},
		}))
		pods := []*corev1.Pod{
			test.UnschedulablePod(), test.UnschedulablePod(), test.UnschedulablePod(),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(3))
		for _, pod := range pods {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should not use a different NodePool hash on the NodeClaim if the NodePool changes during scheduling", func() {
		// This test was added since we were generating the NodeClaim's NodePool hash from a NodePool that was re-retrieved
		// after scheduling had been completed. This could have resulted in the hash not accurately reflecting the actual NodePool
		// state that it was generated from

		nodePool := test.NodePool()
		pod := test.UnschedulablePod()
		hash := nodePool.Hash()
		ExpectApplied(ctx, env.Client, nodePool, pod)

		results, err := prov.Schedule(ctx)
		Expect(err).ToNot(HaveOccurred())

		Expect(results.NewNodeClaims).To(HaveLen(1))
		Expect(results.NewNodeClaims[0].ToNodeClaim().Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, hash))

		nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{
			"new-label": "new-value",
		})
		Expect(nodePool.Hash()).ToNot(Equal(hash))
		ExpectApplied(ctx, env.Client, nodePool) // apply the changed NodePool but expect no change in the hash on the NodeClaim

		nodeClaims, err := prov.CreateNodeClaims(ctx, results.NewNodeClaims)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeClaims).To(HaveLen(1))

		nodeClaim := &v1.NodeClaim{}
		Expect(env.Client.Get(ctx, types.NamespacedName{Name: nodeClaims[0]}, nodeClaim)).To(Succeed())
		Expect(cluster.PodNodeClaimMapping(client.ObjectKeyFromObject(pod))).To(BeEquivalentTo(nodeClaim.Name))

		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, hash))
	})
	It("should schedule all pods on one inflight node when node is in deleting state", func() {
		nodePool := test.NodePool()
		its, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(BeNil())
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: its[0].Name,
				},
				Finalizers: []string{v1.TerminationFinalizer},
			},
		},
		)
		ExpectApplied(ctx, env.Client, node, nodePool)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		// Schedule 3 pods to the node that currently exists
		for i := 0; i < 3; i++ {
			pod := test.UnschedulablePod()
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}

		// Node shouldn't fully delete since it has a finalizer
		Expect(env.Client.Delete(ctx, node)).To(Succeed())
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		// Provision without a binding since some pods will already be bound
		// Should all schedule to the new node, ignoring the old node
		bindings := ExpectProvisionedNoBinding(ctx, env.Client, cluster, cloudProvider, prov, test.UnschedulablePod(), test.UnschedulablePod())
		nodes := &corev1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(2))

		// Scheduler should attempt to schedule all the pods to the new node
		for _, n := range bindings {
			Expect(n.Node.Name).ToNot(Equal(node.Name))
		}
	})
	It("should schedule based on the max resource requests of containers and initContainers with sidecar containers when initcontainer comes first", func() {
		if env.Version.Minor() < 29 {
			Skip("Native Sidecar containers is only on by default starting in K8s version >= 1.29.x")
		}

		ExpectApplied(ctx, env.Client, test.NodePool())

		// Add three instance types, one that's what we want, one that's slightly smaller, one that's slightly bigger.
		// If we miscalculate resources, we'll schedule to the smaller instance type rather than the larger one
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 10)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 4)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 11)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 5)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 12)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 6)),
		})

		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10"), corev1.ResourceMemory: resource.MustParse("4Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10"), corev1.ResourceMemory: resource.MustParse("4Gi")},
					},
				},
				{
					RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
					},
				},
			},
		})

		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("11"),
			corev1.ResourceMemory: resource.MustParse("5Gi"),
		}, node.Status.Capacity)
	})
	It("should schedule based on the max resource requests of containers and initContainers with sidecar containers when sidecar container comes first and init container resources are smaller than container resources", func() {
		if env.Version.Minor() < 29 {
			Skip("Native Sidecar containers is only on by default starting in K8s version >= 1.29.x")
		}

		ExpectApplied(ctx, env.Client, test.NodePool())

		// Add three instance types, one that's what we want, one that's slightly smaller, one that's slightly bigger.
		// If we miscalculate resources, we'll schedule to the smaller instance type rather than the larger one
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 10)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 4)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 11)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 5)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 12)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 6)),
		})

		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
			InitContainers: []corev1.Container{
				{
					RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
					},
				},
				{
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
					},
				},
			},
		})

		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("11"),
			corev1.ResourceMemory: resource.MustParse("5Gi"),
		}, node.Status.Capacity)
	})
	It("should schedule based on the max resource requests of containers and initContainers with sidecar containers when sidecar container comes first and init container resources are bigger than container resources", func() {
		if env.Version.Minor() < 29 {
			Skip("Native Sidecar containers is only on by default starting in K8s version >= 1.29.x")
		}

		ExpectApplied(ctx, env.Client, test.NodePool())

		// Add three instance types, one that's what we want, one that's slightly smaller, one that's slightly bigger.
		// If we miscalculate resources, we'll schedule to the smaller instance type rather than the larger one
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 10)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 4)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 11)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 5)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 12)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 6)),
		})

		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
			InitContainers: []corev1.Container{
				{
					RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4.9"), corev1.ResourceMemory: resource.MustParse("2.9Gi")},
					},
				},
				{
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
					},
				},
			},
		})

		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("11"),
			corev1.ResourceMemory: resource.MustParse("5Gi"),
		}, node.Status.Capacity)
	})
	It("should schedule based on the pod level resources requests", func() {
		if env.Version.Minor() < 34 {
			Skip("Pod level resources is only on by default starting in K8s version >= 1.34.x")
		}

		ExpectApplied(ctx, env.Client, test.NodePool())

		// Add three instance types, one that's what we want, one that's slightly smaller, one that's slightly bigger.
		// If we miscalculate resources, we'll schedule to the smaller instance type rather than the larger one
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 10)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 4)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 11)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 5)),
		})
		cloudProvider.InstanceTypes = AddInstanceResources(cloudProvider.InstanceTypes, corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 12)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", 6)),
		})

		pod := test.UnschedulablePod(test.PodOptions{
			PodResourceRequirements: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10"), corev1.ResourceMemory: resource.MustParse("4Gi")},
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("9.9"), corev1.ResourceMemory: resource.MustParse("3.9Gi")},
			},
			ResourceRequirements: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5"), corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
			InitContainers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("6"), corev1.ResourceMemory: resource.MustParse("2Gi")},
					},
				},
				{
					RestartPolicy: lo.ToPtr(corev1.ContainerRestartPolicyAlways),
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("2Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("2Gi")},
					},
				},
			},
		})

		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		}, node.Status.Capacity)
	})

	Context("Resource Limits", func() {
		It("should not schedule when limits are exceeded", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20")}),
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			cluster.UpdateNodeClaim(test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
				Status: v1.NodeClaimStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100"),
					},
				},
			}))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule if limits would be met", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}),
				},
			}))
			pod := test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						// requires a 2 CPU node, but leaves room for overhead
						corev1.ResourceCPU: resource.MustParse("1.75"),
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// A 2 CPU node can be launched
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should partially schedule if limits would be exceeded", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")}),
				},
			}))

			// prevent these pods from scheduling on the same node
			opts := test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "foo"},
				},
				PodAntiRequirements: []corev1.PodAffinityTerm{
					{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "foo",
							},
						},
					},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1.5"),
					},
				},
			}
			pods := []*corev1.Pod{
				test.UnschedulablePod(opts),
				test.UnschedulablePod(opts),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			scheduledPodCount := 0
			unscheduledPodCount := 0
			pod0 := ExpectPodExists(ctx, env.Client, pods[0].Name, pods[0].Namespace)
			pod1 := ExpectPodExists(ctx, env.Client, pods[1].Name, pods[1].Namespace)
			if pod0.Spec.NodeName == "" {
				unscheduledPodCount++
			} else {
				scheduledPodCount++
			}
			if pod1.Spec.NodeName == "" {
				unscheduledPodCount++
			} else {
				scheduledPodCount++
			}
			Expect(scheduledPodCount).To(Equal(1))
			Expect(unscheduledPodCount).To(Equal(1))
		})
		It("should not schedule if limits would be exceeded", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}),
				},
			}))
			pod := test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("2.1"),
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if limits would be exceeded (GPU)", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourcePods: resource.MustParse("1")}),
				},
			}))
			pod := test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						fake.ResourceGPUVendorA: resource.MustParse("1"),
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// only available instance type has 2 GPUs which would exceed the limit
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule to a nodepool after a scheduling round if limits would be exceeded", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}),
				},
			}))
			pod := test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						// requires a 2 CPU node, but leaves room for overhead
						corev1.ResourceCPU: resource.MustParse("1.75"),
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// A 2 CPU node can be launched
			ExpectScheduled(ctx, env.Client, pod)

			// This pod requests over the existing limit (would add to 3.5 CPUs) so this should fail
			pod = test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						// requires a 2 CPU node, but leaves room for overhead
						corev1.ResourceCPU: resource.MustParse("1.75"),
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
	})
	Context("Daemonsets", func() {
		It("should account for daemonsets", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should account for daemonset hostports", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
					HostPorts:            []int32{8080},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
					HostPorts:            []int32{8080},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// Expect that the host port will be blocked by a compatible daemonset
			ExpectNotScheduled(ctx, env.Client, pod)
			Expect(ExpectNodes(ctx, env.Client)).To(HaveLen(0))
		})
		It("should account for daemonsets (with startup taint)", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							StartupTaints: []corev1.Taint{{Key: "foo.com/taint", Effect: corev1.TaintEffectNoSchedule}},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should not schedule if daemonset overhead is too large", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("10000Gi")}},
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should account for overhead using daemonset pod spec instead of daemonset spec", func() {
			nodePool := test.NodePool()
			// Create a daemonset with large resource requests
			daemonset := test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("4Gi")}},
				}},
			)
			ExpectApplied(ctx, env.Client, nodePool, daemonset)
			// Create the actual daemonSet pod with lower resource requests and expect to use the pod
			daemonsetPod := test.UnschedulablePod(
				test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "apps/v1",
								Kind:               "DaemonSet",
								Name:               daemonset.Name,
								UID:                daemonset.UID,
								Controller:         lo.ToPtr(true),
								BlockOwnerDeletion: lo.ToPtr(true),
							},
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				})
			ExpectApplied(ctx, env.Client, nodePool, daemonsetPod)
			ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))
			pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				NodeSelector:         map[string]string{v1.NodePoolLabelKey: nodePool.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We expect a smaller instance since the daemonset pod is smaller then daemonset spec
			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should not schedule if resource requests are not defined and limits (requests) are too large", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("10000Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule based on the max resource requests of containers and initContainers", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
					InitContainers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("3Gi")},
								Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")},
							},
						},
					},
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should not schedule if combined max resources are too large for any node", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
					InitContainers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("10000Gi")},
								Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
							},
						},
					},
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if initContainer resources are too large", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					InitContainers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10000"), corev1.ResourceMemory: resource.MustParse("10000Gi")},
							},
						},
					},
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should be able to schedule pods if resource requests and limits are not defined", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should ignore daemonsets without matching tolerations", func() {
			ExpectApplied(ctx, env.Client,
				test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Taints: []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}},
							},
						},
					},
				}),
				test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
					}},
				))
			pod := test.UnschedulablePod(
				test.PodOptions{
					Tolerations:          []corev1.Toleration{{Operator: corev1.TolerationOperator(corev1.NodeSelectorOpExists)}},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("2")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
		})
		It("should ignore daemonsets with an invalid selector", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodeSelector:         map[string]string{"node": "invalid"},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("2")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
		})
		It("should account daemonsets with NotIn operator and unspecified key", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodeRequirements:     []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"bar"}}},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements:     []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should remove daemonset node hostname affinity when considering daemonset schedulability", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
			})
			// When simulating the Daemon pod, we should use the pod representation for resource requests but remove the overridden
			// hostname affinity and replace it with the daemonset representation
			daemonset := test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{
							Key:      "foo",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("4Gi")}},
				}},
			)
			ExpectApplied(ctx, env.Client, nodePool, daemonset)
			daemonsetPod := test.UnschedulablePod(
				test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "apps/v1",
								Kind:               "DaemonSet",
								Name:               daemonset.Name,
								UID:                daemonset.UID,
								Controller:         lo.ToPtr(true),
								BlockOwnerDeletion: lo.ToPtr(true),
							},
						},
					},
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{
							Key:      metav1.ObjectNameField,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"node-name"},
						},
					},
					// We specifically make this different from the actual DaemonSet requests to mock a LimitRange overriding pod
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				})
			ExpectApplied(ctx, env.Client, daemonsetPod)
			ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))

			// Deploy pod
			pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				NodeSelector: map[string]string{
					"foo": "bar",
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, pod)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should consider a daemonset schedulable with multiple node affinity terms", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
			})
			daemonSet := test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			)
			// Create a DaemonSet that has the second term that matches this NodePool
			daemonSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"voo"},
									},
								},
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
			}
			ExpectApplied(ctx, env.Client, daemonSet, nodePool)
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should consider a daemonset schedulable with an incompatible node affinity preference", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodePreferences: []corev1.NodeSelectorRequirement{
						{
							Key:      "node",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"invalid"},
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should consider a daemonset schedulable with a PreferNoSchedule taint", func() {
			ExpectApplied(ctx, env.Client,
				test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Taints: []corev1.Taint{
									{
										Key:    "test",
										Effect: corev1.TaintEffectPreferNoSchedule,
									},
								},
							},
						},
					},
				}),
				test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("2Gi")}},
					}},
				),
			)
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)

			// We have multiple instance types that we can launch, one with 2Gi and one with 4Gi
			// If we launch with 2Gi, this means the Daemon pod was not respected
			// If we launch with 4Gi, this means the Daemon pod was respected
			allocatable := instanceTypeMap[node.Labels[corev1.LabelInstanceTypeStable]].Capacity
			Expect(*allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
	})
	Context("Annotations", func() {
		It("should annotate nodes", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Annotations: map[string]string{v1.DoNotDisruptAnnotationKey: "true"},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Annotations).To(HaveKeyWithValue(v1.DoNotDisruptAnnotationKey, "true"))
		})
	})
	Context("Labels", func() {
		It("should label nodes", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{"test-key-1": "test-value-1"},
						},
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-2", Operator: corev1.NodeSelectorOpIn, Values: []string{"test-value-2"}}},
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-3", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"test-value-3"}}},
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-4", Operator: corev1.NodeSelectorOpLt, Values: []string{"4"}}},
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-5", Operator: corev1.NodeSelectorOpGt, Values: []string{"5"}}},
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-6", Operator: corev1.NodeSelectorOpExists}},
								{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "test-key-7", Operator: corev1.NodeSelectorOpDoesNotExist}},
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.NodePoolLabelKey, nodePool.Name))
			Expect(node.Labels).To(HaveKeyWithValue("test-key-1", "test-value-1"))
			Expect(node.Labels).To(HaveKeyWithValue("test-key-2", "test-value-2"))
			Expect(node.Labels).To(And(HaveKey("test-key-3"), Not(HaveValue(Equal("test-value-3")))))
			Expect(node.Labels).To(And(HaveKey("test-key-4"), Not(HaveValue(Equal("test-value-4")))))
			Expect(node.Labels).To(And(HaveKey("test-key-5"), Not(HaveValue(Equal("test-value-5")))))
			Expect(node.Labels).To(HaveKey("test-key-6"))
			Expect(node.Labels).ToNot(HaveKey("test-key-7"))
		})
		It("should label nodes with labels in the LabelDomainExceptions list", func() {
			for domain := range v1.LabelDomainExceptions {
				nodePool := test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							ObjectMeta: v1.ObjectMeta{
								Labels: map[string]string{domain + "/test": "test-value"},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				pod := test.UnschedulablePod(
					test.PodOptions{
						NodeRequirements: []corev1.NodeSelectorRequirement{{Key: domain + "/test", Operator: corev1.NodeSelectorOpIn, Values: []string{"test-value"}}},
					},
				)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(domain+"/test", "test-value"))
			}
		})
		It("should label nodes with labels in the subdomain from LabelDomainExceptions list", func() {
			for domain := range v1.LabelDomainExceptions {
				nodePool := test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							ObjectMeta: v1.ObjectMeta{
								Labels: map[string]string{"subdomain." + domain + "/test": "test-value"},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				pod := test.UnschedulablePod(
					test.PodOptions{
						NodeRequirements: []corev1.NodeSelectorRequirement{{Key: "subdomain." + domain + "/test", Operator: corev1.NodeSelectorOpIn, Values: []string{"test-value"}}},
					},
				)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue("subdomain."+domain+"/test", "test-value"))
			}
		})
	})
	Context("Taints", func() {
		It("should schedule pods that tolerate taints", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Taints: []corev1.Taint{{Key: "nvidia.com/gpu", Value: "true", Effect: corev1.TaintEffectNoSchedule}},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pods := []*corev1.Pod{
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []corev1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpEqual,
							Value:    "true",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []corev1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []corev1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: corev1.TolerationOpExists,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					}}),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})
	})
	Context("NodeClaim Creation", func() {
		It("should create a nodeclaim request with expected requirements", func() {
			nodePool := test.NodePool()
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			ExpectNodeClaimRequirements(cloudProvider.CreateCalls[0],
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   lo.Keys(instanceTypeMap),
				},
				corev1.NodeSelectorRequirement{
					Key:      v1.NodePoolLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{nodePool.Name},
				},
			)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request with additional expected requirements", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      "custom-requirement-key",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"value"},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      "custom-requirement-key2",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"value"},
									},
								},
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			ExpectNodeClaimRequirements(cloudProvider.CreateCalls[0],
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   lo.Keys(instanceTypeMap),
				},
				corev1.NodeSelectorRequirement{
					Key:      v1.NodePoolLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{nodePool.Name},
				},
				corev1.NodeSelectorRequirement{
					Key:      "custom-requirement-key",
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"value"},
				},
				corev1.NodeSelectorRequirement{
					Key:      "custom-requirement-key2",
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"value"},
				},
			)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request restricting instance types on architecture", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelArchStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"arm64"},
									},
								},
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))

			// Expect a more restricted set of instance types
			ExpectNodeClaimRequirements(cloudProvider.CreateCalls[0],
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"arm64"},
				},
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"arm-instance-type"},
				},
			)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request restricting instance types on operating system", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"ios"},
									},
								},
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))

			// Expect a more restricted set of instance types
			ExpectNodeClaimRequirements(cloudProvider.CreateCalls[0],
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"ios"},
				},
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"arm-instance-type"},
				},
			)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request restricting instance types based on pod resource requests", func() {
			nodePool := test.NodePool()
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						fake.ResourceGPUVendorA: resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						fake.ResourceGPUVendorA: resource.MustParse("1"),
					},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))

			// Expect a more restricted set of instance types
			ExpectNodeClaimRequirements(cloudProvider.CreateCalls[0],
				corev1.NodeSelectorRequirement{
					Key:      corev1.LabelInstanceTypeStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"gpu-vendor-instance-type"},
				},
			)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request with the correct owner reference", func() {
			nodePool := test.NodePool()
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			Expect(cloudProvider.CreateCalls[0].OwnerReferences).To(ContainElement(
				metav1.OwnerReference{
					APIVersion:         "karpenter.sh/v1",
					Kind:               "NodePool",
					Name:               nodePool.Name,
					UID:                nodePool.UID,
					BlockOwnerDeletion: lo.ToPtr(true),
				},
			))
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim request propagating the nodeClass reference", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							NodeClassRef: &v1.NodeClassReference{
								Group: "karpenter.test.sh",
								Kind:  "TestNodeClass",
								Name:  "test",
							},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			Expect(cloudProvider.CreateCalls[0].Spec.NodeClassRef).To(Equal(
				&v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "TestNodeClass",
					Name:  "test",
				},
			))
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim with resource requests", func() {
			ExpectApplied(ctx, env.Client, test.NodePool())
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:      resource.MustParse("1"),
							corev1.ResourceMemory:   resource.MustParse("1Mi"),
							fake.ResourceGPUVendorA: resource.MustParse("1"),
						},
						Limits: corev1.ResourceList{
							fake.ResourceGPUVendorA: resource.MustParse("1"),
						},
					},
				})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			Expect(cloudProvider.CreateCalls[0].Spec.Resources.Requests).To(HaveLen(4))
			ExpectNodeClaimRequests(cloudProvider.CreateCalls[0], corev1.ResourceList{
				corev1.ResourceCPU:      resource.MustParse("1"),
				corev1.ResourceMemory:   resource.MustParse("1Mi"),
				fake.ResourceGPUVendorA: resource.MustParse("1"),
				corev1.ResourcePods:     resource.MustParse("1"),
			})
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should create a nodeclaim with resource requests with daemon overhead", func() {
			ExpectApplied(ctx, env.Client, test.NodePool(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Mi")}},
				}},
			))
			pod := test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Mi")}},
				},
			)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			Expect(cloudProvider.CreateCalls).To(HaveLen(1))
			ExpectNodeClaimRequests(cloudProvider.CreateCalls[0], corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("2Mi"),
				corev1.ResourcePods:   resource.MustParse("2"),
			})
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("Volume Topology Requirements", func() {
		var storageClass *storagev1.StorageClass
		BeforeEach(func() {
			storageClass = test.StorageClass(test.StorageClassOptions{
				Zones:             []string{"test-zone-2", "test-zone-3"},
				VolumeBindingMode: lo.ToPtr(storagev1.VolumeBindingWaitForFirstConsumer),
			})
		})
		It("should not schedule if invalid pvc", func() {
			ExpectApplied(ctx, env.Client, test.NodePool())
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{"invalid"},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectMetricGaugeValue(pscheduling.IgnoredPodCount, 1, nil)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule with an empty storage class if the pvc is bound", func() {
			storageClass := ""
			volumeName := "test-volume"
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: &storageClass,
				VolumeName:       volumeName,
			})
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: volumeName,
				},
				StorageClassName: storageClass,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim, persistentVolume)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not schedule with an empty storage class if the pvc is not bound", func() {
			storageClass := ""
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule with a missing storage class if the pvc is bound", func() {
			missingStorageClass := "missing-storage-class"
			volumeName := "test-volume"
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: &missingStorageClass,
				VolumeName:       volumeName,
			})
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: volumeName,
				},
				StorageClassName: missingStorageClass,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim, persistentVolume)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not schedule with a missing storage class if the pvc is not bound", func() {
			missingStorageClass := "missing-storage-class"
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: &missingStorageClass,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule valid pods when a pod with an invalid pvc is encountered (pvc)", func() {
			ExpectApplied(ctx, env.Client, test.NodePool())
			invalidPod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{"invalid"},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, invalidPod)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, invalidPod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should schedule valid pods when a pod with an invalid pvc is encountered (storage class)", func() {
			invalidStorageClass := "invalid-storage-class"
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &invalidStorageClass})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim)
			invalidPod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, invalidPod)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, invalidPod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should schedule valid pods when a pod with an invalid pvc is encountered (volume name)", func() {
			invalidVolumeName := "invalid-volume-name"
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: invalidVolumeName})
			ExpectApplied(ctx, env.Client, test.NodePool(), persistentVolumeClaim)
			invalidPod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, invalidPod)
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, invalidPod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should schedule to storage class zones if volume does not exist", func() {
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"},
				}},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-3"))
		})
		It("should schedule to storage class zones if volume does not exist (ephemeral volume)", func() {
			pod := test.UnschedulablePod(test.PodOptions{
				EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{
					{
						StorageClassName: &storageClass.Name,
					},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"},
				}},
			})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s", pod.Name, pod.Spec.Volumes[0].Name),
				},
				StorageClassName: &storageClass.Name,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule if storage class zones are incompatible", func() {
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
				}},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if storage class zones are incompatible (ephemeral volume)", func() {
			pod := test.UnschedulablePod(test.PodOptions{
				EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{
					{
						StorageClassName: &storageClass.Name,
					},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
				}},
			})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s", pod.Name, pod.Spec.Volumes[0].Name),
				},
				StorageClassName: &storageClass.Name,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule to volume zones if volume already bound", func() {
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim, persistentVolume)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-3"))
		})
		It("should schedule to volume zones if volume already bound (ephemeral volume)", func() {
			pod := test.UnschedulablePod(test.PodOptions{
				EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{
					{
						StorageClassName: &storageClass.Name,
					},
				},
			})
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s", pod.Name, pod.Spec.Volumes[0].Name),
				},
				VolumeName:       persistentVolume.Name,
				StorageClassName: &storageClass.Name,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, pod, persistentVolumeClaim, persistentVolume)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-3"))
		})
		DescribeTable("should ignore hostname affinity scheduling when using local path volumes",
			func(volumeOptions test.PersistentVolumeOptions) {
				// StorageClass that references "no-provisioner" and is used for local volume storage
				storageClass = test.StorageClass(test.StorageClassOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: "local-path",
					},
					Provisioner: lo.ToPtr("kubernetes.io/no-provisioner"),
				})
				// Create a PersistentVolume that is using a random node name for its affinity
				persistentVolume := test.PersistentVolume(volumeOptions)
				persistentVolume.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelHostname,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{test.RandomName()},
									},
								},
							},
						},
					},
				}
				persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					VolumeName:       persistentVolume.Name,
					StorageClassName: &storageClass.Name,
				})
				ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim, persistentVolume)
				pod := test.UnschedulablePod(test.PodOptions{
					PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				// Expect that we are still able to schedule this pod to a node, even though we had a hostname affinity on it
				ExpectScheduled(ctx, env.Client, pod)
			},
			Entry("when using local volumes", test.PersistentVolumeOptions{UseLocal: true}),
			Entry("when using hostpath volumes", test.PersistentVolumeOptions{UseHostPath: true}),
		)
		DescribeTable("should ignore hostname affinity scheduling when using local path volumes (ephemeral volume)",
			func(volumeOptions test.PersistentVolumeOptions) {
				// StorageClass that references "no-provisioner" and is used for local volume storage
				storageClass = test.StorageClass(test.StorageClassOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: "local-path",
					},
					Provisioner: lo.ToPtr("kubernetes.io/no-provisioner"),
				})
				pod := test.UnschedulablePod(test.PodOptions{
					EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{
						{
							StorageClassName: &storageClass.Name,
						},
					},
				})
				persistentVolume := test.PersistentVolume(volumeOptions)
				persistentVolume.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelHostname,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{test.RandomName()},
									},
								},
							},
						},
					},
				}
				persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", pod.Name, pod.Spec.Volumes[0].Name),
					},
					VolumeName:       persistentVolume.Name,
					StorageClassName: &storageClass.Name,
				})
				ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, pod, persistentVolumeClaim, persistentVolume)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
			},
			Entry("when using local volumes", test.PersistentVolumeOptions{UseLocal: true}),
			Entry("when using hostpath volumes", test.PersistentVolumeOptions{UseHostPath: true}),
		)
		It("should not ignore hostname affinity when using non-local path volumes", func() {
			// This PersistentVolume is going to use a standard CSI volume for provisioning
			persistentVolume := test.PersistentVolume()
			persistentVolume.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      corev1.LabelHostname,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{test.RandomName()},
								},
							},
						},
					},
				},
			}
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim, persistentVolume)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			// Expect that this pod can't schedule because we have a hostname affinity, and we don't currently have a pod that we can schedule to
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if volume zones are incompatible", func() {
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim, persistentVolume)
			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
				}},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if volume zones are incompatible (ephemeral volume)", func() {
			pod := test.UnschedulablePod(test.PodOptions{
				EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{
					{
						StorageClassName: &storageClass.Name,
					},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{{
					Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
				}},
			})
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s", pod.Name, pod.Spec.Volumes[0].Name),
				},
				VolumeName:       persistentVolume.Name,
				StorageClassName: &storageClass.Name,
			})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, pod, persistentVolumeClaim, persistentVolume)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not relax an added volume topology zone node-selector away", func() {
			persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
			persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
			ExpectApplied(ctx, env.Client, test.NodePool(), storageClass, persistentVolumeClaim, persistentVolume)

			pod := test.UnschedulablePod(test.PodOptions{
				PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      "example.com/label",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"unsupported"},
					},
				},
			})

			// Add the second capacity type that is OR'd with the first. Previously we only added the volume topology requirement
			// to a single node selector term which would sometimes get relaxed away.  Now we add it to all of them to AND
			// it with each existing term.
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      v1.CapacityTypeLabelKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{v1.CapacityTypeOnDemand},
						},
					},
				})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-3"))
		})
	})
	Context("Preferential Fallback", func() {
		Context("Required", func() {
			It("should not relax the final term", func() {
				pod := test.UnschedulablePod()
				pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}}, // Should not be relaxed
					}},
				}}}}
				// Don't relax
				nodePool := test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Requirements: []v1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should relax multiple terms", func() {
				pod := test.UnschedulablePod()
				pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
					}},
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}, // OR operator, never get to this one
					}},
				}}}}
				// Success
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-1"))
			})
		})
		Context("Preferences", func() {
			It("should relax all node affinity terms", func() {
				pod := test.UnschedulablePod()
				pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
						}},
					},
					{
						Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
						}},
					},
				}}}
				// Success
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
			})
			It("should relax to use lighter weights", func() {
				pod := test.UnschedulablePod()
				pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 100, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-3"}},
						}},
					},
					{
						Weight: 50, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2"}},
						}},
					},
					{
						Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{ // OR operator, never get to this one
							{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
						}},
					},
				}}}
				// Success
				nodePool := test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Requirements: []v1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(corev1.LabelTopologyZone, "test-zone-2"))
			})
			It("should tolerate PreferNoSchedule taint only after trying to relax Affinity terms", func() {
				pod := test.UnschedulablePod()
				pod.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
						}},
					},
					{
						Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelInstanceTypeStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"invalid"}},
						}},
					},
				}}}
				// Success
				nodePool := test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Taints: []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectPreferNoSchedule}},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, nodePool)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Spec.Taints).To(ContainElement(corev1.Taint{Key: "foo", Value: "bar", Effect: corev1.TaintEffectPreferNoSchedule}))
			})
			DescribeTable("should ignore node preferredDuringSchedulingIgnoredDuringExecution affinity", func(topologyKey string) {
				pod1 := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "foo",
						},
					},
					PodPreferences: []corev1.WeightedPodAffinityTerm{},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				})
				nodePreferencePod := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "baz",
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					// Create a nodePreference that can't be satisfied
					NodePreferences: []corev1.NodeSelectorRequirement{
						{
							Key:      topologyKey,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"value-1"},
						},
					},
				})
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, nodePreferencePod)

				pod1Node := ExpectScheduled(ctx, env.Client, pod1)
				podNodePreferenceNode := ExpectScheduled(ctx, env.Client, nodePreferencePod)

				Expect(pod1Node.Name).To(Equal(podNodePreferenceNode.Name))
			},
				Entry(corev1.LabelTopologyZone, corev1.LabelTopologyZone),
				Entry(corev1.LabelHostname, corev1.LabelHostname),
			)
		})
		Context("Ignore Preferences", func() {
			BeforeEach(func() {
				ctx = options.ToContext(ctx, test.Options(test.OptionsFields{PreferencePolicy: lo.ToPtr(options.PreferencePolicyIgnore)}))
			})
			It("should ignore node affinity preferences", func() {
				zone1Pod := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "foo",
						},
					},
					NodeSelector: map[string]string{
						corev1.LabelTopologyZone: "test-zone-1",
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				})
				zone2Pods := test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "bar",
						},
					},
					NodeSelector: map[string]string{
						corev1.LabelTopologyZone: "test-zone-2",
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3"),
						},
					},
				}, 2)
				// Create an affinity pod that has a preference to schedule in the zone2Pod
				// Without considering the preference, we would schedule the pod to the zone1Pod's node
				// because the number of pods on that node should be lower than the zone2Pod's
				nodeAffinityPod := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "baz",
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					NodePreferences: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelTopologyZone,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"test-zone-2"},
						},
					},
				})
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pods[0], zone2Pods[1], nodeAffinityPod)

				zone1Node := ExpectScheduled(ctx, env.Client, zone1Pod)
				zone2Node := ExpectScheduled(ctx, env.Client, zone2Pods[0])
				ExpectScheduled(ctx, env.Client, zone2Pods[1])
				nodeAffinityPodNode := ExpectScheduled(ctx, env.Client, nodeAffinityPod)

				Expect(nodeAffinityPodNode.Name).To(Equal(zone1Node.Name))
				Expect(nodeAffinityPodNode.Name).ToNot(Equal(zone2Node.Name))
			})
			DescribeTable("should ignore topologySpreadConstraint preferences", func(topologyKey string) {
				opts := test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "foo"},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       topologyKey,
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "foo",
								},
							},
						},
					},
				}
				pods := test.UnschedulablePods(opts, 5)
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

				// If we were respecting preferences, we would add these pods on separate nodes
				// Because we are ignoring them, we will schedule them all on the same node
				nodeNames := sets.New[string]()
				for _, p := range pods {
					nodeNames.Insert(ExpectScheduled(ctx, env.Client, p).Name)
				}
				Expect(nodeNames).To(HaveLen(1))
			},
				Entry(corev1.LabelTopologyZone, corev1.LabelTopologyZone),
				Entry(corev1.LabelTopologyZone, corev1.LabelHostname),
			)
			DescribeTable("should ignore pod anti-affinity preferences", func(topologyKey string) {
				opts := test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "foo"},
					},
					PodAntiPreferences: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 1,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: topologyKey,
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "foo",
									},
								},
							},
						},
					},
				}
				pods := test.UnschedulablePods(opts, 5)
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

				// If we were respecting preferences, we would add these pods on separate nodes
				// Because we are ignoring them, we will schedule them all on the same node
				nodeNames := sets.New[string]()
				for _, p := range pods {
					nodeNames.Insert(ExpectScheduled(ctx, env.Client, p).Name)
				}
				Expect(nodeNames).To(HaveLen(1))
			},
				Entry(corev1.LabelTopologyZone, corev1.LabelTopologyZone),
				Entry(corev1.LabelTopologyZone, corev1.LabelHostname),
			)
			DescribeTable("should ignore pod affinity preferences", func(topologyKey string) {
				zone1Pod := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "foo",
						},
					},
					NodeSelector: map[string]string{
						corev1.LabelTopologyZone: "test-zone-1",
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				})
				zone2Pods := test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "bar",
						},
					},
					NodeSelector: map[string]string{
						corev1.LabelTopologyZone: "test-zone-2",
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("3"),
						},
					},
				}, 2)
				// Create an affinity pod that has a preference to schedule in the zone2Pod
				// Without considering the preference, we would schedule the pod to the zone1Pod's node
				// because the number of pods on that node should be lower than the zone2Pod's
				affinityPod := test.UnschedulablePod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "baz",
						},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					PodPreferences: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 1,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: topologyKey,
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "bar",
									},
								},
							},
						},
					},
				})
				ExpectApplied(ctx, env.Client, test.NodePool())
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pods[0], zone2Pods[1], affinityPod)

				zone1Node := ExpectScheduled(ctx, env.Client, zone1Pod)
				zone2Node := ExpectScheduled(ctx, env.Client, zone2Pods[0])
				ExpectScheduled(ctx, env.Client, zone2Pods[0])
				affinityPodNode := ExpectScheduled(ctx, env.Client, affinityPod)

				Expect(affinityPodNode.Name).To(Equal(zone1Node.Name))
				Expect(affinityPodNode.Name).ToNot(Equal(zone2Node.Name))
			},
				Entry(corev1.LabelTopologyZone, corev1.LabelTopologyZone),
				Entry(corev1.LabelTopologyZone, corev1.LabelHostname),
			)
		})
	})
	Context("Multiple NodePools", func() {
		It("should schedule to an explicitly selected NodePool", func() {
			nodePool := test.NodePool()
			ExpectApplied(ctx, env.Client, nodePool, test.NodePool())
			pod := test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.NodePoolLabelKey: nodePool.Name}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(nodePool.Name))
		})
		It("should schedule to a NodePool by labels", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{"foo": "bar"},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, test.NodePool())
			pod := test.UnschedulablePod(test.PodOptions{NodeSelector: nodePool.Spec.Template.Labels})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(nodePool.Name))
		})
		It("should not match NodePool with PreferNoSchedule taint when other NodePool match", func() {
			nodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Taints: []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectPreferNoSchedule}},
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, test.NodePool())
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels[v1.NodePoolLabelKey]).ToNot(Equal(nodePool.Name))
		})

		Context("Weighted NodePools", func() {
			It("should schedule to the nodepool with the highest priority always", func() {
				nodePools := []client.Object{
					test.NodePool(),
					test.NodePool(v1.NodePool{Spec: v1.NodePoolSpec{Weight: lo.ToPtr(int32(20))}}),
					test.NodePool(v1.NodePool{Spec: v1.NodePoolSpec{Weight: lo.ToPtr(int32(100))}}),
				}
				ExpectApplied(ctx, env.Client, nodePools...)
				pods := []*corev1.Pod{
					test.UnschedulablePod(), test.UnschedulablePod(), test.UnschedulablePod(),
				}
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
				for _, pod := range pods {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(nodePools[2].GetName()))
				}
			})
			It("should schedule to explicitly selected nodepool even if other nodePools are higher priority", func() {
				targetedNodePool := test.NodePool()
				nodePools := []client.Object{
					targetedNodePool,
					test.NodePool(v1.NodePool{Spec: v1.NodePoolSpec{Weight: lo.ToPtr(int32(20))}}),
					test.NodePool(v1.NodePool{Spec: v1.NodePoolSpec{Weight: lo.ToPtr(int32(100))}}),
				}
				ExpectApplied(ctx, env.Client, nodePools...)
				pod := test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.NodePoolLabelKey: targetedNodePool.Name}})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(targetedNodePool.Name))
			})
		})
	})

	Context("StaticNodePool", func() {
		It("should not create NodeClaims for StaticNodePool", func() {
			ExpectApplied(ctx, env.Client, test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas: lo.ToPtr(int64(2)),
				}},
			))
			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})

		It("should still provision dynamic NodeClaims when there are both Static and Dynamic NodePools", func() {
			// Should contain both static and dynamic node pool
			targetedNodePool := test.NodePool()
			staticNodePool := test.StaticNodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Replicas: lo.ToPtr(int64(1)),
				}})
			ExpectApplied(ctx, env.Client, targetedNodePool, staticNodePool)

			pod := test.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels[v1.NodePoolLabelKey]).ToNot(Equal(staticNodePool.Name))
			Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(targetedNodePool.Name))
		})
	})

	Context("MinValuesPolicy", func() {
		AfterEach(func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyStrict)}))
		})

		Context("with instance type requirements", func() {
			var defaultNodePool *v1.NodePool

			BeforeEach(func() {
				// Create a nodepool with instance type minValues requirement
				defaultNodePool = test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Weight: lo.ToPtr(int32(100)),
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Requirements: []v1.NodeSelectorRequirementWithMinValues{
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      corev1.LabelInstanceTypeStable,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"instance-type-1", "instance-type-2", "instance-type-3"},
										},
										MinValues: lo.ToPtr(3),
									},
								},
							},
						},
					},
				})
			})

			Context("with MinValuesPolicy set to Strict", func() {
				BeforeEach(func() {
					ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyStrict)}))
				})

				It("should not schedule when minValues requirement is not met", func() {
					// Create only two instance types, which doesn't meet the minValues=3 requirement
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
								Price:        0.52,
							},
						},
					}

					opts2 := fake.InstanceTypeOptions{
						Name:             "instance-type-2",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2-spot"}),
								Price:        0.52,
							},
						},
					}

					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1), fake.NewInstanceType(opts2))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, defaultNodePool)
					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectNotScheduled(ctx, env.Client, pod)
				})
			})

			Context("with MinValuesPolicy set to BestEffort", func() {
				BeforeEach(func() {
					ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyBestEffort)}))
				})

				It("should schedule even when minValues requirement is not met", func() {
					// Create only two instance types, which doesn't meet the minValues=3 requirement
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
								Price:        0.52,
							},
						},
					}

					opts2 := fake.InstanceTypeOptions{
						Name:             "instance-type-2",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2-spot"}),
								Price:        0.52,
							},
						},
					}

					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1), fake.NewInstanceType(opts2))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, defaultNodePool)
					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeClaim := cloudProvider.CreateCalls[0]
					Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("instance-type-1"))
					Expect(node.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey]).To(Equal("true"))

					ExpectMetricCounterValue(metrics.NodeClaimsCreatedTotal, 1, map[string]string{
						metrics.ReasonLabel:           metrics.ProvisionedReason,
						metrics.NodePoolLabel:         nodeClaim.Labels[v1.NodePoolLabelKey],
						metrics.MinValuesRelaxedLabel: "true",
					})
					Expect(nodeClaim.Spec.Requirements).To(ContainElements(
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelInstanceTypeStable,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"instance-type-1", "instance-type-2"},
							},
							MinValues: lo.ToPtr(2),
						}))
				})

				It("should relax minValues before falling back to other nodepools", func() {
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
								Price:        0.52,
							},
						},
					}

					opts2 := fake.InstanceTypeOptions{
						Name:             "instance-type-2",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2-spot"}),
								Price:        0.52,
							},
						},
					}

					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1), fake.NewInstanceType(opts2))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, defaultNodePool)

					nodePoolWithNoMinValues := test.NodePool(v1.NodePool{
						Spec: v1.NodePoolSpec{
							Weight: lo.ToPtr(int32(10)),
							Template: v1.NodeClaimTemplate{
								Spec: v1.NodeClaimTemplateSpec{
									Requirements: []v1.NodeSelectorRequirementWithMinValues{
										{
											NodeSelectorRequirement: corev1.NodeSelectorRequirement{
												Key:      corev1.LabelInstanceTypeStable,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"instance-type-1", "instance-type-2", "instance-type-3"},
											},
										},
									},
								},
							},
						},
					})

					ExpectApplied(ctx, env.Client, nodePoolWithNoMinValues)

					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeClaim := cloudProvider.CreateCalls[0]
					Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("instance-type-1"))
					Expect(node.Annotations).To(HaveKey(v1.NodeClaimMinValuesRelaxedAnnotationKey))
					Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(defaultNodePool.Name))
					Expect(nodeClaim.Spec.Requirements).To(ContainElements(
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelInstanceTypeStable,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"instance-type-1", "instance-type-2"},
							},
							MinValues: lo.ToPtr(2),
						}))
				})

				It("should choose nodepool with higher weight when relaxing minValues", func() {
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
								Price:        0.52,
							},
						},
					}

					opts2 := fake.InstanceTypeOptions{
						Name:             "instance-type-2",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Offerings: []*cloudprovider.Offering{
							{
								Available:    true,
								Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2-spot"}),
								Price:        0.52,
							},
						},
					}

					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1), fake.NewInstanceType(opts2))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, defaultNodePool)

					lowerWeightNodePool := test.NodePool(v1.NodePool{
						Spec: v1.NodePoolSpec{
							Weight: lo.ToPtr(int32(10)),
							Template: v1.NodeClaimTemplate{
								Spec: v1.NodeClaimTemplateSpec{
									Requirements: []v1.NodeSelectorRequirementWithMinValues{
										{
											NodeSelectorRequirement: corev1.NodeSelectorRequirement{
												Key:      corev1.LabelInstanceTypeStable,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"instance-type-1", "instance-type-2", "instance-type-3"},
											},
											MinValues: lo.ToPtr(3),
										},
									},
								},
							},
						},
					})

					ExpectApplied(ctx, env.Client, lowerWeightNodePool)

					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeClaim := cloudProvider.CreateCalls[0]
					Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("instance-type-1"))
					Expect(node.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey]).To(Equal("true"))
					// Ensure that the nodepool with higher weight was chosen
					Expect(node.Labels[v1.NodePoolLabelKey]).To(Equal(defaultNodePool.Name))

					ExpectMetricCounterValue(metrics.NodeClaimsCreatedTotal, 1, map[string]string{
						metrics.ReasonLabel:           metrics.ProvisionedReason,
						metrics.NodePoolLabel:         nodeClaim.Labels[v1.NodePoolLabelKey],
						metrics.MinValuesRelaxedLabel: "true",
					})
					Expect(nodeClaim.Spec.Requirements).To(ContainElements(
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelInstanceTypeStable,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"instance-type-1", "instance-type-2"},
							},
							MinValues: lo.ToPtr(2),
						}))
				})
			})
		})

		Context("with zone requirements", func() {
			var nodePool *v1.NodePool

			BeforeEach(func() {
				// Create a nodepool with zone minValues requirement
				nodePool = test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Requirements: []v1.NodeSelectorRequirementWithMinValues{
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      corev1.LabelTopologyZone,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"test-zone-1", "test-zone-2", "test-zone-3"},
										},
										MinValues: lo.ToPtr(3),
									},
								},
							},
						},
					},
				})
			})

			Context("with MinValuesPolicy set to Strict", func() {
				BeforeEach(func() {
					ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyStrict)}))
				})

				It("should not schedule when zone minValues requirement is not met", func() {
					// Create instance types with only two zones, which doesn't meet the minValues=3 requirement
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					}
					opts1.Offerings = []*cloudprovider.Offering{
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1"}),
							Price:        0.52,
						},
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2"}),
							Price:        0.54,
						},
					}
					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, nodePool)
					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectNotScheduled(ctx, env.Client, pod)
				})
			})

			Context("with MinValuesPolicy set to BestEffort", func() {
				BeforeEach(func() {
					ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyBestEffort)}))
				})

				It("should schedule even when zone minValues requirement is not met", func() {
					// Create instance types with only two zones, which doesn't meet the minValues=3 requirement
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					}
					opts1.Offerings = []*cloudprovider.Offering{
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1"}),
							Price:        0.52,
						},
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2"}),
							Price:        0.54,
						},
					}
					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, nodePool)
					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeClaim := cloudProvider.CreateCalls[0]
					Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("instance-type-1"))
					Expect(node.Labels[corev1.LabelTopologyZone]).To(Or(Equal("test-zone-1"), Equal("test-zone-2")))
					Expect(node.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey]).To(Equal("true"))

					ExpectMetricCounterValue(metrics.NodeClaimsCreatedTotal, 1, map[string]string{
						metrics.ReasonLabel:           metrics.ProvisionedReason,
						metrics.NodePoolLabel:         nodeClaim.Labels[v1.NodePoolLabelKey],
						metrics.MinValuesRelaxedLabel: "true",
					})
					Expect(nodeClaim.Spec.Requirements).To(ContainElements(
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelTopologyZone,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"test-zone-1", "test-zone-2", "test-zone-3"},
							},
							MinValues: lo.ToPtr(2),
						}))
				})
			})
		})

		Context("with both instance type and zone requirements", func() {
			var defaultNodePool *v1.NodePool

			BeforeEach(func() {
				// Create a nodepool with instance type minValues requirement
				defaultNodePool = test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Weight: lo.ToPtr(int32(100)),
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Requirements: []v1.NodeSelectorRequirementWithMinValues{
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      corev1.LabelInstanceTypeStable,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"instance-type-1", "instance-type-2", "instance-type-3"},
										},
										MinValues: lo.ToPtr(3),
									},
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      corev1.LabelTopologyZone,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"test-zone-1", "test-zone-2", "test-zone-3"},
										},
										MinValues: lo.ToPtr(3),
									},
								},
							},
						},
					},
				})
			})

			Context("with MinValuesPolicy set to BestEffort", func() {
				BeforeEach(func() {
					ctx = options.ToContext(ctx, test.Options(test.OptionsFields{MinValuesPolicy: lo.ToPtr(options.MinValuesPolicyBestEffort)}))
				})

				It("should schedule when minValues requirement is not met", func() {
					var instanceTypes []*cloudprovider.InstanceType
					opts1 := fake.InstanceTypeOptions{
						Name:             "instance-type-1",
						Architecture:     v1.ArchitectureArm64,
						OperatingSystems: sets.New(string(corev1.Linux)),
						Resources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					}
					opts1.Offerings = []*cloudprovider.Offering{
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1"}),
							Price:        0.52,
						},
						{
							Available:    true,
							Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-2"}),
							Price:        0.54,
						},
					}
					instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
					cloudProvider.InstanceTypes = instanceTypes

					ExpectApplied(ctx, env.Client, defaultNodePool)
					pod := test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.9"),
							corev1.ResourceMemory: resource.MustParse("0.9Gi")},
						},
					})

					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeClaim := cloudProvider.CreateCalls[0]
					Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("instance-type-1"))
					Expect(node.Labels[corev1.LabelTopologyZone]).To(Or(Equal("test-zone-1"), Equal("test-zone-2")))
					Expect(node.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey]).To(Equal("true"))
					Expect(nodeClaim.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey]).To(Equal("true"))
					Expect(nodeClaim.Spec.Requirements).To(ContainElements(
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelInstanceTypeStable,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"instance-type-1"},
							},
							MinValues: lo.ToPtr(1),
						},
						v1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      corev1.LabelTopologyZone,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"test-zone-1", "test-zone-2", "test-zone-3"},
							},
							MinValues: lo.ToPtr(2),
						}))

					ExpectMetricCounterValue(metrics.NodeClaimsCreatedTotal, 1, map[string]string{
						metrics.ReasonLabel:           metrics.ProvisionedReason,
						metrics.NodePoolLabel:         nodeClaim.Labels[v1.NodePoolLabelKey],
						metrics.MinValuesRelaxedLabel: "true",
					})
				})
			})
		})
	})
})

func ExpectNodeClaimRequirements(nodeClaim *v1.NodeClaim, requirements ...corev1.NodeSelectorRequirement) {
	GinkgoHelper()
	for _, requirement := range requirements {
		req, ok := lo.Find(nodeClaim.Spec.Requirements, func(r v1.NodeSelectorRequirementWithMinValues) bool {
			return r.Key == requirement.Key && r.Operator == requirement.Operator
		})
		Expect(ok).To(BeTrue())

		have := sets.New(req.Values...)
		expected := sets.New(requirement.Values...)
		Expect(have.Len()).To(Equal(expected.Len()))
		Expect(have.Intersection(expected).Len()).To(Equal(expected.Len()))
	}
}

func ExpectNodeClaimRequests(nodeClaim *v1.NodeClaim, resources corev1.ResourceList) {
	GinkgoHelper()
	for name, value := range resources {
		v := nodeClaim.Spec.Resources.Requests[name]
		Expect(v.AsApproximateFloat64()).To(BeNumerically("~", value.AsApproximateFloat64(), 10))
	}
}

func AddInstanceResources(instanceTypes []*cloudprovider.InstanceType, resources corev1.ResourceList) []*cloudprovider.InstanceType {
	opts := fake.InstanceTypeOptions{
		Name:             "example",
		Architecture:     "arch",
		Resources:        resources,
		OperatingSystems: sets.New(string(corev1.Linux)),
	}
	price := fake.PriceFromResources(opts.Resources)
	opts.Offerings = []*cloudprovider.Offering{
		{
			Available: true,
			Requirements: scheduling.NewLabelRequirements(map[string]string{
				v1.CapacityTypeLabelKey:  v1.CapacityTypeSpot,
				corev1.LabelTopologyZone: "test-zone-1",
			}),
			Price: price,
		},
	}

	instanceTypes = append(instanceTypes, fake.NewInstanceType(opts))

	return instanceTypes
}
