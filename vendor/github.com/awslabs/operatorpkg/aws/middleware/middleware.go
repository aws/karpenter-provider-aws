package middleware

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	"github.com/awslabs/operatorpkg/serrors"
)

const (
	AWSRequestIDLogKey     = "aws-request-id"
	AWSStatusCodeLogKey    = "aws-status-code"
	AWSServiceNameLogKey   = "aws-service-name"
	AWSOperationNameLogKey = "aws-operation-name"
	AWSErrorCodeLogKey     = "aws-error-code"
)

// StructuredErrorHandler injects structured keys and values into the error returned by the AWS request
// It doesn't modify the error message so error messages will still contain the structured values
var StructuredErrorHandler = func(stack *middleware.Stack) error {
	return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("StructuredErrorHandler", func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
		out, metadata, err := next.HandleDeserialize(ctx, in)
		if err == nil {
			return out, metadata, nil
		}
		values := []any{AWSServiceNameLogKey, middleware.GetServiceID(ctx), AWSOperationNameLogKey, middleware.GetOperationName(ctx)}
		temp := err
		for temp != nil {
			if v, ok := temp.(*http.ResponseError); ok {
				values = append(values, AWSRequestIDLogKey, v.RequestID, AWSStatusCodeLogKey, v.Response.StatusCode)
			}
			if v, ok := temp.(*smithy.GenericAPIError); ok {
				values = append(values, AWSErrorCodeLogKey, v.Code)
			}
			temp = errors.Unwrap(temp)
		}
		return out, metadata, serrors.Wrap(err, values...)
	}), middleware.Before)
}
