package middleware_test

import (
	"context"
	"io"
	"testing"

	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
)

type mockWithContextLogger struct {
	logging.Logger
	Context context.Context
}

func (m mockWithContextLogger) WithContext(ctx context.Context) logging.Logger {
	m.Context = ctx
	return m
}

func TestGetLogger(t *testing.T) {
	if logger := middleware.GetLogger(context.Background()); logger == nil {
		t.Fatal("expect logger to not be nil")
	} else if _, ok := logger.(logging.Nop); !ok {
		t.Fatal("expect GetLogger to fallback to Nop")
	}

	standardLogger := logging.NewStandardLogger(io.Discard)
	ctx := middleware.SetLogger(context.Background(), standardLogger)

	if logger := middleware.GetLogger(ctx); logger == nil {
		t.Fatal("expect logger to not be nil")
	} else if logger != standardLogger {
		t.Error("expect logger to be standard logger")
	}

	withContextLogger := mockWithContextLogger{}
	ctx = middleware.SetLogger(context.Background(), withContextLogger)
	if logger := middleware.GetLogger(ctx); logger == nil {
		t.Fatal("expect logger to not be nil")
	} else if mock, ok := logger.(mockWithContextLogger); !ok {
		t.Error("expect logger to be context logger")
	} else if mock.Context != ctx {
		t.Error("expect logger context to match")
	}
}
