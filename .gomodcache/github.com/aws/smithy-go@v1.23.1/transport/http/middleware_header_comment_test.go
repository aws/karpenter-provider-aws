package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/smithy-go/middleware"
)

func TestWithHeaderComment_CaseInsensitive(t *testing.T) {
	stack, err := newTestStack(
		WithHeaderComment("foo", "bar"),
	)
	if err != nil {
		t.Errorf("expected no error on new stack, got %v", err)
	}

	r := injectBuildRequest(stack)
	r.Header.Set("Foo", "baz")

	if err := handle(stack); err != nil {
		t.Errorf("expected no error on handle, got %v", err)
	}

	expectHeader(t, r.Header, "Foo", "baz (bar)")
}

func TestWithHeaderComment_Noop(t *testing.T) {
	stack, err := newTestStack(
		WithHeaderComment("foo", "bar"),
	)
	if err != nil {
		t.Errorf("expected no error on new stack, got %v", err)
	}

	r := injectBuildRequest(stack)

	if err := handle(stack); err != nil {
		t.Errorf("expected no error on handle, got %v", err)
	}

	expectHeader(t, r.Header, "Foo", "")
}

func TestWithHeaderComment_MultiCaseInsensitive(t *testing.T) {
	stack, err := newTestStack(
		WithHeaderComment("foo", "c1"),
		WithHeaderComment("Foo", "c2"),
		WithHeaderComment("baz", "c3"),
		WithHeaderComment("Baz", "c4"),
	)
	if err != nil {
		t.Errorf("expected no error on new stack, got %v", err)
	}

	r := injectBuildRequest(stack)
	r.Header.Set("Foo", "1")
	r.Header.Set("Baz", "2")

	if err := handle(stack); err != nil {
		t.Errorf("expected no error on handle, got %v", err)
	}

	expectHeader(t, r.Header, "Foo", "1 (c1) (c2)")
	expectHeader(t, r.Header, "Baz", "2 (c3) (c4)")
}

func newTestStack(fns ...func(*middleware.Stack) error) (*middleware.Stack, error) {
	s := middleware.NewStack("", NewStackRequest)
	for _, fn := range fns {
		if err := fn(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func handle(stack *middleware.Stack) error {
	_, _, err := middleware.DecorateHandler(
		middleware.HandlerFunc(
			func(ctx context.Context, input interface{}) (
				interface{}, middleware.Metadata, error,
			) {
				return nil, middleware.Metadata{}, nil
			},
		),
		stack,
	).Handle(context.Background(), nil)
	return err
}

func injectBuildRequest(s *middleware.Stack) *Request {
	r := NewStackRequest()
	s.Build.Add(
		middleware.BuildMiddlewareFunc(
			"injectBuildRequest",
			func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
				middleware.BuildOutput, middleware.Metadata, error,
			) {
				return next.HandleBuild(ctx, middleware.BuildInput{Request: r})
			},
		),
		middleware.Before,
	)
	return r.(*Request)
}

func expectHeader(t *testing.T, header http.Header, h, ev string) {
	if av := header.Get(h); ev != av {
		t.Errorf("expected header '%s: %s', got '%s'", h, ev, av)
	}
}
