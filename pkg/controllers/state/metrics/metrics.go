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
	"strings"
	"time"

	"github.com/aws/karpenter/pkg/controllers/state"
	"knative.dev/pkg/logging"
)

const tickPeriodSeconds = 5

type scraper interface {
	getName() string
	init(context.Context)
	reset()
	update(context.Context)
}

type MetricScraper struct {
	Cluster *state.Cluster

	scrapers []scraper
}

func NewMetricCollector(ctx context.Context, cluster *state.Cluster) *MetricScraper {
	mc := &MetricScraper{
		Cluster: cluster,
	}
	mc.init(ctx)
	return mc
}

func (ms *MetricScraper) Reset() {
	for _, c := range ms.scrapers {
		c.reset()
	}
}

func (ms *MetricScraper) init(ctx context.Context) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("metric-scraper"))

	for _, c := range []scraper{
		newNodeCollector(ms.Cluster),
	} {
		ms.scrapers = append(ms.scrapers, c)
	}

	logging.FromContext(ctx).Infof("Starting metric-scraper with the following scrapers: %s", func() string {
		var sb strings.Builder
		for idx := range ms.scrapers {
			sb.WriteString(ms.scrapers[idx].getName())
			if idx < len(ms.scrapers)-1 {
				sb.WriteString(", ")
			}
		}
		return sb.String()
	}())

	// Initialize all metrics scrapers
	for _, scraper := range ms.scrapers {
		scraper.init(ctx)
	}

	go func() {
		ticker := time.NewTicker(tickPeriodSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logging.FromContext(ctx).Infof("Terminating metric-scraper")
				return
			case <-ticker.C:
				ms.update(ctx)
			}
		}
	}()
}

func (ms *MetricScraper) update(ctx context.Context) {
	for _, c := range ms.scrapers {
		c.update(ctx)
	}
}

func (ms *MetricScraper) ResetScrapers() {
	for _, c := range ms.scrapers {
		c.reset()
	}
}
