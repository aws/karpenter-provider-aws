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
	nodeClassSubsystem = "ec2nodeclasses"
	nodeClassLabel     = "nodeclass"
	// userDataMaxBytes is the ec2:CreateLaunchTemplate user data limit. EC2 is the authority here — we
	// never gate provisioning on this locally, we only warn as it's approached so growth (cert bundles,
	// custom user data) is visible before the hard rejection.
	userDataMaxBytes  = 16384
	userDataWarnBytes = userDataMaxBytes * 9 / 10
)

var (
	// UserDataBytes is recorded when the user data is rendered, just before ec2:CreateLaunchTemplate, so an
	// oversized rendering is still reported even though EC2 rejects the create. Only the create path writes
	// the gauge, which stays current because launch templates are deleted from EC2 when their cache entry
	// expires (cache.DefaultTTL), so the next EnsureAll re-creates and re-measures. A NodeClass has no series
	// until it reaches this path — notably when dry-run validation is disabled, nothing calls EnsureAll until
	// a node actually launches.
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
