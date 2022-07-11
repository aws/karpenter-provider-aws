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

package metrics_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	statemetrics "github.com/aws/karpenter/pkg/controllers/metrics/state"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var nodeController *state.NodeController
var podController *state.PodController
var cloudProvider *fake.CloudProvider
var provisioner *v1alpha5.Provisioner
var metricScraper *statemetrics.MetricScraper

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/State")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	cloudProvider = &fake.CloudProvider{InstanceTypes: fake.InstanceTypesAssorted()}
	cluster = state.NewCluster(env.Client, cloudProvider)
	provisioner = test.Provisioner(test.ProvisionerOptions{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	nodeController = state.NewNodeController(env.Client, cluster)
	podController = state.NewPodController(env.Client, cluster)
	metricScraper = statemetrics.NewMetricScraper(ctx, cluster)
	ExpectApplied(ctx, env.Client, provisioner)
})

var _ = AfterEach(func() {
	metricScraper.Terminate()
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Node Metrics", func() {
	It("should update the allocatable metric", func() {
		resources := v1.ResourceList{
			v1.ResourcePods:   resource.MustParse("100"),
			v1.ResourceCPU:    resource.MustParse("5000"),
			v1.ResourceMemory: resource.MustParse("32Gi"),
		}

		node := test.Node(test.NodeOptions{Allocatable: resources})
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		// metrics should now be tracking the allocatable capacity of our single node
		metricScraper.Update()
		nodeAllocation := ExpectMetric("karpenter_nodes_allocatable")

		expectedValues := map[string]float64{
			"cpu":    float64(resources.Cpu().MilliValue()) / float64(1000),
			"pods":   float64(resources.Pods().Value()),
			"memory": float64(resources.Memory().Value()),
		}

		for _, m := range nodeAllocation.Metric {
			for _, l := range m.Label {
				if l.GetName() == "resource_type" {
					Expect(m.GetGauge().GetValue()).To(Equal(expectedValues[l.GetValue()]),
						fmt.Sprintf("%s, %f to equal %f", l.GetValue(), m.GetGauge().GetValue(),
							expectedValues[l.GetValue()]))
				}
			}
		}
	})
})
