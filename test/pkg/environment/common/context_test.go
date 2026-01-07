package common

import (
	"context"
	"testing"
	"time"
)

type testContextKey string

func TestMergeContextsValues(t *testing.T) {
	base := context.WithValue(context.Background(), GitRefContextKey, "base-ref")
	spec := context.WithValue(context.Background(), testContextKey("spec"), "spec-value")

	merged := mergeContexts(base, spec)

	if got := merged.Value(testContextKey("spec")); got != "spec-value" {
		t.Fatalf("expected spec value, got %v", got)
	}
	if got := merged.Value(GitRefContextKey); got != "base-ref" {
		t.Fatalf("expected base value, got %v", got)
	}
}

func TestMergeContextsCancellation(t *testing.T) {
	t.Run("spec cancel", func(t *testing.T) {
		base, baseCancel := context.WithCancel(context.Background())
		t.Cleanup(baseCancel)
		spec, specCancel := context.WithCancel(context.Background())

		merged := mergeContexts(base, spec)
		specCancel()

		select {
		case <-merged.Done():
		case <-time.After(time.Second):
			t.Fatal("expected merged context to be canceled by spec context")
		}

		if merged.Err() == nil {
			t.Fatal("expected merged context to have an error after cancellation")
		}
	})

	t.Run("base cancel", func(t *testing.T) {
		base, baseCancel := context.WithCancel(context.Background())
		spec := context.Background()

		merged := mergeContexts(base, spec)
		baseCancel()

		select {
		case <-merged.Done():
		case <-time.After(time.Second):
			t.Fatal("expected merged context to be canceled by base context")
		}

		if merged.Err() == nil {
			t.Fatal("expected merged context to have an error after cancellation")
		}
	})
}
