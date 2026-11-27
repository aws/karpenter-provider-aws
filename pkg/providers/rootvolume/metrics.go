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

package rootvolume

import (
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	cloudProviderSubsystem = "cloudprovider"
	stateLabel             = "state"
)

var State = opmetrics.Label{
	Name: stateLabel,
	Help: "The final state of the root volume replacement task: `succeeded`, `failed` or `failed-detached`. See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/replace-root.html.",
}

var TasksTotal = opmetrics.NewPrometheusCounter(
	crmetrics.Registry,
	prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: cloudProviderSubsystem,
		Name:      "root_volume_replacement_tasks_total",
		Help:      "Number of root volume replacement tasks Karpenter observed finishing, dimensioned by final state.",
	},
	[]opmetrics.Label{
		State,
	},
	opmetrics.Alpha,
)
