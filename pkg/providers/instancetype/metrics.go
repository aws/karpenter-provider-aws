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
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	cloudProviderSubsystem = "cloudprovider"
	instanceTypeLabel      = "instance_type"
)

var (
	InstanceTypeVCPU = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
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
	InstanceTypeMemory = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
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
)
