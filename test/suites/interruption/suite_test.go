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
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment
var provider *v1alpha1.AWSNodeTemplate

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
	env.BeforeEach()
	env.ExpectQueueExists()
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.ForceCleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Interruption", Label("AWS"), func() {
	It("should terminate the spot instance and spin-up a new node on spot interruption warning", func() {
		By("Creating a single healthy node with a healthy deployment")
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeSpot},
				},
			},
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		ctx, cancel := context.WithCancel(env.Context)
		defer cancel() // In case the test fails, we need this so that the goroutine monitoring the events is closed

		node := env.Monitor.CreatedNodes()[0]
		instanceID := parseProviderID(node.Spec.ProviderID)

		By("Interrupting the spot instance")
		_, events, err := env.InterruptionAPI.Interrupt(env.Context, []string{instanceID}, 0, true)
		Expect(err).ToNot(HaveOccurred())

		// Monitor the events channel
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer fmt.Println("[FIS EVENT MONITOR] Closing event goroutine monitoring")
			for {
				select {
				case event := <-events:
					if strings.Contains(event.Message, "Spot Instance Shutdown sent") {
						Fail("Node didn't terminate before spot instance shutdown was sent")
					}
					fmt.Printf("[FIS EVENT MONITOR] %s\n", event.Message)
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()

		env.EventuallyExpectNotFound(node)
		close(done) // Once the node is gone, we can close the event channel because the test has effectively succeeded
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
	It("should terminate the node at the API server when the EC2 instance is stopped", func() {
		By("Creating a single healthy node with a healthy deployment")
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeOnDemand},
				},
			},
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Stopping the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceStopped(node.Name)                                 // Make a call to the EC2 api to stop the instance
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute) // shorten the timeout since we should react faster
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
	It("should terminate the node at the API server when the EC2 instance is terminated", func() {
		By("Creating a single healthy node with a healthy deployment")
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeOnDemand},
				},
			},
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]

		By("Terminating the EC2 instance without the EKS cluster's knowledge")
		env.ExpectInstanceTerminated(node.Name)                              // Make a call to the EC2 api to stop the instance
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute) // shorten the timeout since we should react faster
		env.EventuallyExpectHealthyPodCount(selector, 1)
	})
	It("should terminate the node when receiving a scheduled change health event", func() {
		By("Creating a single healthy node with a healthy deployment")
		provider = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeOnDemand},
				},
			},
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		numPods := 1
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: ptr.Int64(0),
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		instanceID := parseProviderID(node.Spec.ProviderID)

		By("Creating a scheduled change health event in the SQS message queue")
		env.ExpectMessagesCreated(scheduledChangeMessage(env.Region, "000000000000", instanceID))
		env.EventuallyExpectNotFoundAssertion(node).WithTimeout(time.Minute) // shorten the timeout since we should react faster
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

func parseProviderID(pid string) string {
	r := regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
	matches := r.FindStringSubmatch(pid)
	if matches == nil {
		return ""
	}
	for i, name := range r.SubexpNames() {
		if name == "InstanceID" {
			return matches[i]
		}
	}
	return ""
}
