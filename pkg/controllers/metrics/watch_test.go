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
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	prometheusmodel "github.com/prometheus/client_model/go"
	"github.com/samber/lo"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/metrics"
)

// watchTimeout is well under the one minute requeue, so a pass means a watch triggered the reconcile.
const watchTimeout = 10 * time.Second

var _ = Describe("Watches", func() {
	Context("NodeClassChangedPredicate", func() {
		var update func(func(*v1.EC2NodeClass)) bool
		BeforeEach(func() {
			update = func(mutate func(*v1.EC2NodeClass)) bool {
				o := nodeClass.DeepCopy()
				n := nodeClass.DeepCopy()
				mutate(n)
				return metrics.NodeClassChangedPredicate().Update(event.UpdateEvent{ObjectOld: o, ObjectNew: n})
			}
		})
		It("should pass spec changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) { n.Generation++ })).To(BeTrue())
		})
		It("should pass subnet status changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) { n.Status.Subnets = n.Status.Subnets[:1] })).To(BeTrue())
		})
		It("should drop subnet status reorders", func() {
			Expect(len(nodeClass.Status.Subnets)).To(BeNumerically(">", 1))
			Expect(update(func(n *v1.EC2NodeClass) { slices.Reverse(n.Status.Subnets) })).To(BeFalse())
		})
		It("should pass subnet zone changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) { n.Status.Subnets[0].ZoneID = "tstz1-az9" })).To(BeTrue())
		})
		It("should pass AMI status changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) { n.Status.AMIs = append(n.Status.AMIs, v1.AMI{ID: "ami-new"}) })).To(BeTrue())
		})
		It("should pass capacity reservation status changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) {
				n.Status.CapacityReservations = append(n.Status.CapacityReservations, v1.CapacityReservation{ID: "cr-new"})
			})).To(BeTrue())
		})
		It("should drop condition-only status changes", func() {
			Expect(update(func(n *v1.EC2NodeClass) {
				n.StatusConditions().SetFalse(status.ConditionReady, "NotReady", "not ready")
			})).To(BeFalse())
		})
		It("should drop status changes that don't feed offerings", func() {
			Expect(update(func(n *v1.EC2NodeClass) {
				n.Status.SecurityGroups = append(n.Status.SecurityGroups, v1.SecurityGroup{ID: "sg-new"})
				n.Status.InstanceProfile = "new-profile"
			})).To(BeFalse())
		})
	})
	Context("Manager", func() {
		var cancel context.CancelFunc
		var done chan struct{}
		BeforeEach(func() {
			metrics.InstanceTypeOfferingAvailable.Reset()
			metrics.InstanceTypeOfferingPriceEstimate.Reset()
			baseline := reconcileCount()

			mgrCtx := ctx
			mgr, err := controllerruntime.NewManager(env.Config, controllerruntime.Options{
				Scheme:      scheme.Scheme,
				BaseContext: func() context.Context { return mgrCtx },
				Metrics:     metricsserver.Options{BindAddress: "0"},
				// Each spec registers the same controller name on a fresh manager
				Controller: config.Controller{SkipNameValidation: lo.ToPtr(true)},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics.NewController(env.Client, cloudProvider).Register(ctx, mgr)).To(Succeed())

			var startCtx context.Context
			startCtx, cancel = context.WithCancel(ctx)
			done = make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(done)
				Expect(mgr.Start(startCtx)).To(Succeed())
			}()
			// Wait out the singleton's startup reconcile so later metrics can only come from a watch event.
			expectReconcilesSettled(baseline)
			Expect(seriesCount("karpenter_cloudprovider_instance_type_offering_price_estimate")).To(BeZero())
		})
		AfterEach(func() {
			// Stop the manager before the suite AfterEach resets the shared fakes it reads.
			cancel()
			Eventually(done, watchTimeout).Should(BeClosed())
		})
		It("should publish metrics when a NodePool is created", func() {
			// Drain EC2NodeClass events so only the NodePool event can publish.
			baseline := reconcileCount()
			ExpectApplied(ctx, env.Client, nodeClass)
			expectReconcilesSettled(baseline)
			ExpectApplied(ctx, env.Client, nodePool)
			Eventually(func(g Gomega) {
				metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_price_estimate", map[string]string{
					"instance_type": "m5.large",
					"capacity_type": karpv1.CapacityTypeOnDemand,
					"zone":          "test-zone-1a",
				})
				g.Expect(ok).To(BeTrue())
				g.Expect(aws.ToFloat64(metric.GetGauge().Value)).To(BeNumerically(">", 0))
			}, watchTimeout).Should(Succeed())
		})
		It("should update availability when the EC2NodeClass subnets change", func() {
			ExpectApplied(ctx, env.Client, nodeClass, nodePool)
			Eventually(func(g Gomega) {
				g.Expect(availableInZone("test-zone-1b")).To(BeNumerically(">", 0))
			}, watchTimeout).Should(Succeed())

			// Status-only write, like the nodeclass controller's subnet discovery
			stored := &v1.EC2NodeClass{}
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodeClass), stored)).To(Succeed())
			generation := stored.Generation
			stored.Status.Subnets = []v1.Subnet{{ID: "subnet-test1", Zone: "test-zone-1a"}}
			Expect(env.Client.Status().Update(ctx, stored)).To(Succeed())
			Expect(stored.Generation).To(Equal(generation))

			Eventually(func(g Gomega) {
				g.Expect(availableInZone("test-zone-1b")).To(BeZero())
				g.Expect(availableInZone("test-zone-1a")).To(BeNumerically(">", 0))
			}, watchTimeout).Should(Succeed())
		})
	})
})

// expectReconcilesSettled waits for at least one reconcile past the baseline and then for the count to stop moving.
func expectReconcilesSettled(baseline float64) {
	GinkgoHelper()
	Eventually(reconcileCount, watchTimeout).Should(BeNumerically(">", baseline))
	Eventually(func() bool {
		before := reconcileCount()
		time.Sleep(time.Second)
		return reconcileCount() == before
	}, watchTimeout).Should(BeTrue())
}

func reconcileCount() float64 {
	return lo.SumBy(familyMetrics("controller_runtime_reconcile_total"), func(m metricWithLabels) float64 {
		return lo.Ternary(m.labels["controller"] == "cloudprovider.metrics", m.value, 0)
	})
}

func availableInZone(zone string) int {
	return lo.CountBy(familyMetrics("karpenter_cloudprovider_instance_type_offering_available"), func(m metricWithLabels) bool {
		return m.labels["zone"] == zone && m.value > 0
	})
}

func seriesCount(name string) int {
	return len(familyMetrics(name))
}

type metricWithLabels struct {
	labels map[string]string
	value  float64
}

func familyMetrics(name string) []metricWithLabels {
	families, err := crmetrics.Registry.Gather()
	Expect(err).ToNot(HaveOccurred())
	family, ok := lo.Find(families, func(f *prometheusmodel.MetricFamily) bool { return f.GetName() == name })
	if !ok {
		return nil
	}
	return lo.Map(family.Metric, func(m *prometheusmodel.Metric, _ int) metricWithLabels {
		labels := lo.SliceToMap(m.Label, func(l *prometheusmodel.LabelPair) (string, string) { return l.GetName(), l.GetValue() })
		value := lo.Ternary(m.GetCounter() != nil, m.GetCounter().GetValue(), m.GetGauge().GetValue())
		return metricWithLabels{labels: labels, value: value}
	})
}
