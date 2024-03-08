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

package instancetype

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	cloudProviderSubsystem = "cloudprovider"
	instanceTypeLabel      = "instance_type"
	capacityTypeLabel      = "capacity_type"
	zoneLabel              = "zone"
)

var (
	instanceTypeVCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_type_cpu_cores",
			Help:      "VCPUs cores for a given instance type.",
		},
		[]string{
			instanceTypeLabel,
		},
	)
	instanceTypeMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_type_memory_bytes",
			Help:      "Memory, in bytes, for a given instance type.",
		},
		[]string{
			instanceTypeLabel,
		},
	)
	instanceTypeOfferingAvailable = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_type_offering_available",
			Help:      "Instance type offering availability, based on instance type, capacity type, and zone",
		},
		[]string{
			instanceTypeLabel,
			capacityTypeLabel,
			zoneLabel,
		},
	)
	instanceTypeOfferingPriceEstimate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_type_offering_price_estimate",
			Help:      "Instance type offering estimated hourly price used when making informed decisions on node cost calculation, based on instance type, capacity type, and zone.",
		},
		[]string{
			instanceTypeLabel,
			capacityTypeLabel,
			zoneLabel,
		})
)

func init() {
	crmetrics.Registry.MustRegister(instanceTypeVCPU, instanceTypeMemory, instanceTypeOfferingAvailable, instanceTypeOfferingPriceEstimate)
}
