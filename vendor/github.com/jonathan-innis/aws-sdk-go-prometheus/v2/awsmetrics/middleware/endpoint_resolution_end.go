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

type EndpointResolutionEnd struct{}

func GetRecordEndpointResolutionEndMiddleware() *EndpointResolutionEnd {
	return &EndpointResolutionEnd{}
}

func (m *EndpointResolutionEnd) ID() string {
	return "EndpointResolutionEnd"
}

// Deprecated: Endpoint resolution now occurs in Finalize. The ResolveEndpoint
// middleware remains in serialize but is largely a no-op.
func (m *EndpointResolutionEnd) HandleSerialize(
	ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler,
) (
	out middleware.SerializeOutput, metadata middleware.Metadata, err error,
) {

	mctx := awsmetrics.Context(ctx)
	mctx.Data().ResolveEndpointEndTime = time.Now().UTC()

	out, metadata, err = next.HandleSerialize(ctx, in)

	return out, metadata, err
}

func (m *EndpointResolutionEnd) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	middleware.FinalizeOutput, middleware.Metadata, error,
) {
	mctx := awsmetrics.Context(ctx)
	mctx.Data().ResolveEndpointEndTime = time.Now().UTC()
	return next.HandleFinalize(ctx, in)
}
