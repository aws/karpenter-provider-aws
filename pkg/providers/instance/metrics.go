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
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	cloudProviderSubsystem = "cloudprovider"
	zoneLabel              = "zone"
	zoneIDLabel            = "zone_id"
)

var (
	// Counts per-offering CreateFleet errors, not per-NodeClaim attempts: one CreateFleet
	// call can fail multiple offerings across different zones and instance types.
	InstanceLaunchFailuresTotal = opmetrics.NewPrometheusCounter(
		crmetrics.Registry,
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_launch_failures_total",
			Help:      "Number of instance launch (CreateFleet offering) failures, dimensioned by availability zone, zone ID, capacity type, and launch failure reason.",
		},
		[]string{
			zoneLabel,
			zoneIDLabel,
			metrics.CapacityTypeLabel,
			metrics.ReasonLabel,
		},
	)
	InstanceTerminationFailuresTotal = opmetrics.NewPrometheusCounter(
		crmetrics.Registry,
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: cloudProviderSubsystem,
			Name:      "instance_termination_failures_total",
			Help:      "Number of instance termination (TerminateInstances) failures, dimensioned by availability zone and zone ID.",
		},
		[]string{
			zoneLabel,
			zoneIDLabel,
		},
	)
)
