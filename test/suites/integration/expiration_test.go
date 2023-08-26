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

package integration_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter/pkg/apis/settings"
	awstest "github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("Expiration", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	BeforeEach(func() {
		nodeTemplate = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner = test.Provisioner(test.ProvisionerOptions{
			ProviderRef:            &v1alpha5.MachineTemplateRef{Name: nodeTemplate.Name},
			TTLSecondsUntilExpired: ptr.Int64(30),
		})
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "false",
		})
	})
	It("should expire the node after the TTLSecondsUntilExpired is reached", func() {
		var numPods int32 = 1

		dep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1alpha5.DoNotEvictPodAnnotationKey: "true",
					},
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, nodeTemplate, dep)

		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		// Expect that the Machine will get an expired status condition
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		// Remove the do-not-evict annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be set as unschedulable, which means its actively being deprovisioned
		Eventually(func(g Gomega) {
			n := &v1.Node{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), n)).Should(Succeed())
			g.Expect(n.Spec.Unschedulable).Should(BeTrue())
		}).Should(Succeed())

		// Remove the TTLSecondsUntilExpired to make sure new node isn't deleted
		// This is CRITICAL since it prevents nodes that are immediately spun up from immediately being expired and
		// racing at the end of the E2E test, leaking node resources into subsequent tests
		provisioner.Spec.TTLSecondsUntilExpired = nil
		env.ExpectUpdated(provisioner)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(machine, node)

		env.EventuallyExpectCreatedMachineCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
	})
	It("should replace expired node with a single node and schedule all pods", func() {
		var numPods int32 = 5

		// We should setup a PDB that will only allow a minimum of 1 pod to be pending at a time
		minAvailable := intstr.FromInt(int(numPods) - 1)
		pdb := test.PodDisruptionBudget(test.PDBOptions{
			Labels: map[string]string{
				"app": "large-app",
			},
			MinAvailable: &minAvailable,
		})
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1alpha5.DoNotEvictPodAnnotationKey: "true",
					},
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(provisioner, nodeTemplate, pdb, dep)

		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		// Set the TTLSecondsUntilExpired to get the node deleted
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(60)
		env.ExpectUpdated(provisioner)

		// Expect that the Machine will get an expired status condition
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		// Remove the do-not-evict annotation so that the Nodes are now deprovisionable
		for _, pod := range env.ExpectPodsMatchingSelector(selector) {
			delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
			env.ExpectUpdated(pod)
		}

		// Eventually the node will be set as unschedulable, which means its actively being deprovisioned
		Eventually(func(g Gomega) {
			n := &v1.Node{}
			g.Expect(env.Client.Get(env.Context, types.NamespacedName{Name: node.Name}, n)).Should(Succeed())
			g.Expect(n.Spec.Unschedulable).Should(BeTrue())
		}).Should(Succeed())

		// Remove the TTLSecondsUntilExpired to make sure new node isn't deleted
		// This is CRITICAL since it prevents nodes that are immediately spun up from immediately being expired and
		// racing at the end of the E2E test, leaking node resources into subsequent tests
		provisioner.Spec.TTLSecondsUntilExpired = nil
		env.ExpectUpdated(provisioner)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(machine, node)

		env.EventuallyExpectCreatedMachineCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
	})
	Context("Expiration Failure", func() {
		It("should not continue to expire if a node never registers", func() {
			// launch a new machine
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeTemplate, provisioner)
			env.EventuallyExpectNodeCount("==", 2)

			// Set a configuration that will not register a machine
			parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
				Name: aws.String("/aws/service/ami-amazon-linux-latest/amzn-ami-hvm-x86_64-ebs"),
			})
			Expect(err).ToNot(HaveOccurred())
			nodeTemplate.Spec.AMISelector = map[string]string{"aws::ids": *parameter.Parameter.Value}
			env.ExpectCreatedOrUpdated(nodeTemplate)

			// Should see the machine has expired
			statingMachineState := env.EventuallyExpectCreatedMachineCount("==", int(numPods))
			Eventually(func(g Gomega) {
				for _, machine := range statingMachineState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
					g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To get cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Expire should fail and the original node should be uncordoned
			// TODO: reduce timeouts when deprovisioning waits are factored out
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(cordonedNodes[0]), cordonedNodes[0]))
				g.Expect(cordonedNodes[0].Spec.Unschedulable).To(BeFalse())
			}).WithTimeout(11 * time.Minute).Should(Succeed())

			endMachineState := &v1alpha5.MachineList{}
			Eventually(func(g Gomega) {
				g.Expect(env.Client.List(env, endMachineState, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				g.Expect(len(endMachineState.Items)).To(BeNumerically("==", int(numPods)))
			}).WithTimeout(6 * time.Minute).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(lo.EveryBy(statingMachineState, func(sm *v1alpha5.Machine) bool {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(sm), sm)).To(Succeed())
					return lo.ContainsBy(endMachineState.Items, func(em v1alpha5.Machine) bool {
						g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(&em), &em)).To(Succeed())
						return sm.Name == em.Name
					})
				})).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not continue to expiration if a node registers but never becomes initialized", func() {
			// launch a new machine
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeTemplate, provisioner)
			statingNodeState := env.EventuallyExpectNodeCount("==", int(numPods))

			// Set a configuration that will not initialize a machine
			provisioner.Spec.StartupTaints = []v1.Taint{{Key: "example.com/taint", Effect: v1.TaintEffectPreferNoSchedule}}
			env.ExpectCreatedOrUpdated(provisioner)

			// Should see the machine has expired
			machines := env.EventuallyExpectCreatedMachineCount("==", int(numPods))
			Eventually(func(g Gomega) {
				for _, machine := range machines {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
					g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineExpired).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To be cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Expire should fail and original node should be uncordoned
			// TODO: reduce timeouts when deprovisioning waits are factored out
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(cordonedNodes[0]), cordonedNodes[0]))
				g.Expect(cordonedNodes[0].Spec.Unschedulable).To(BeFalse())
			}).WithTimeout(15 * time.Minute).Should(Succeed())
			endingNodeState := env.EventuallyExpectNodeCount("==", 3)

			Consistently(func(g Gomega) {
				g.Expect(lo.EveryBy(statingNodeState, func(sn *v1.Node) bool {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(sn), sn)).To(Succeed())
					return lo.ContainsBy(endingNodeState, func(en *v1.Node) bool {
						g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(en), en)).To(Succeed())
						return sn.Name == en.Name
					})
				})).To(BeTrue())
			}, "2m").Should(Succeed())
		})
	})
})
