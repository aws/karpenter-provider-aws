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

package instance

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const instanceCacheSubsystem = "instance_cache"

// lastSyncUnixNano holds the time of the last successful instance cache sync, initialized to package load so a scrape
// before the first sync still reports a growing age. It is read at scrape time (not reconcile time) by the
// seconds_since_last_sync gauge below, so the reported staleness keeps climbing even if the controller stalls and
// stops reconciling.
var lastSyncUnixNano atomic.Int64

func init() {
	lastSyncUnixNano.Store(time.Now().UnixNano())
	crmetrics.Registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: instanceCacheSubsystem,
			Name:      "seconds_since_last_sync",
			Help: "Time in seconds since the last successful instance cache sync (DescribeInstances refresh), " +
				"computed at scrape time. Grows without bound if the instance cache controller stalls, while List/Get " +
				"keep serving the increasingly stale cache.",
		},
		func() float64 { return time.Since(time.Unix(0, lastSyncUnixNano.Load())).Seconds() },
	))
}

// recordSync marks a successful instance cache sync at time t.
func recordSync(t time.Time) { lastSyncUnixNano.Store(t.UnixNano()) }
