// This package is designated as private and is intended for use only by the
// smithy client runtime. The exported API therein is not considered stable and
// is subject to breaking changes without notice.

package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

type MetricCollection struct {
	cc        *awsmetrics.SharedConnectionCounter
	publisher awsmetrics.MetricPublisher
}

func GetSetupMetricCollectionMiddleware(
	counter *awsmetrics.SharedConnectionCounter, publisher awsmetrics.MetricPublisher,
) *MetricCollection {
	return &MetricCollection{
		cc:        counter,
		publisher: publisher,
	}
}

func (m *MetricCollection) ID() string {
	return "MetricCollection"
}

func (m *MetricCollection) HandleInitialize(
	ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {

	ctx = awsmetrics.InitMetricContext(ctx, m.cc, m.publisher)

	mctx := awsmetrics.Context(ctx)
	metricData := mctx.Data()

	metricData.RequestStartTime = time.Now().UTC()

	out, metadata, err = next.HandleInitialize(ctx, in)

	metricData.RequestEndTime = time.Now().UTC()

	if err == nil {
		metricData.Success = 1
	} else {
		metricData.Success = 0
	}

	metricData.ComputeRequestMetrics()

	publishErr := m.publisher.PostRequestMetrics(awsmetrics.Context(ctx).Data())
	if publishErr != nil {
		fmt.Println("Failed to post request metrics")
	}

	return out, metadata, err
}
