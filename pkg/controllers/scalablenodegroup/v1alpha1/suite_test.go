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

	v1alpha1 "github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"knative.dev/pkg/ptr"

	"github.com/awslabs/karpenter/pkg/test/environment"
	. "github.com/awslabs/karpenter/pkg/test/expectations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"ScalableNodeGroup",
		[]Reporter{printer.NewlineReporter{}})
}

var fakeCloudProvider = fake.NewFactory(cloudprovider.Options{})

var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	e.Manager.Register(&Controller{CloudProvider: fakeCloudProvider})
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Examples", func() {
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
			Expect(ns.ParseResources("docs/examples/reserved-capacity-utilization.yaml", sng)).To(Succeed())
			sng.Spec.Replicas = ptr.Int32(5)

			ExpectCreated(ns.Client, sng)
			ExpectEventuallyHappy(ns.Client, sng)

			ExpectDeleted(ns.Client, sng)
		})
	})
})
