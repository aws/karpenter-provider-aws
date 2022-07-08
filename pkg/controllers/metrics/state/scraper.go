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

	scrapers         []scraper
	terminateChan    chan struct{}
	updateChan       chan struct{}
	updateReturnChan chan struct{}
}

func NewMetricScraper(ctx context.Context, cluster *state.Cluster) *MetricScraper {
	mc := &MetricScraper{
		Cluster:          cluster,
		terminateChan:    make(chan struct{}),
		updateChan:       make(chan struct{}),
		updateReturnChan: make(chan struct{}),
	}
	mc.init(ctx)
	return mc
}

func (ms *MetricScraper) Terminate() {
	ms.terminateChan <- struct{}{}
	ms.ResetScrapers()
}

func (ms *MetricScraper) Update() {
	ms.updateChan <- struct{}{}
	<-ms.updateReturnChan
}

func (ms *MetricScraper) init(ctx context.Context) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("metric-scraper"))

	for _, c := range []scraper{
		newNodeCollector(ms.Cluster),
	} {
		ms.scrapers = append(ms.scrapers, c)
	}

	logging.FromContext(ctx).Debugf("Starting metric-scraper with the following scrapers: %s", strings.Join(ms.getScraperNames(), ", "))

	// Initialize all metrics scrapers
	for _, scraper := range ms.scrapers {
		scraper.init(ctx)
	}

	go func() {
		ticker := time.NewTicker(tickPeriodSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ms.terminateChan:
				logging.FromContext(ctx).Debugf("Terminating metric-scraper")
				return
			case <-ctx.Done():
				logging.FromContext(ctx).Debugf("Terminating metric-scraper")
				return
			case <-ms.updateChan:
				ms.update(ctx)
				ms.updateReturnChan <- struct{}{}
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

func (ms *MetricScraper) getScraperNames() []string {
	names := []string{}
	for _, scraper := range ms.scrapers {
		names = append(names, scraper.getName())
	}
	return names
}

func (ms *MetricScraper) ResetScrapers() {
	for _, c := range ms.scrapers {
		c.reset()
	}
}
