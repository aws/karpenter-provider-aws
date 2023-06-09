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
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"

	"github.com/aws/karpenter/test/pkg/environment/common"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
)

var _ cloudwatchiface.CloudWatchAPI = (*NoOpCloudwatchAPI)(nil)

type NoOpCloudwatchAPI struct {
	cloudwatch.CloudWatch
}

func (o NoOpCloudwatchAPI) PutMetricData(_ *cloudwatch.PutMetricDataInput) (*cloudwatch.PutMetricDataOutput, error) {
	return nil, nil
}

type EventType string

const (
	ProvisioningEventType   EventType = "provisioning"
	DeprovisioningEventType EventType = "deprovisioning"
)

const (
	scaleTestingMetricNamespace     = v1alpha5.TestingGroup + "/scale"
	TestEventTypeDimension          = "eventType"
	TestSubEventTypeDimension       = "subEventType"
	TestGroupDimension              = "group"
	TestNameDimension               = "name"
	GitRefDimension                 = "gitRef"
	DeprovisionedNodeCountDimension = "deprovisionedNodeCount"
	ProvisionedNodeCountDimension   = "provisionedNodeCount"
	PodDensityDimension             = "podDensity"
)

// MeasureDurationFor observes the duration between the beginning of the function f() and the end of the function f()
func (env *Environment) MeasureDurationFor(f func(), eventType EventType, group, name string, additionalLabels map[string]string) {
	GinkgoHelper()
	start := time.Now()
	f()
	gitRef := "n/a"
	if env.Context.Value(common.GitRefContextKey) != nil {
		gitRef = env.Value(common.GitRefContextKey).(string)
	}
	env.ExpectEventDurationMetric(time.Since(start), lo.Assign(map[string]string{
		TestEventTypeDimension: string(eventType),
		TestGroupDimension:     group,
		TestNameDimension:      name,
		GitRefDimension:        gitRef,
	}, additionalLabels))
}

func (env *Environment) ExpectEventDurationMetric(d time.Duration, labels map[string]string) {
	GinkgoHelper()
	env.ExpectMetric("eventDuration", cloudwatch.StandardUnitSeconds, d.Seconds(), labels)
}

func (env *Environment) ExpectMetric(name string, unit string, value float64, labels map[string]string) {
	GinkgoHelper()
	_, err := env.CloudwatchAPI.PutMetricData(&cloudwatch.PutMetricDataInput{
		Namespace: aws.String(scaleTestingMetricNamespace),
		MetricData: []*cloudwatch.MetricDatum{
			{
				MetricName: aws.String(name),
				Dimensions: lo.MapToSlice(labels, func(k, v string) *cloudwatch.Dimension {
					return &cloudwatch.Dimension{
						Name:  aws.String(k),
						Value: aws.String(v),
					}
				}),
				Unit:      aws.String(unit),
				Value:     aws.Float64(value),
				Timestamp: aws.Time(time.Now()),
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())
}

func GenerateTestDimensions(provisionedNodeCount, deprovisionedNodeCount, podDensity int) map[string]string {
	return map[string]string{
		DeprovisionedNodeCountDimension: strconv.Itoa(deprovisionedNodeCount),
		ProvisionedNodeCountDimension:   strconv.Itoa(provisionedNodeCount),
		PodDensityDimension:             strconv.Itoa(podDensity),
	}
}
