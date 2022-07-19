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

package pod_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/metrics/pod"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	prometheus "github.com/prometheus/client_model/go"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var controller *pod.Controller
var ctx context.Context
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Metrics/Pod")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		controller = pod.NewController(env.Client)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Pod Metrics", func() {
	It("should update the pod state metrics", func() {
		p := test.Pod()
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(p))

		podState := ExpectMetric("karpenter_pods_state")
		ExpectMetricLabel(podState, "name", p.GetName())
		ExpectMetricLabel(podState, "namespace", p.GetNamespace())
	})
})

func ExpectMetricLabel(mf *prometheus.MetricFamily, name string, value string) {
	found := false
	for _, m := range mf.Metric {
		for _, l := range m.Label {
			if l.GetName() == name {
				Expect(l.GetValue()).To(Equal(value), fmt.Sprintf("expected metrics %s = %s", name, value))
				found = true
			}
		}
	}
	Expect(found).To(BeTrue())
}
