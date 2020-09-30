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
	"testing"

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"

	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup"
	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/ellistarn/karpenter/pkg/test/environment"
	. "github.com/ellistarn/karpenter/pkg/test/expectations"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"ScalableNodeGroup Suite",
		[]Reporter{printer.NewlineReporter{}})
}

func injectScalableNodeGroupController(environment *environment.Local) {
	controller := &Controller{
		Client:           environment.Manager.GetClient(),
		NodegroupFactory: nodegroup.Factory{},
	}
	Expect(controllers.RegisterController(environment.Manager, controller)).To(Succeed(), "Failed to register controller")
	Expect(controllers.RegisterWebhook(environment.Manager, controller)).To(Succeed(), "Failed to register webhook")
}

var env environment.Environment = environment.NewLocal(injectScalableNodeGroupController)

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Test Samples", func() {
	var ns *environment.Namespace
	var sng *v1alpha1.ScalableNodeGroup

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		sng = &v1alpha1.ScalableNodeGroup{}
	})

	Context("ScalableNodeGroup", func() {
		It("should be created", func() {
			Expect(ns.ParseResources("docs/samples/scalable-node-group/resources.yaml", sng)).To(Succeed())
			sng.Spec.Replicas = ptr.Int32(5)

			ExpectEventuallyCreated(ns.Client, sng)
			ExpectEventuallyDeleted(ns.Client, sng)
		})
	})

})
