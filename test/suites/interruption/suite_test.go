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

package interruption_test

import (
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *aws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestInterruption(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Interruption")
}

var _ = BeforeEach(func() {
	env.Context = options.ToContext(env.Context, test.Options(test.OptionsFields{
		InterruptionQueue: lo.ToPtr(env.InterruptionQueue),
	}))
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Interruption", func() {
	It("should terminate the spot instance and spin-up a new node on spot interruption warning", func() {
		By("Creating a single healthy node with a healthy deployment")
		nodePool = coretest.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
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

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		By("interrupting the spot instance")
		exp := env.ExpectSpotInterruptionExperiment(instanceID)
		DeferCleanup(func() {
			env.ExpectExperimentTemplateDeleted(*exp.ExperimentTemplateId)
		})

		// We are expecting the node to be terminated before the termination is complete
		By("waiting to receive the interruption and terminate the node")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(time.Minute).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
	It("should terminate the interruptible reserved capacity instance and spin-up a new node on reserved capacity interruption warning", func() {
		By("Creating an IODCR and configuring the nodeclass to select on it")
		sourceReservationID, interruptibleReservationID := aws.ExpectInterruptibleCapacityReservationCreated(
			env.Context,
			env.EC2API,
			ec2types.InstanceTypeM5Large,
			env.ZoneInfo[0].Zone,
			1,
			1,
			nil,
		)
		DeferCleanup(func() {
			aws.ExpectInterruptibleAndSourceCapacityCanceled(env.Context, env.EC2API, sourceReservationID, interruptibleReservationID)
		})
		nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{
			{ID: sourceReservationID}, {ID: interruptibleReservationID},
		}
		nodePool = coretest.ReplaceRequirements(nodePool,
			karpv1.NodeSelectorRequirementWithMinValues{
				Key:      karpv1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeReserved},
			},
		)
		env.ExpectCreated(nodeClass, nodePool)

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

		env.ExpectCreated(dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.Monitor.CreatedNodes()[0]

		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationInterruptible, "true"))
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, interruptibleReservationID))

		By("Interrupting the reserved instance")
		aws.ExpectModifyInterruptibleCapacity(env.Context, env.EC2API, sourceReservationID, 0)

		// We are expecting the node to be terminated before the termination is complete
		By("Waiting to receive the interruption and terminate the node")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(time.Minute).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
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

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Stopping the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceStopped(node.Name) // Make a call to the EC2 api to stop the instance

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(time.Minute).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
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

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Terminating the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceTerminated(node.Name) // Make a call to the EC2 api to stop the instance

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(time.Minute).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
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

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		By("Creating a scheduled change health event in the SQS message queue")
		env.ExpectMessagesCreated(scheduledChangeMessage(env.Region, "000000000000", instanceID))

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(time.Minute).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
	It("should terminate the node when receiving an instance status failure", func() {
		numPods := 1
		dep := coretest.Deployment(coretest.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
				// Tolerate the unreachable taint so that Kubernetes does not evict the pod
				// and trigger a cascading replacement loop of new nodes that also lose their
				// network interface before the instance status controller can act.
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

		// Schedule the interface to go down after 90 seconds. This must be above a minute
		// so that the instance status check initializes to healthy before the interface is
		// brought down. EC2 instance status checks use ARP requests to verify instance
		// reachability, so disabling the interface will cause the check to fail.
		nodeClass.Spec.UserData = lo.ToPtr(`#!/usr/bin/env bash
(
  sleep 90
  IFACE=$(ip route show default | awk '{print $5}' | head -n1)
  ip link set dev "$IFACE" down
) >>/var/log/disable-net.log 2>&1 &`)

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
		Expect(err).ToNot(HaveOccurred())

		GinkgoWriter.Printf("[DEBUG] Node created: %s, InstanceID: %s, Time: %s\n", node.Name, instanceID, time.Now().UTC().Format(time.RFC3339))

		// The instance status controller polls DescribeInstanceStatus every 30s.
		// After the interface goes down (~90s), EC2 needs ~1-2 minutes to detect the failure,
		// then the UnhealthyThreshold (120s) must elapse before the controller acts.
		// Total expected time: ~90s + ~120s + ~120s + ~30s polling = ~6 minutes.
		Eventually(func(g Gomega) {
			// Debug: call DescribeInstanceStatus directly to see what EC2 reports
			// First call without IncludeAllInstances (what the controller sees)
			statusOut, statusErr := env.EC2API.DescribeInstanceStatus(env.Context, &ec2.DescribeInstanceStatusInput{
				InstanceIds: []string{instanceID},
			})
			if statusErr != nil {
				GinkgoWriter.Printf("[DEBUG] [%s] DescribeInstanceStatus (filtered) error: %v\n", time.Now().UTC().Format(time.RFC3339), statusErr)
			} else if len(statusOut.InstanceStatuses) == 0 {
				GinkgoWriter.Printf("[DEBUG] [%s] DescribeInstanceStatus (filtered): no results (instance not impaired or not running)\n", time.Now().UTC().Format(time.RFC3339))
			} else {
				for _, s := range statusOut.InstanceStatuses {
					GinkgoWriter.Printf("[DEBUG] [%s] DescribeInstanceStatus (filtered): instance=%s state=%s instanceStatus=%s systemStatus=%s",
						time.Now().UTC().Format(time.RFC3339),
						awssdk.ToString(s.InstanceId),
						s.InstanceState.Name,
						s.InstanceStatus.Status,
						s.SystemStatus.Status,
					)
					if s.InstanceStatus != nil {
						for _, d := range s.InstanceStatus.Details {
							GinkgoWriter.Printf(" instanceDetail=[name=%s status=%s impairedSince=%v]", d.Name, d.Status, d.ImpairedSince)
						}
					}
					if s.SystemStatus != nil {
						for _, d := range s.SystemStatus.Details {
							GinkgoWriter.Printf(" systemDetail=[name=%s status=%s impairedSince=%v]", d.Name, d.Status, d.ImpairedSince)
						}
					}
					for _, e := range s.Events {
						GinkgoWriter.Printf(" event=[code=%s notBefore=%v notAfter=%v]", e.Code, e.NotBefore, e.NotAfter)
					}
					GinkgoWriter.Println()
				}
			}

			// Second call with IncludeAllInstances to see the full picture
			allStatusOut, allStatusErr := env.EC2API.DescribeInstanceStatus(env.Context, &ec2.DescribeInstanceStatusInput{
				InstanceIds:         []string{instanceID},
				IncludeAllInstances: awssdk.Bool(true),
			})
			if allStatusErr != nil {
				GinkgoWriter.Printf("[DEBUG] [%s] DescribeInstanceStatus (all): error: %v\n", time.Now().UTC().Format(time.RFC3339), allStatusErr)
			} else {
				for _, s := range allStatusOut.InstanceStatuses {
					GinkgoWriter.Printf("[DEBUG] [%s] DescribeInstanceStatus (all): instance=%s state=%s instanceStatus=%s systemStatus=%s",
						time.Now().UTC().Format(time.RFC3339),
						awssdk.ToString(s.InstanceId),
						s.InstanceState.Name,
						s.InstanceStatus.Status,
						s.SystemStatus.Status,
					)
					if s.InstanceStatus != nil {
						for _, d := range s.InstanceStatus.Details {
							GinkgoWriter.Printf(" instanceDetail=[name=%s status=%s impairedSince=%v]", d.Name, d.Status, d.ImpairedSince)
						}
					}
					if s.SystemStatus != nil {
						for _, d := range s.SystemStatus.Details {
							GinkgoWriter.Printf(" systemDetail=[name=%s status=%s impairedSince=%v]", d.Name, d.Status, d.ImpairedSince)
						}
					}
					GinkgoWriter.Println()
				}
			}

			// Debug: check node status
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), node)).To(Succeed())
			GinkgoWriter.Printf("[DEBUG] [%s] Node %s: DeletionTimestamp=%v, Ready=%s\n",
				time.Now().UTC().Format(time.RFC3339),
				node.Name,
				node.DeletionTimestamp,
				getNodeReadyCondition(node),
			)
			g.Expect(!node.DeletionTimestamp.IsZero()).To(BeTrue())
		}).WithTimeout(15 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())
		env.EventuallyExpectNotFound(node)
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})

})

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

func getNodeReadyCondition(node *corev1.Node) string {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return string(c.Status)
		}
	}
	return "Unknown"
}
