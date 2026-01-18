/*
Copyright The Kubernetes Authors.

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
	"testing"

	opmetrics "github.com/awslabs/operatorpkg/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var testGauge1, testGauge2 opmetrics.GaugeMetric

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store")
}

var _ = BeforeSuite(func() {
	testGauge1 = opmetrics.NewPrometheusGauge(crmetrics.Registry, prometheus.GaugeOpts{Name: "test_gauge_1"}, []string{"label_1", "label_2"})
	testGauge2 = opmetrics.NewPrometheusGauge(crmetrics.Registry, prometheus.GaugeOpts{Name: "test_gauge_2"}, []string{"label_1", "label_2"})
})

var _ = Describe("Store", func() {
	var ms *metrics.Store
	var key client.ObjectKey

	BeforeEach(func() {
		ms = metrics.NewStore()
		key = client.ObjectKey{Namespace: "default", Name: "test"}
	})
	Context("Update", func() {
		It("should create metrics when calling update", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			// Expect to find the metrics with the correct values
			m, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", storeMetrics[0].Value))

			m, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", storeMetrics[1].Value))

		})
		It("should delete metrics when calling update", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			newStoreMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test_2",
						"label_2": "test_2",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test_2",
						"label_2": "test_2",
					},
				},
			}
			ms.Update(key.String(), newStoreMetrics)

			// Expect to not find the old metrics
			_, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())

			// Expect to find the new metrics with the correct values
			m, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test_2", "label_2": "test_2"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", newStoreMetrics[0].Value))
			m, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test_2", "label_2": "test_2"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", newStoreMetrics[1].Value))
		})
		It("should consider metrics equal with the same labels", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			// Expect to find the metrics with the correct values
			m, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", storeMetrics[0].Value))

			m, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", storeMetrics[1].Value))

			// Flip around the labels in the map
			newStoreMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       4.5,
					Labels: map[string]string{
						"label_2": "test",
						"label_1": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       6.9,
					Labels: map[string]string{
						"label_2": "test",
						"label_1": "test",
					},
				},
			}
			ms.Update(key.String(), newStoreMetrics)

			// Expect to find the metrics with the new values
			m, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", newStoreMetrics[0].Value))

			m, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			Expect(lo.FromPtr(m.Gauge.Value)).To(BeNumerically("==", newStoreMetrics[1].Value))
		})
	})
	Context("Delete", func() {
		It("should delete metrics by key", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			// Expect to find the metrics
			_, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())

			ms.Delete(key.String())

			// Expect the metrics to be gone
			_, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
		})
		It("should delete the metrics if key didn't previously exist", func() {
			ms.Delete(key.String())
		})
	})
	Context("ReplaceAll", func() {
		It("should replace all metrics", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			// Expect to find the metrics
			_, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())

			key2 := client.ObjectKey{Namespace: "default", Name: "test2"}
			key3 := client.ObjectKey{Namespace: "default", Name: "test3"}

			newStore := map[string][]*metrics.StoreMetric{
				key2.String(): {
					{
						GaugeMetric: testGauge1,
						Value:       3.65,
						Labels: map[string]string{
							"label_1": "test2",
							"label_2": "test2",
						},
					},
					{
						GaugeMetric: testGauge2,
						Value:       4.3,
						Labels: map[string]string{
							"label_1": "test2",
							"label_2": "test2",
						},
					},
				},
				key3.String(): {
					{
						GaugeMetric: testGauge1,
						Value:       2.1,
						Labels: map[string]string{
							"label_1": "test3",
							"label_2": "test3",
						},
					},
					{
						GaugeMetric: testGauge2,
						Value:       8.9,
						Labels: map[string]string{
							"label_1": "test3",
							"label_2": "test3",
						},
					},
				},
			}
			ms.ReplaceAll(newStore)

			// Expect to not find the old metrics
			_, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())

			// Expect to find the new metrics for test2
			_, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test2", "label_2": "test2"})
			Expect(ok).To(BeTrue())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test2", "label_2": "test2"})
			Expect(ok).To(BeTrue())

			// Expect to find the new metrics for test3
			_, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test3", "label_2": "test3"})
			Expect(ok).To(BeTrue())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test3", "label_2": "test3"})
			Expect(ok).To(BeTrue())
		})
		It("should replace with an empty store", func() {
			storeMetrics := []*metrics.StoreMetric{
				{
					GaugeMetric: testGauge1,
					Value:       3.65,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
				{
					GaugeMetric: testGauge2,
					Value:       5.3,
					Labels: map[string]string{
						"label_1": "test",
						"label_2": "test",
					},
				},
			}
			ms.Update(key.String(), storeMetrics)

			// Expect to find the metrics
			_, ok := FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeTrue())

			ms.ReplaceAll(nil)

			// Expect to not find the metrics now
			_, ok = FindMetricWithLabelValues("test_gauge_1", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
			_, ok = FindMetricWithLabelValues("test_gauge_2", map[string]string{"label_1": "test", "label_2": "test"})
			Expect(ok).To(BeFalse())
		})
	})
})
