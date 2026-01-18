package middleware

import (
	"context"
	"testing"
)

type mockIder struct {
	Identifier string
}

func (m *mockIder) ID() string { return m.Identifier }

func noError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}
}

func mockInitializeMiddleware(id string) InitializeMiddleware {
	return InitializeMiddlewareFunc(id,
		func(
			ctx context.Context, in InitializeInput, next InitializeHandler,
		) (
			out InitializeOutput, metadata Metadata, err error,
		) {
			return next.HandleInitialize(ctx, in)
		})
}

func mockSerializeMiddleware(id string) SerializeMiddleware {
	return SerializeMiddlewareFunc(id,
		func(
			ctx context.Context, in SerializeInput, next SerializeHandler,
		) (
			out SerializeOutput, metadata Metadata, err error,
		) {
			return next.HandleSerialize(ctx, in)
		})
}

func mockBuildMiddleware(id string) BuildMiddleware {
	return BuildMiddlewareFunc(id,
		func(
			ctx context.Context, in BuildInput, next BuildHandler,
		) (
			out BuildOutput, metadata Metadata, err error,
		) {
			return next.HandleBuild(ctx, in)
		})
}

func mockFinalizeMiddleware(id string) FinalizeMiddleware {
	return FinalizeMiddlewareFunc(id,
		func(
			ctx context.Context, in FinalizeInput, next FinalizeHandler,
		) (
			out FinalizeOutput, metadata Metadata, err error,
		) {
			return next.HandleFinalize(ctx, in)
		})
}

func mockDeserializeMiddleware(id string) DeserializeMiddleware {
	return DeserializeMiddlewareFunc(id,
		func(
			ctx context.Context, in DeserializeInput, next DeserializeHandler,
		) (
			out DeserializeOutput, metadata Metadata, err error,
		) {
			return next.HandleDeserialize(ctx, in)
		})
}
