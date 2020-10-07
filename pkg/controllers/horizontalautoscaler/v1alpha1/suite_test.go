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
	"fmt"
	"net/http"
	"testing"
	"time"

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler"
	"github.com/ellistarn/karpenter/pkg/controllers"
	scalablenodegroupv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/test/environment"
	. "github.com/ellistarn/karpenter/pkg/test/expectations"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Horizontal Autoscaler Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var fakeServer *ghttp.Server

func injectFakeServer(environment *environment.Local) {
	fakeServer = environment.Server
}

func injectHorizontalAutoscalerController(environment *environment.Local) {
	scale, err := scale.NewForConfig(
		environment.Manager.GetConfig(),
		environment.Manager.GetRESTMapper(),
		dynamic.LegacyAPIPathResolverFunc,
		scale.NewDiscoveryScaleKindResolver(discovery.NewDiscoveryClientForConfigOrDie(environment.Manager.GetConfig())),
	)
	Expect(err).ToNot(HaveOccurred(), "Failed to create scale client")
	prometheusClient, err := api.NewClient(api.Config{Address: environment.Server.URL()})
	Expect(err).ToNot(HaveOccurred(), "Unable to create prometheus client")
	Expect(controllers.Register(environment.Manager, &Controller{
		Client: environment.Manager.GetClient(),
		AutoscalerFactory: autoscaler.Factory{
			MetricsClientFactory: clients.Factory{
				PrometheusClient: prometheusv1.NewAPI(prometheusClient),
			},
			Mapper:          environment.Manager.GetRESTMapper(),
			ScaleNamespacer: scale,
		},
	}), "Failed to register controller")
	Expect(controllers.Register(environment.Manager, &scalablenodegroupv1alpha1.Controller{})).To(Succeed(), "Failed to register controller")
}

var env environment.Environment = environment.NewLocal(injectFakeServer, injectHorizontalAutoscalerController)

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Test Samples", func() {
	var ns *environment.Namespace
	var ha *v1alpha1.HorizontalAutoscaler
	var sng *v1alpha1.ScalableNodeGroup

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		ha = &v1alpha1.HorizontalAutoscaler{}
		sng = &v1alpha1.ScalableNodeGroup{}
	})

	AfterEach(func() {
		fakeServer.Reset()
	})

	Context("Capacity Reservations", func() {
		It("should scale to average utilization target, metric=85, target=60, replicas=5, want=8", func() {
			Expect(ns.ParseResources("docs/samples/reserved-capacity/resources.yaml", ha, sng)).To(Succeed())
			sng.Spec.Replicas = ptr.Int32(5)
			MockMetricValue(fakeServer, .85)

			ExpectEventuallyCreated(ns.Client, sng)
			ExpectEventuallyCreated(ns.Client, ha)
			ExpectEventuallyHappy(ns.Client, ha)
			Expect(ha.Status.DesiredReplicas).To(BeEquivalentTo(8), log.Pretty(ha))
			ExpectEventuallyDeleted(ns.Client, ha)
		})
	})

	Context("Queue Length", func() {
		It("should scale to average value target, metric=41, target=4, want=11", func() {
			Expect(ns.ParseResources("docs/samples/queue-length/resources.yaml", ha, sng)).To(Succeed())
			sng.Spec.Replicas = ptr.Int32(1)
			MockMetricValue(fakeServer, 41)

			ExpectEventuallyCreated(ns.Client, sng)
			ExpectEventuallyCreated(ns.Client, ha)
			ExpectEventuallyHappy(ns.Client, ha)
			Expect(ha.Status.DesiredReplicas).To(BeEquivalentTo(11), log.Pretty(ha))
			ExpectEventuallyDeleted(ns.Client, ha)
		})
	})
})

func MockMetricValue(server *ghttp.Server, value float64) {
	response := fmt.Sprintf(
		`{"status":"success", "data": {"resultType":"vector","result":[{"metric":{},"value":[%d, "%f"]}]}}`,
		time.Now().Second(),
		value,
	)
	fakeServer.RouteToHandler("POST", "/api/v1/query", ghttp.RespondWith(http.StatusOK, response))
}
