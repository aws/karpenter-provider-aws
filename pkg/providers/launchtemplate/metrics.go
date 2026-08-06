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

package launchtemplate

import (
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

const (
	nodeClassSubsystem = "nodeclass"
	nodeClassLabel     = "nodeclass"
	// userDataMaxBytes is the ec2:CreateLaunchTemplate user data limit. EC2 is the authority here — we
	// never gate provisioning on this locally, we only warn as it's approached so growth (cert bundles,
	// custom user data) is visible before the hard rejection.
	userDataMaxBytes  = 16384
	userDataWarnBytes = userDataMaxBytes * 9 / 10
)

var (
	// UserDataBytes is recorded when a launch template is created, which is the only time the user data
	// is rendered. The launch template name is a hash that includes the user data, so any change to the
	// rendered user data produces a new launch template and refreshes this gauge.
	UserDataBytes = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: nodeClassSubsystem,
			Name:      "userdata_bytes",
			Help:      "Size in bytes of the rendered user data (raw, pre-base64) for the EC2NodeClass",
		},
		[]string{
			nodeClassLabel,
		},
	)
)
