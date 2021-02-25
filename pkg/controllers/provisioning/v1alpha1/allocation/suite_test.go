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

package allocation

import (
	"context"
	"testing"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/test/environment"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Provisioner",
		[]Reporter{printer.NewlineReporter{}})
}

var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	e.Manager.Register(&Controller{})
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Provisioner", func() {
	var ns *environment.Namespace
	var p *v1alpha1.Provisioner

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		p = &v1alpha1.Provisioner{}
	})

	Context("Provisioner", func() {
		It("should do something", func() {
			Expect(ns.ParseResources("docs/examples/provisioner/provisioner.yaml", p)).To(Succeed())
			Expect(ns.Create(context.TODO(), p)).To(Succeed())
		})
	})
})
