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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	timestreamtypes "github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
)

const (
	karpenterMetricRegion    = "us-east-2"
	karpenterMetricDatabase  = "karpenterTesting"
	karpenterMetricTableName = "sweeperCleanedResources"
)

type Client interface {
	FireMetric(context.Context, string, float64, string) error
}

type TimeStream struct {
	timestreamClient *timestreamwrite.Client
}

func NewTimeStream(cfg aws.Config) *TimeStream {
	return &TimeStream{timestreamClient: timestreamwrite.NewFromConfig(cfg, WithRegion(karpenterMetricRegion))}
}

func (t *TimeStream) FireMetric(ctx context.Context, name string, value float64, region string) error {
	_, err := t.timestreamClient.WriteRecords(ctx, &timestreamwrite.WriteRecordsInput{
		DatabaseName: aws.String(karpenterMetricDatabase),
		TableName:    aws.String(karpenterMetricTableName),
		Records: []timestreamtypes.Record{
			{
				MeasureName:  aws.String(name),
				MeasureValue: aws.String(fmt.Sprintf("%f", value)),
				Time:         aws.String(fmt.Sprintf("%d", time.Now().UnixMilli())),
				Dimensions: []timestreamtypes.Dimension{
					{
						Name:  aws.String("region"),
						Value: aws.String(region),
					},
				},
			},
		},
	})
	return err
}

func WithRegion(region string) func(*timestreamwrite.Options) {
	return func(o *timestreamwrite.Options) {
		o.Region = region
	}
}
