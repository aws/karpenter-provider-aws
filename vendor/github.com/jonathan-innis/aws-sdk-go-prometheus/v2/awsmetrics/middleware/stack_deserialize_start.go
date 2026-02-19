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

type StackDeserializeStart struct{}

func GetRecordStackDeserializeStartMiddleware() *StackDeserializeStart {
	return &StackDeserializeStart{}
}

func (m *StackDeserializeStart) ID() string {
	return "StackDeserializeStart"
}

func (m *StackDeserializeStart) HandleDeserialize(
	ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {

	out, metadata, err = next.HandleDeserialize(ctx, in)

	mctx := awsmetrics.Context(ctx)

	attemptMetrics, attemptErr := mctx.Data().LatestAttempt()

	if attemptErr != nil {
		fmt.Println(err)
	} else {
		attemptMetrics.DeserializeStartTime = time.Now().UTC()
	}

	return out, metadata, err
}
