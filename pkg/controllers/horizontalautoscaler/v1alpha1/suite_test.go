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

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1/autoscaler"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/test"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Horizontal Autoscaler Suite")
}

var (
	kubernetesClient client.Client
	stopEnvironment  chan struct{}
)

var _ = BeforeSuite(func() {
	kubernetesClient, stopEnvironment = test.Environment(func(manager manager.Manager) error {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(controllerruntime.GetConfigOrDie())
		Expect(err).ToNot(HaveOccurred(), "Failed to create discovery client")
		scale, err := scale.NewForConfig(
			controllerruntime.GetConfigOrDie(),
			manager.GetRESTMapper(),
			dynamic.LegacyAPIPathResolverFunc,
			scale.NewDiscoveryScaleKindResolver(discoveryClient),
		)
		Expect(err).ToNot(HaveOccurred(), "Failed to create scale client")
		controller := &Controller{
			Client: manager.GetClient(),
			AutoscalerFactory: autoscaler.Factory{
				MetricsClientFactory: clients.Factory{},
				KubernetesClient:     manager.GetClient(),
				Mapper:               manager.GetRESTMapper(),
				ScaleNamespacer:      scale,
			},
		}
		Expect(controllers.RegisterController(manager, controller)).To(Succeed(), "Failed to register controller")
		Expect(controllers.RegisterWebhook(manager, controller)).To(Succeed(), "Failed to register webhook")
		return nil
	})
}, 60)

var _ = AfterSuite(func() {
	close(stopEnvironment)
})

var _ = Describe("Controller", func() {
	Context("with an empty resource", func() {
		nn := types.NamespacedName{Name: "foo", Namespace: "default"}
		ha := &v1alpha1.HorizontalAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
		}

		It("should should create and delete", func() {
			Expect(kubernetesClient.Create(context.Background(), ha)).To(Succeed())
			Eventually(func() error {
				return kubernetesClient.Get(context.Background(), nn, ha)
			}).Should(Succeed())
			Expect(kubernetesClient.Delete(context.Background(), ha)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(kubernetesClient.Get(context.Background(), nn, ha))
			}).Should(BeTrue())
		})
	})
})
