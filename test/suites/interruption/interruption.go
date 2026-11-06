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

package interruption

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var Env *aws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	return Describe("Interruption", func() {
		BeforeEach(func() {
			Env.Context = options.ToContext(Env.Context, test.Options(test.OptionsFields{
				InterruptionQueue: lo.ToPtr(Env.InterruptionQueue),
			}))
		})
		It("should terminate the spot instance and spin-up a new node on spot interruption warning", func() {
			By("Creating a single healthy node with a healthy deployment")
			NodePool = coretest.ReplaceRequirements(NodePool, karpv1.NodeSelectorRequirementWithMinValues{
				Key:      karpv1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{karpv1.CapacityTypeSpot},
			})
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			Env.ExpectCreated(NodeClass, NodePool, dep)

			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)

			node := Env.Monitor.CreatedNodes()[0]
			instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
			Expect(err).ToNot(HaveOccurred())

			By("interrupting the spot instance")
			exp := Env.ExpectSpotInterruptionExperiment(instanceID)
			DeferCleanup(func() {
				Env.ExpectExperimentTemplateDeleted(*exp.ExperimentTemplateId)
			})

			// We are expecting the node to be terminated before the termination is complete
			By("waiting to receive the interruption and terminate the node")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
			}).WithTimeout(time.Minute).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})
		It("should terminate the interruptible reserved capacity instance and spin-up a new node on reserved capacity interruption warning", func() {
			By("Creating an IODCR and configuring the nodeclass to select on it")
			sourceReservationID := Env.ExpectCapacityReservationCreated(1).ID
			interruptibleReservationID := aws.ExpectInterruptibleCapacityAllocationCreated(Env.Context, Env.EC2API, sourceReservationID, 1)
			DeferCleanup(func() {
				aws.ExpectInterruptibleAndSourceCapacityCanceled(Env.Context, Env.EC2API, sourceReservationID, interruptibleReservationID)
			})
			NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{
				{ID: sourceReservationID}, {ID: interruptibleReservationID},
			}
			NodePool = coretest.ReplaceRequirements(NodePool,
				karpv1.NodeSelectorRequirementWithMinValues{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved},
				},
			)
			Env.ExpectCreated(NodeClass, NodePool)

			By("Creating a node from IODCR")
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			Env.ExpectCreated(dep)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)
			node := Env.Monitor.CreatedNodes()[0]

			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationInterruptible, "true"))
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, interruptibleReservationID))

			By("Interrupting the reserved instance")
			aws.ExpectModifyInterruptibleCapacity(Env.Context, Env.EC2API, sourceReservationID, 0)

			// We are expecting the node to be terminated before the termination is complete
			By("Waiting to receive the interruption and terminate the node")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
			}).WithTimeout(2 * time.Minute).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})
		It("should terminate the node at the API server when the EC2 instance is stopped", func() {
			By("Creating a single healthy node with a healthy deployment")
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			Env.ExpectCreated(NodeClass, NodePool, dep)

			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)

			node := Env.Monitor.CreatedNodes()[0]

			By("Stopping the EC2 instance without the EKS cluster's knowledge")
			Env.ExpectInstanceStopped(node.Name) // Make a call to the EC2 api to stop the instance

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
			}).WithTimeout(time.Minute).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})
		It("should terminate the node at the API server when the EC2 instance is terminated", func() {
			By("Creating a single healthy node with a healthy deployment")
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			Env.ExpectCreated(NodeClass, NodePool, dep)

			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)

			node := Env.Monitor.CreatedNodes()[0]

			By("Terminating the EC2 instance without the EKS cluster's knowledge")
			Env.ExpectInstanceTerminated(node.Name) // Make a call to the EC2 api to stop the instance

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
			}).WithTimeout(time.Minute).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})
		It("should terminate the node when receiving a scheduled change health event", func() {
			By("Creating a single healthy node with a healthy deployment")
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			Env.ExpectCreated(NodeClass, NodePool, dep)

			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)

			node := Env.Monitor.CreatedNodes()[0]
			instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
			Expect(err).ToNot(HaveOccurred())

			By("Creating a scheduled change health event in the SQS message queue")
			Env.ExpectMessagesCreated(scheduledChangeMessage(Env.Region, "000000000000", instanceID))

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
			}).WithTimeout(time.Minute).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})
		It("should repair the node when EC2 reports a failed instance status check", func() {
			By("Disabling the SQS interruption queue so only DescribeInstanceStatus detects the failure")
			Env.ExpectSettingsOverridden(corev1.EnvVar{Name: "INTERRUPTION_QUEUE", Value: ""})
			DeferCleanup(func() {
				Env.ExpectSettingsOverridden(corev1.EnvVar{Name: "INTERRUPTION_QUEUE", Value: Env.InterruptionQueue})
			})

			// Pin to amd64 instances because the kernel panic approach used to trigger
			// EC2 instance status check failures did not work reliably on arm64/Graviton instances.
			NodePool = coretest.ReplaceRequirements(NodePool, karpv1.NodeSelectorRequirementWithMinValues{
				Key:      corev1.LabelArchStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"amd64"},
			})

			By("Creating a single healthy node with a healthy deployment")
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
					// Tolerate the unreachable taint so that Kubernetes does not evict the pod
					// and trigger a cascading replacement loop of new nodes.
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.kubernetes.io/unreachable",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoExecute,
						},
					},
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			// Schedule a kernel panic after 180 seconds. The EC2 instance status checks
			// take ~2-3 minutes to initialize from "initializing" to "passing", which must
			// to happen before the failure.
			NodeClass.Spec.UserData = lo.ToPtr(`#!/usr/bin/env bash
(
  sleep 180
  echo 0 > /proc/sys/kernel/panic
  echo c > /proc/sysrq-trigger
) >>/var/log/trigger-panic.log 2>&1 &`)

			Env.ExpectCreated(NodeClass, NodePool, dep)

			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			Env.ExpectCreatedNodeCount("==", 1)

			node := Env.Monitor.CreatedNodes()[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			instanceID := lo.Must(utils.ParseInstanceID(node.Spec.ProviderID))

			By("Waiting for the instance status controller to report the node healthy")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				condition := nodeutils.GetCondition(node, instancestatus.ConditionTypeEC2StatusReady)
				g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(instancestatus.ReasonNoImpairmentReported))
			}).Should(Succeed())

			By("Waiting for the instance status controller to report the failure on the node")
			// Poll quickly so EC2 still reports the instance when the condition flips, before repair terminates it.
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
				condition := nodeutils.GetCondition(node, instancestatus.ConditionTypeEC2StatusReady)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(instancestatus.ReasonInstanceReachabilityFailed))
				out, err := Env.EC2API.DescribeInstanceStatus(Env.Context, &ec2.DescribeInstanceStatusInput{InstanceIds: []string{instanceID}})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(out.InstanceStatuses).To(HaveLen(1))
				g.Expect(out.InstanceStatuses[0].InstanceStatus.Details).ToNot(BeEmpty())
				g.Expect(condition.LastTransitionTime.Time).To(BeTemporally("==", lo.FromPtr(out.InstanceStatuses[0].InstanceStatus.Details[0].ImpairedSince)))
			}).WithTimeout(30 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

			By("Waiting for node repair to terminate the node")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
				g.Expect(nodeClaim.DeletionTimestamp.IsZero()).To(BeFalse())
				g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDisruptionReason).Reason).To(Equal(string(karpv1.DisruptionReasonUnhealthy)))
			}).WithTimeout(30 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())
			Env.EventuallyExpectNotFound(node)
			Env.EventuallyExpectHealthyPodCount(selector, 1)
		})

	})
}

func scheduledChangeMessage(region, accountID, involvedInstanceID string) scheduledchange.Message {
	return scheduledchange.Message{
		Metadata: messages.Metadata{
			Version:    "0",
			Account:    accountID,
			DetailType: "AWS Health Event",
			ID:         string(uuid.NewUUID()),
			Region:     region,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", region, involvedInstanceID),
			},
			Source: "aws.health",
			Time:   time.Now(),
		},
		Detail: scheduledchange.Detail{
			Service:           "EC2",
			EventTypeCategory: "scheduledChange",
			AffectedEntities: []scheduledchange.AffectedEntity{
				{
					EntityValue: involvedInstanceID,
				},
			},
		},
	}
}
