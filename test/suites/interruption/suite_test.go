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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/ptr"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment
var nodeClass *v1beta1.EC2NodeClass
var nodePool *corev1beta1.NodePool

func TestInterruption(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Beta/Interruption")
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

var _ = Describe("Interruption", Label("AWS"), func() {
	It("should terminate the spot instance and spin-up a new node on spot interruption warning", func() {
		By("Creating a single healthy node with a healthy deployment")
		nodePool = coretest.ReplaceRequirements(nodePool, v1.NodeSelectorRequirement{
			Key:      corev1beta1.CapacityTypeLabelKey,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{corev1beta1.CapacityTypeSpot},
		})
		numPods := 1
		dep := coretest.Deployment(coretest.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: ptr.Int64(0),
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
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Second * 110).Should(Succeed())
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
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Stopping the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceStopped(node.Name)                                                   // Make a call to the EC2 api to stop the instance
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute).Should(Succeed()) // shorten the timeout since we should react faster
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
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Terminating the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceTerminated(node.Name)                                                // Make a call to the EC2 api to stop the instance
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute).Should(Succeed()) // shorten the timeout since we should react faster
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
				TerminationGracePeriodSeconds: ptr.Int64(0),
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
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute).Should(Succeed()) // shorten the timeout since we should react faster
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
