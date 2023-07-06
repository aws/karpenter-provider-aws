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

package aws

import (
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"

	"github.com/aws/karpenter/test/pkg/environment/common"
)

type EventType string

const (
	ProvisioningEventType   EventType = "provisioning"
	DeprovisioningEventType EventType = "deprovisioning"
)

var provisioningDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "karpenter_testing",
	Subsystem: "scale",
	Name:      "provisioning_duration_seconds",
	Help:      "The provisioning duration in seconds.",
}, dimensions)

var deprovisioningDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "karpenter_testing",
	Subsystem: "scale",
	Name:      "deprovisioning_duration_seconds",
	Help:      "The deprovisioning duration in seconds.",
}, dimensions)

const (
	TestGroupDimension              = "group"
	TestNameDimension               = "name"
	GitRefDimension                 = "gitRef"
	ProvisionedNodeCountDimension   = "provisionedNodeCount"
	DeprovisionedNodeCountDimension = "deprovisionedNodeCount"
	PodDensityDimension             = "podDensity"
)

var dimensions = []string{
	TestGroupDimension,
	TestNameDimension,
	GitRefDimension,
	ProvisionedNodeCountDimension,
	DeprovisionedNodeCountDimension,
	PodDensityDimension,
}

func (env *Environment) MeasureProvisioningDurationFor(f func(), dimensions map[string]string) {
	GinkgoHelper()

	env.MeasureDurationFor(f, ProvisioningEventType, dimensions)
}

func (env *Environment) MeasureDeprovisioningDurationFor(f func(), dimensions map[string]string) {
	GinkgoHelper()

	env.MeasureDurationFor(f, DeprovisioningEventType, dimensions)
}

// MeasureDurationFor observes the duration between the beginning of the function f() and the end of the function f()
func (env *Environment) MeasureDurationFor(f func(), eventType EventType, dimensions map[string]string) {
	GinkgoHelper()
	start := time.Now()
	f()
	gitRef := "n/a"
	if env.Context.Value(common.GitRefContextKey) != nil {
		gitRef = env.Value(common.GitRefContextKey).(string)
	}

	labels := lo.Assign(dimensions, map[string]string{
		GitRefDimension: gitRef,
	})
	switch eventType {
	case ProvisioningEventType:
		provisioningDuration.With(labels).Set(time.Since(start).Seconds())
	case DeprovisioningEventType:
		deprovisioningDuration.With(labels).Set(time.Since(start).Seconds())
	}
	Expect(env.Pusher.Add()).To(Succeed())
}
