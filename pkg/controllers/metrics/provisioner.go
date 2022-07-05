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

import "github.com/prometheus/client_golang/prometheus"

var (
	// Provisioner Metrics
	limitGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "limit",
			Help:      "The Provisioner Limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
	usageGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage",
			Help:      "The Provisioner Usage is the amount of resources that have been provisioned by a particular provisioner. Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
	usagePctGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage_pct",
			Help:      "The Provisioner Usage Percentage is the percentage of each resource used based on the resources provisioned and the limits that have been configured in the range [0,100].  Labeled by provisioner name and resource type.",
		},
		provisionerLabelNames(),
	)
)

func provisionerLabelNames() []string {
	return []string{
		resourceType,
		provisionerName,
	}
}
