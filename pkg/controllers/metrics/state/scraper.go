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

const tickPeriodSeconds = 5

type Scraper interface {
	Scrape(context.Context)
}

type MetricScraper struct {
	Cluster *state.Cluster

	scrapers         []Scraper
	updateChan       chan struct{}
	updateReturnChan chan struct{}
}

func NewMetricScraper(ctx context.Context, cluster *state.Cluster) *MetricScraper {
	mc := &MetricScraper{
		Cluster:          cluster,
		updateChan:       make(chan struct{}),
		updateReturnChan: make(chan struct{}),
	}
	mc.init(ctx)
	return mc
}

func (ms *MetricScraper) Update() {
	ms.updateChan <- struct{}{}
	<-ms.updateReturnChan
}

func (ms *MetricScraper) init(ctx context.Context) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("metric-scraper"))

	for _, c := range []Scraper{
		NewNodeScraper(ms.Cluster),
	} {
		ms.scrapers = append(ms.scrapers, c)
	}

	go func() {
		ticker := time.NewTicker(tickPeriodSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logging.FromContext(ctx).Debugf("Terminating metric-scraper")
				return
			case <-ms.updateChan:
				ms.scrape(ctx)
				ms.updateReturnChan <- struct{}{}
			case <-ticker.C:
				ms.scrape(ctx)
			}
		}
	}()
}

func (ms *MetricScraper) scrape(ctx context.Context) {
	for _, c := range ms.scrapers {
		c.Scrape(ctx)
	}
}
