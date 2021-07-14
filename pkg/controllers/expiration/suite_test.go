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

package expiration_test

import (
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/controllers/expiration"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Expiration")
}

var controller *expiration.Controller
var env = test.NewEnvironment(func(e *test.Environment) {
	controller = expiration.NewController(e.Client)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Reconciliation", func() {
	var provisioner *v1alpha3.Provisioner

	BeforeEach(func() {
		provisioner = &v1alpha3.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: v1alpha3.ProvisionerSpec{
				Cluster:                v1alpha3.Cluster{Name: ptr.String("test-cluster"), Endpoint: "http://test-cluster", CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
				TTLSecondsUntilExpired: ptr.Int64(30),
			},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})
	It("should ignore nodes without TTLSecondsUntilExpired", func() {
		node := test.Node(test.NodeOptions{
			Labels: map[string]string{
				v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
			},
		})
		provisioner.Spec.TTLSecondsUntilExpired = nil
		ExpectCreated(env.Client, provisioner, node)
		ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(node))

		node = ExpectNodeExists(env.Client, node.Name)
		Expect(node.DeletionTimestamp.IsZero()).To(BeTrue())
	})
	It("should ignore nodes without a provisioner", func() {
		node := test.Node(test.NodeOptions{})
		ExpectCreated(env.Client, provisioner, node)
		ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(node))

		node = ExpectNodeExists(env.Client, node.Name)
		Expect(node.DeletionTimestamp.IsZero()).To(BeTrue())
	})
	It("should not terminate nodes before expiry", func() {
		node := test.Node(test.NodeOptions{
			Labels: map[string]string{
				v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
			},
		})
		ExpectCreated(env.Client, provisioner, node)
		ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(node))

		node = ExpectNodeExists(env.Client, node.Name)
		Expect(node.DeletionTimestamp.IsZero()).To(BeTrue())
	})
	It("should terminate nodes after expiry", func() {
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(0)
		node := test.Node(test.NodeOptions{
			Labels: map[string]string{
				v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
			},
		})
		ExpectCreated(env.Client, provisioner, node)
		ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(node))

		ExpectNotFound(env.Client, node)
	})
})
