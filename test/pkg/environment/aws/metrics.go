package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"

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
	scaleTestingMetricNamespace = v1alpha5.TestingGroup + "/scale"
	TestEventTypeDimension      = "eventType"
	TestGroupDimension          = "group"
	TestNameDimension           = "name"
)

// MeasureDurationFor observes the duration between the beginning of the function f() and the end of the function f()
func (env *Environment) MeasureDurationFor(f func(), eventType EventType, group, name string) {
	GinkgoHelper()
	start := time.Now()
	f()
	env.ExpectEventDurationMetric(time.Since(start), map[string]string{
		TestEventTypeDimension: string(eventType),
		TestGroupDimension:     group,
		TestNameDimension:      name,
	})
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
