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

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler"
	"github.com/ellistarn/karpenter/pkg/controllers"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	Timeout = time.Second * 5
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Horizontal Autoscaler Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var (
	kubernetesClient client.Client
	stopEnvironment  chan struct{}
)

var _ = BeforeSuite(func() {
	kubernetesClient, stopEnvironment = test.Environment(func(manager manager.Manager) error {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(manager.GetConfig())
		Expect(err).ToNot(HaveOccurred(), "Failed to create discovery client")
		scale, err := scale.NewForConfig(
			manager.GetConfig(),
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.Identifier(),
				Kind:       "HorizontalAutoscaler",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
			Spec: v1alpha1.HorizontalAutoscalerSpec{
				ScaleTargetRef: v1alpha1.CrossVersionObjectReference{
					APIVersion: v1alpha1.SchemeGroupVersion.Identifier(),
					Kind:       "ScalableNodeGroup",
					Name:       "foo",
				},
			},
		}

		It("should should create and delete", func() {
			Expect(kubernetesClient.Create(context.Background(), ha)).To(Succeed())
			Eventually(func() error {
				return kubernetesClient.Get(context.Background(), nn, ha)
			}, Timeout).Should(Succeed())
			Expect(kubernetesClient.Delete(context.Background(), ha)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(kubernetesClient.Get(context.Background(), nn, ha))
			}, Timeout).Should(BeTrue())
		})
	})
})
