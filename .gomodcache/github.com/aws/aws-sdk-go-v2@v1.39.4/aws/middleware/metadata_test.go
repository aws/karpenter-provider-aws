package middleware

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/smithy-go/middleware"
)

func TestServiceMetadataProvider(t *testing.T) {
	m := RegisterServiceMetadata{
		ServiceID:     "Bar",
		SigningName:   "Jaz",
		Region:        "Fuz",
		OperationName: "FooOp",
	}

	_, _, err := m.HandleInitialize(context.Background(), middleware.InitializeInput{}, middleware.InitializeHandlerFunc(func(
		ctx context.Context, input middleware.InitializeInput,
	) (o middleware.InitializeOutput, m middleware.Metadata, err error) {
		t.Helper()
		if e, a := "Bar", GetServiceID(ctx); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
		if e, a := "Jaz", GetSigningName(ctx); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
		if e, a := "Fuz", GetRegion(ctx); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
		if e, a := "FooOp", GetOperationName(ctx); !reflect.DeepEqual(e, a) {
			t.Errorf("expected %v, got %v", e, a)
		}
		return o, m, err
	}))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
