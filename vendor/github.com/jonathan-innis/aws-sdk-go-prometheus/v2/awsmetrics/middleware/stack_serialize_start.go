package middleware

import (
	"context"
	"time"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

type StackSerializeStart struct{}

func GetRecordStackSerializeStartMiddleware() *StackSerializeStart {
	return &StackSerializeStart{}
}

func (m *StackSerializeStart) ID() string {
	return "StackSerializeStart"
}

func (m *StackSerializeStart) HandleSerialize(
	ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler,
) (
	out middleware.SerializeOutput, metadata middleware.Metadata, err error,
) {

	mctx := awsmetrics.Context(ctx)
	mctx.Data().SerializeStartTime = time.Now().UTC()

	out, metadata, err = next.HandleSerialize(ctx, in)

	return out, metadata, err
}
