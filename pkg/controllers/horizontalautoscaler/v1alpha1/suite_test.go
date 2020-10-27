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
	"github.com/ellistarn/karpenter/pkg/cloudprovider/mock"
	scalablenodegroupv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/test/environment"
	. "github.com/ellistarn/karpenter/pkg/test/expectations"
	"github.com/ellistarn/karpenter/pkg/utils/log"
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
	metricsClientFactory := clients.NewFactoryOrDie(environment.Server.URL())
	autoscalerFactory := autoscaler.NewFactoryOrDie(metricsClientFactory, environment.Manager.GetRESTMapper(), environment.Config)
	environment.Manager.Register(
		&Controller{AutoscalerFactory: autoscalerFactory},
		&scalablenodegroupv1alpha1.Controller{CloudProvider: &mock.Factory{}},
	)
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
			sng.Spec.Replicas = 5
			ha.Spec.Behavior.ScaleUp = &v1alpha1.ScalingRules{StabilizationWindowSeconds: ptr.Int32(5)}
			MockMetricValue(fakeServer, .85)

			ExpectCreated(ns.Client, sng)
			ExpectCreated(ns.Client, ha)
			ExpectEventuallyHappy(ns.Client, ha)
			Expect(ha.Status.DesiredReplicas).To(BeEquivalentTo(8), log.Pretty(ha))
			ExpectDeleted(ns.Client, ha)
		})
	})

	Context("Queue Length", func() {
		It("should scale to average value target, metric=41, target=4, want=11", func() {
			Expect(ns.ParseResources("docs/samples/queue-length/resources.yaml", ha, sng)).To(Succeed())
			sng.Spec.Replicas = 1
			ha.Spec.Behavior.ScaleUp = &v1alpha1.ScalingRules{StabilizationWindowSeconds: ptr.Int32(5)}
			MockMetricValue(fakeServer, 41)

			ExpectCreated(ns.Client, sng)
			ExpectCreated(ns.Client, ha)
			ExpectEventuallyHappy(ns.Client, ha)
			Expect(ha.Status.DesiredReplicas).To(BeEquivalentTo(11), log.Pretty(ha))
			ExpectDeleted(ns.Client, ha)
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
