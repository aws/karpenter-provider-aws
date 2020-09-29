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
	"context"
	"testing"
	"time"

	//v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	//	"github.com/ellistarn/karpenter/pkg/autoscaler"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers"
	//"github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup"
	"github.com/ellistarn/karpenter/pkg/test/environment"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	//"k8s.io/client-go/discovery"
	//"k8s.io/client-go/dynamic"
	//"k8s.io/client-go/scale"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const (
	Timeout = time.Second * 30
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"ScalableNodeGroup Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var env environment.Environment = environment.NewLocal(func(environment *environment.Local) error {
	controller := &Controller{
		Client:           environment.Manager.GetClient(),
		NodegroupFactory: nodegroup.Factory{},
	}
	Expect(controllers.RegisterController(environment.Manager, controller)).To(Succeed(), "Failed to register controller")
	Expect(controllers.RegisterWebhook(environment.Manager, controller)).To(Succeed(), "Failed to register webhook")
	return nil
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Test ScalableNodeGroup Samples", func() {
	var ns *environment.Namespace
	var sng *v1alpha1.ScalableNodeGroup

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		sng = &v1alpha1.ScalableNodeGroup{}
	})

	Context("Minimal ScalableNodeGroup", func() {
		BeforeEach(func() {
			Expect(ns.ParseResource("docs/samples/scalable-node-group/resources.yaml", sng)).To(Succeed())
		})

		It("should should create and delete", func() {
			Expect(ns.Create(context.Background(), sng)).To(Succeed())
			Eventually(func() error {
				return ns.Get(context.Background(), types.NamespacedName{Name: sng.Name, Namespace: sng.Namespace}, sng)
			}, Timeout).Should(Succeed())
			Expect(ns.Delete(context.Background(), sng)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(ns.Get(context.Background(), types.NamespacedName{Name: sng.Name, Namespace: sng.Namespace}, sng))
			}, Timeout).Should(BeTrue())
		})
	})
})
