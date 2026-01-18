// Copyright 2025 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import "testing"

func TestCollectorFunc(t *testing.T) {
	testDesc := NewDesc(
		"test_metric",
		"A test metric",
		nil, nil,
	)

	cf := CollectorFunc(func(ch chan<- Metric) {
		ch <- MustNewConstMetric(
			testDesc,
			GaugeValue,
			42.0,
		)
	})

	ch := make(chan Metric, 1)
	cf.Collect(ch)
	close(ch)

	metric := <-ch
	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	descCh := make(chan *Desc, 1)
	cf.Describe(descCh)
	close(descCh)

	desc := <-descCh
	if desc == nil {
		t.Fatal("Expected desc, got nil")
	}

	if desc.String() != testDesc.String() {
		t.Fatalf("Expected %s, got %s", testDesc.String(), desc.String())
	}
}

func TestCollectorFuncWithRegistry(t *testing.T) {
	reg := NewPedanticRegistry()

	cf := CollectorFunc(func(ch chan<- Metric) {
		ch <- MustNewConstMetric(
			NewDesc(
				"test_metric",
				"A test metric",
				nil, nil,
			),
			GaugeValue,
			42.0,
		)
	})

	if err := reg.Register(cf); err != nil {
		t.Errorf("Failed to register CollectorFunc: %v", err)
	}

	collectedMetrics, err := reg.Gather()
	if err != nil {
		t.Errorf("Failed to gather metrics: %v", err)
	}

	if len(collectedMetrics) != 1 {
		t.Errorf("Expected 1 metric family, got %d", len(collectedMetrics))
	}
}
