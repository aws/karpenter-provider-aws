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

package termination

import (
	"context"
	"testing"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/test"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Termination")
}

var controller *Controller
var env = test.NewEnvironment(func(e *test.Environment) {
	cloudProvider := &fake.CloudProvider{}
	registry.RegisterOrDie(cloudProvider)
	controller = NewController(
		e.Client,
		corev1.NewForConfigOrDie(e.Config),
		cloudProvider,
	)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Termination", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconciliation", func() {
		It("should terminate deleted nodes", func() {
			node := test.NodeWith(test.NodeOptions{
				Finalizers: []string{v1alpha2.KarpenterFinalizer},
				Labels: map[string]string{
					v1alpha2.ProvisionerNameLabelKey:      "default",
					v1alpha2.ProvisionerNamespaceLabelKey: "default",
				},
			})
			ExpectCreated(env.Client, node)
			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			ExpectNotFound(env.Client, node)
		})
		It("should not evict pods that tolerate unschedulable taint", func() {
			node := test.NodeWith(test.NodeOptions{
				Finalizers: []string{v1alpha2.KarpenterFinalizer},
				Labels: map[string]string{
					v1alpha2.ProvisionerNameLabelKey:      "default",
					v1alpha2.ProvisionerNamespaceLabelKey: "default",
				},
			})
			ExpectCreated(env.Client, node)
			ExpectCreated(env.Client, test.Pod(test.PodOptions{
				NodeName:    node.Name,
				Tolerations: []v1.Toleration{{Key: v1.TaintNodeUnschedulable, Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule}},
			}))

			Expect(env.Client.Delete(ctx, node)).To(Succeed())
			node = ExpectNodeExists(env.Client, node.Name)
			ExpectReconcileSucceeded(controller, node)
			pods := &v1.PodList{}
			Expect(env.Client.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name})).To(Succeed())
			Expect(pods.Items).To(HaveLen(1))
			ExpectNotFound(env.Client, node)
		})
	})
})
