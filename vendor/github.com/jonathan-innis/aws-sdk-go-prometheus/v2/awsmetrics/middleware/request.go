// This package is designated as private and is intended for use only by the
// smithy client runtime. The exported API therein is not considered stable and
// is subject to breaking changes without notice.

package middleware

import (
	"context"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

const (
	clientRequestIdKey = "Amz-Sdk-Invocation-Id"
	unkClientId        = "unk"
)

type RegisterMetricContext struct{}

func GetRegisterRequestMetricContextMiddleware() *RegisterMetricContext {
	return &RegisterMetricContext{}
}

func (m *RegisterMetricContext) ID() string {
	return "RegisterMetricContext"
}

func (m *RegisterMetricContext) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
) {

	mctx := awsmetrics.Context(ctx)
	metricData := mctx.Data()

	metricData.ServiceID = awsmiddleware.GetServiceID(ctx)
	metricData.OperationName = awsmiddleware.GetOperationName(ctx)
	metricData.PartitionID = awsmiddleware.GetPartitionID(ctx)
	metricData.Region = awsmiddleware.GetSigningRegion(ctx)

	switch req := in.Request.(type) {
	case *smithyhttp.Request:
		crid := req.Header.Get(clientRequestIdKey)
		if len(crid) == 0 {
			crid = unkClientId
		}
		metricData.ClientRequestID = crid
		metricData.RequestContentLength = req.ContentLength
	default:
		metricData.ClientRequestID = unkClientId
		metricData.RequestContentLength = -1
	}

	out, metadata, err = next.HandleFinalize(ctx, in)

	return out, metadata, err
}
