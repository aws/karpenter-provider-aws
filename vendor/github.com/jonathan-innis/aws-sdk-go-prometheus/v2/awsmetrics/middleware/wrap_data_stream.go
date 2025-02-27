package middleware

import (
	"context"
	"io"
	"reflect"

	"github.com/aws/smithy-go/middleware"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics/readcloserwithmetrics"
)

const (
	responseBodyFieldName = "Body"
)

type WrapDataContext struct{}

func GetWrapDataStreamMiddleware() *WrapDataContext {
	return &WrapDataContext{}
}

func (m *WrapDataContext) ID() string {
	return "BuildWrapDataStream"
}

func (m *WrapDataContext) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {

	out, metadata, err = next.HandleBuild(ctx, in)

	value := reflect.ValueOf(out.Result)

	if value.Kind() != reflect.Ptr {
		return out, metadata, err
	}
	value = value.Elem()

	if value.Kind() != reflect.Struct {
		return out, metadata, err
	}
	bodyField := value.FieldByName(responseBodyFieldName)

	if !(bodyField.IsValid() && bodyField.CanInterface()) {
		return out, metadata, err
	}

	body, ok := bodyField.Interface().(io.ReadCloser)

	if !ok {
		return out, metadata, err
	}

	bodyField.Set(reflect.ValueOf(readcloserwithmetrics.New(awsmetrics.Context(ctx), body)))

	return out, metadata, err
}
