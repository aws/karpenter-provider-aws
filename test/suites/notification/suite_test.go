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

package consolidation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment"
)

var env *environment.Environment

func TestNotification(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = environment.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "Notification")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
})

var _ = AfterEach(func() {
	env.AfterEach()
})

var _ = Describe("Notification", func() {
	FIt("should terminate the spot instance and spin-up a new node on spot interruption warning", func() {
		ctx, cancel := context.WithCancel(env.Context)
		defer cancel() // In case the test fails, we need this so that the goroutine monitoring the events is closed

		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"spot"},
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
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelHostname,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "my-app",
							},
						},
					},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.GetCreatedNodes()[0]
		instanceID := parseProviderID(node.Spec.ProviderID)

		_, events, _ := env.InterruptionAPI.Interrupt(env.Context, []string{instanceID}, 0, true)

		// Monitor the events channel
		done := make(chan struct{})
		go func() {
			defer fmt.Println("Closing event goroutine monitoring")
			select {
			case event := <-events:
				if strings.Contains(event.Message, "Spot Instance Shutdown sent") {
					Fail("Node didn't terminate before spot instance shutdown was sent")
				}
				fmt.Printf("[SPOT INTERRUPTION EVENT] %s\n", event.Message)
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}()

		env.EventuallyExpectNotFound(&node)
		close(done) // Once the node is gone, we can close the event channel because the test has effectively succeeded
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
})

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
