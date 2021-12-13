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

package selection_test

import (
	"context"
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var provisioner *v1alpha5.Provisioner
var provisioners *provisioning.Controller
var selectionController *selection.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Scheduling")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		provisioners = provisioning.NewController(ctx, e.Client, corev1.NewForConfigOrDie(e.Config), cloudProvider)
		selectionController = selection.NewController(e.Client, provisioners)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	provisioner = &v1alpha5.Provisioner{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha5.DefaultProvisioner.Name},
		Spec:       v1alpha5.ProvisionerSpec{},
	}
	provisioner.SetDefaults(ctx)
})

var _ = AfterEach(func() {
	ExpectProvisioningCleanedUp(ctx, env.Client, provisioners)
})

var _ = Describe("Multiple Provisioners", func() {
	It("should schedule to an explicitly selected provisioner", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "provisioner2"
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner2.Name}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
	It("should schedule to a provisioner by labels", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "provisioner2"
		provisioner2.Spec.Labels = map[string]string{"foo": "bar"}
		provisioner.Spec.Labels = map[string]string{"foo": "baz"}
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
	It("should prioritize provisioners alphabetically if multiple match", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "aaaaaaaaa"
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
})
