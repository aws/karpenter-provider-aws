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

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics/producers"
	"github.com/ellistarn/karpenter/pkg/test"
	"github.com/ellistarn/karpenter/pkg/test/environment"
	. "github.com/ellistarn/karpenter/pkg/test/expectations"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Metrics Producer Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	e.Manager.Register(&Controller{ProducerFactory: &producers.Factory{Client: e.Manager.GetClient()}})
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Test Samples", func() {
	var ns *environment.Namespace
	var mp *v1alpha1.MetricsProducer

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		mp = &v1alpha1.MetricsProducer{}
	})

	Context("Capacity Reservations", func() {
		It("should produce reservation metrics for 7/48 cores, 77/384 memory, 4/150 pods", func() {
			Expect(ns.ParseResources("docs/samples/reserved-capacity/resources.yaml", mp)).To(Succeed())
			mp.Spec.ReservedCapacity.NodeSelector = map[string]string{"k8s.io/nodegroup": ns.Name}

			capacity := v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("16"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
				v1.ResourcePods:   resource.MustParse("50"),
			}

			nodes := []client.Object{
				test.NodeWith(test.NodeOptions{Labels: mp.Spec.ReservedCapacity.NodeSelector, Capacity: capacity}),
				test.NodeWith(test.NodeOptions{Labels: mp.Spec.ReservedCapacity.NodeSelector, Capacity: capacity}),
				test.NodeWith(test.NodeOptions{Labels: map[string]string{"unknown": "label"}, Capacity: capacity}),
				test.NodeWith(test.NodeOptions{Labels: mp.Spec.ReservedCapacity.NodeSelector, Capacity: capacity}),
				test.NodeWith(test.NodeOptions{Labels: mp.Spec.ReservedCapacity.NodeSelector, Capacity: capacity, ReadyStatus: v1.ConditionFalse}),
				test.NodeWith(test.NodeOptions{Labels: mp.Spec.ReservedCapacity.NodeSelector, Capacity: capacity, Unschedulable: true}),
			}

			pods := []client.Object{
				// node[0] 6/16 cores, 76/128 gig allocated
				test.Pod(nodes[0].GetName(), ns.Name, v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}),
				test.Pod(nodes[0].GetName(), ns.Name, v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("25Gi")}),
				test.Pod(nodes[0].GetName(), ns.Name, v1.ResourceList{v1.ResourceCPU: resource.MustParse("3"), v1.ResourceMemory: resource.MustParse("50Gi")}),
				// node[1] 1/16 cores, 76/128 gig allocated
				test.Pod(nodes[1].GetName(), ns.Name, v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}),
				// node[2] is ignored
				test.Pod(nodes[2].GetName(), ns.Name, v1.ResourceList{v1.ResourceCPU: resource.MustParse("99"), v1.ResourceMemory: resource.MustParse("99Gi")}),
				// node[3] is unallocated
				// node[4] isn't ready
				// node[5] isn't schedulable
			}

			ExpectCreated(ns.Client, nodes...)
			ExpectCreated(ns.Client, pods...)
			ExpectCreated(ns.Client, mp)

			ExpectEventuallyHappy(ns.Client, mp)
			Expect(mp.Status.ReservedCapacity[v1.ResourceCPU]).To(BeEquivalentTo("14%, 7/48"))
			Expect(mp.Status.ReservedCapacity[v1.ResourceMemory]).To(BeEquivalentTo("20%, 82678120448/412316860416"))
			Expect(mp.Status.ReservedCapacity[v1.ResourcePods]).To(BeEquivalentTo("2%, 4/150"))

			ExpectDeleted(ns.Client, mp)
			ExpectDeleted(ns.Client, nodes...)
			ExpectDeleted(ns.Client, pods...)
		})
		It("should produce reservation metrics for an empty node group", func() {
			Expect(ns.ParseResources("docs/samples/reserved-capacity/resources.yaml", mp)).To(Succeed())

			ExpectCreated(ns.Client, mp)

			ExpectEventuallyHappy(ns.Client, mp)
			Expect(mp.Status.ReservedCapacity[v1.ResourceCPU]).To(BeEquivalentTo("NaN%, 0/0"))
			Expect(mp.Status.ReservedCapacity[v1.ResourceMemory]).To(BeEquivalentTo("NaN%, 0/0"))
			Expect(mp.Status.ReservedCapacity[v1.ResourcePods]).To(BeEquivalentTo("NaN%, 0/0"))

			ExpectDeleted(ns.Client, mp)
		})
	})
})
