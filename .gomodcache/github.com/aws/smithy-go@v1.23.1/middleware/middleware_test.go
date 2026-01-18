package middleware

import (
	"context"
	"fmt"
	"testing"
)

var _ Handler = (HandlerFunc)(nil)
var _ Handler = (decoratedHandler{})

type mockMiddleware struct {
	id int
}

func (m mockMiddleware) ID() string {
	return fmt.Sprintf("mock middleware %d", m.id)
}

func (m mockMiddleware) HandleMiddleware(ctx context.Context, input interface{}, next Handler) (
	output interface{}, metadata Metadata, err error,
) {
	output, metadata, err = next.Handle(ctx, input)

	mockKeySet(&metadata, m.id, fmt.Sprintf("mock-%d", m.id))

	return output, metadata, err
}

type mockKey struct{ Key int }

func mockKeySet(md *Metadata, key int, val string) {
	md.Set(mockKey{Key: key}, val)
}

func mockKeyGet(md MetadataReader, key int) string {
	v := md.Get(mockKey{Key: key})
	if v == nil {
		return ""
	}

	return v.(string)
}

type mockHandler struct {
}

func (m *mockHandler) Handle(ctx context.Context, input interface{}) (
	output interface{}, metadata Metadata, err error,
) {
	return nil, metadata, nil
}

func TestDecorateHandler(t *testing.T) {
	mockHandler := &mockHandler{}
	h := DecorateHandler(
		mockHandler,
		mockMiddleware{id: 0},
		mockMiddleware{id: 1},
		mockMiddleware{id: 2},
	)

	_, metadata, err := h.Handle(context.Background(), struct{}{})
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	expectMeta := map[int]interface{}{
		0: "mock-0",
		1: "mock-1",
		2: "mock-2",
	}

	for key, expect := range expectMeta {
		v := mockKeyGet(metadata, key)
		if e, a := expect, v; e != a {
			t.Errorf("expect %v: %v metadata got %v", key, e, a)
		}
	}
}
