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

type StackDeserializeEnd struct{}

func GetRecordStackDeserializeEndMiddleware() *StackDeserializeEnd {
	return &StackDeserializeEnd{}
}

func (m *StackDeserializeEnd) ID() string {
	return "StackDeserializeEnd"
}

func (m *StackDeserializeEnd) HandleDeserialize(
	ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, attemptErr error,
) {

	out, metadata, err := next.HandleDeserialize(ctx, in)

	mctx := awsmetrics.Context(ctx)

	attemptMetrics, attemptErr := mctx.Data().LatestAttempt()

	if attemptErr != nil {
		fmt.Println(attemptErr)
	} else {
		attemptMetrics.DeserializeEndTime = time.Now().UTC()
	}

	return out, metadata, err

}
