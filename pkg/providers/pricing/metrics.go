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

package pricing

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	cloudProviderSubsystem = "cloudprovider"
)

var (
	InstanceTypeLabel     = "instance_type"
	CapacityTypeLabel     = "capacity_type"
	RegionLabel           = "region"
	TopologyLabel         = "zone"
	InstancePriceEstimate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_type_price_estimate",
			Help:      "Estimated hourly price used when making informed decisions on node cost calculation. This is updated once on startup and then every 12 hours.",
		},
		[]string{
			InstanceTypeLabel,
			CapacityTypeLabel,
			RegionLabel,
			TopologyLabel,
		})
)

func init() {
	crmetrics.Registry.MustRegister(InstancePriceEstimate)
}
