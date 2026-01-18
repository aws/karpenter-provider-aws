package sdk

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSleepWithContext(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	err := sleepWithContext(ctx, 1*time.Millisecond)
	if err != nil {
		t.Errorf("expect context to not be canceled, got %v", err)
	}
}

func TestSleepWithContext_Canceled(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.Background())
	cancelFn()

	err := sleepWithContext(ctx, 10*time.Second)
	if err == nil {
		t.Fatalf("expect error, did not get one")
	}

	if e, a := "context canceled", err.Error(); !strings.Contains(a, e) {
		t.Errorf("expect %v error, got %v", e, a)
	}
}
