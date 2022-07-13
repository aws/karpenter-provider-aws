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

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/controllers/state"
)

const tickPeriodSeconds = 5

type Scraper interface {
	Scrape(context.Context)
}

type MetricScraper struct {
	cluster  *state.Cluster
	scrapers []Scraper
}

func StartMetricScraper(ctx context.Context, cluster *state.Cluster) {
	mc := &MetricScraper{
		cluster: cluster,
	}
	mc.init(ctx)
}

func (ms *MetricScraper) init(ctx context.Context) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("metric-scraper"))

	ms.scrapers = append(ms.scrapers, []Scraper{
		NewNodeScraper(ms.cluster),
	}...)

	go func() {
		ticker := time.NewTicker(tickPeriodSeconds * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logging.FromContext(ctx).Debugf("Terminating metric-scraper")
				return
			case <-ticker.C:
				for _, c := range ms.scrapers {
					c.Scrape(ctx)
				}
			}
		}
	}()
}
