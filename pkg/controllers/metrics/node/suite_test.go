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

package node_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/metrics/node"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var controller *node.Controller
var ctx context.Context
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Metrics/Node")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		controller = node.NewController(env.Client)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = Describe("Node Metrics", func() {
	It("should update the allocatable metric", func() {
		node := test.Node(test.NodeOptions{
			Allocatable: v1.ResourceList{
				v1.ResourcePods:   resource.MustParse("100"),
				v1.ResourceCPU:    resource.MustParse("5000"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
			},
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))

		// metrics should now be tracking the allocatable capacity of our single node
		nodeAllocation := ExpectMetric("karpenter_nodes_allocatable")

		expectedValues := map[string]float64{
			"cpu":    5000.0,
			"pods":   100.0,
			"memory": 32 * 1024 * 1024 * 1024,
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
