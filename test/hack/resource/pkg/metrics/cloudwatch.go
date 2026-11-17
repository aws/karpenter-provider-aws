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

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"go.uber.org/multierr"
)

// CloudWatch mirrors the Timestream records so that CloudWatch alarms can watch them. The namespace is
// <database>/<table>, for example karpenterTesting/resourceCount, and the Region dimension is the record's region.
type CloudWatch struct {
	client *cloudwatch.Client
}

func NewCloudWatch(cfg aws.Config) *CloudWatch {
	return &CloudWatch{client: cloudwatch.NewFromConfig(cfg, func(o *cloudwatch.Options) { o.Region = karpenterMetricRegion })}
}

func (c *CloudWatch) FireMetric(ctx context.Context, tableName string, name string, value float64, region string) error {
	_, err := c.client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(fmt.Sprintf("%s/%s", karpenterMetricDatabase, tableName)),
		MetricData: []cloudwatchtypes.MetricDatum{{
			MetricName: aws.String(name),
			Value:      aws.Float64(value),
			Unit:       cloudwatchtypes.StandardUnitCount,
			Dimensions: []cloudwatchtypes.Dimension{{Name: aws.String("Region"), Value: aws.String(region)}},
		}},
	})
	return err
}

// Multi fires every metric to all clients, so that one failing client doesn't stop the others
type Multi []Client

func (m Multi) FireMetric(ctx context.Context, tableName string, name string, value float64, region string) error {
	var errs error
	for _, c := range m {
		errs = multierr.Append(errs, c.FireMetric(ctx, tableName, name, value, region))
	}
	return errs
}
