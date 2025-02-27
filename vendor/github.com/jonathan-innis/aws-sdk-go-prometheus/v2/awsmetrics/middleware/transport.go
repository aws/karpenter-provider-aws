// This package is designated as private and is intended for use only by the
// smithy client runtime. The exported API therein is not considered stable and
// is subject to breaking changes without notice.

package middleware

import (
	"context"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

type TransportMetrics struct{}

func GetTransportMetricsMiddleware() *TransportMetrics {
	return &TransportMetrics{}
}

func (m *TransportMetrics) ID() string {
	return "TransportMetrics"
}

func (m *TransportMetrics) HandleDeserialize(
	ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, attemptErr error,
) {

	mctx := awsmetrics.Context(ctx)

	if attempt, e := mctx.Data().LatestAttempt(); e == nil {
		attempt.ServiceCallStart = time.Now().UTC()
		mctx.ConnectionCounter().AddActiveRequest()
	}

	out, metadata, err := next.HandleDeserialize(ctx, in)

	if attempt, e := mctx.Data().LatestAttempt(); e == nil {
		attempt.ServiceCallEnd = time.Now().UTC()
		mctx.ConnectionCounter().RemoveActiveRequest()
	}

	return out, metadata, err

}
