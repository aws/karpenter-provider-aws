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

package metrics

import (
	"context"
	"time"

	"github.com/aws/karpenter/pkg/controllers/state"
	"knative.dev/pkg/logging"
)

type collector interface {
	init(context.Context)
	update(context.Context)
	reset()
}

type MetricCollector struct {
	Cluster *state.Cluster

	terminate  chan bool
	collectors []collector
}

func NewMetricCollector(ctx context.Context, cluster *state.Cluster) *MetricCollector {
	mc := &MetricCollector{
		Cluster:   cluster,
		terminate: make(chan bool),
	}
	mc.init(ctx)
	return mc
}

func (mc *MetricCollector) Terminate() {
	mc.terminate <- true
}

func (mc *MetricCollector) Reset() {
	for _, c := range mc.collectors {
		c.reset()
	}
}

func (mc *MetricCollector) init(ctx context.Context) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("metrics-scraper"))

	// Add metric collectors
	mc.collectors = append(mc.collectors, newPodCollector(mc.Cluster))
	logging.FromContext(ctx).Infof("Starting metrics collector with %d collectors", len(mc.collectors))

	// Initialize all metrics collectors
	for _, collector := range mc.collectors {
		collector.init(ctx)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-mc.terminate:
				logging.FromContext(ctx).Infof("Terminating cluster-state metrics scraper")
				return
			case <-ticker.C:
				mc.update(ctx)
			}
		}
	}()
}

func (m *MetricCollector) update(ctx context.Context) {
	for _, c := range m.collectors {
		c.update(ctx)
	}
}
