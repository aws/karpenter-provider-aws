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

package v1alpha1

import (
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		e.Manager.GetClient(),
		corev1.NewForConfigOrDie(e.Manager.GetConfig()),
		cloudProvider,
	)
	e.Manager.RegisterControllers(controller)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Reallocation", func() {
	var provisioner *v1alpha1.Provisioner

	BeforeEach(func() {
		// Create Provisioner to give some time for the node to reconcile
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
			},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Manager.GetClient())
	})

	Context("Reconciliation", func() {
		It("should terminate nodes marked terminable", func() {
			node := test.NodeWith(test.NodeOptions{
				Labels: map[string]string{
					v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
					v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
					v1alpha1.ProvisionerPhaseLabel:        v1alpha1.ProvisionerTerminablePhase,
				},
				Annotations: map[string]string{
					v1alpha1.ProvisionerTTLKey: time.Now().Add(time.Duration(-100) * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreatedWithStatus(env.Client, node)
			ExpectCreated(env.Client, provisioner)
			ExpectNotFound(env.Client, node)
		})
	})
})
