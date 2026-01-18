package middleware

import (
	"context"
	"testing"
)

func TestStackValues(t *testing.T) {
	ctx := context.Background()

	// Ensure empty stack values don't return something
	if v := GetStackValue(ctx, "some key"); v != nil {
		t.Fatalf("expect not-existing key to be nil, got %T, %v", v, v)
	}

	// Add a stack values, ensure not polluting previous context.
	ctx2 := WithStackValue(ctx, "some key", "foo")
	ctx2 = WithStackValue(ctx2, "some other key", "bar")
	if v := GetStackValue(ctx, "some key"); v != nil {
		t.Fatalf("expect not-existing key to be nil, got %T, %v", v, v)
	}
	if v, ok := GetStackValue(ctx2, "some key").(string); !ok || v != "foo" {
		t.Fatalf("expect key to be present")
	}
	if v, ok := GetStackValue(ctx2, "some other key").(string); !ok || v != "bar" {
		t.Fatalf("expect key to be present")
	}

	// Clear the stack values ensure new context doesn't have any stack values.
	ctx3 := ClearStackValues(ctx2)
	if v, ok := GetStackValue(ctx2, "some key").(string); !ok || v != "foo" {
		t.Fatalf("expect key to be present")
	}
	if v := GetStackValue(ctx3, "some key"); v != nil {
		t.Fatalf("expect not-existing key to be nil, got %T, %v", v, v)
	}
	if v := GetStackValue(ctx3, "some other key"); v != nil {
		t.Fatalf("expect not-existing key to be nil, got %T, %v", v, v)
	}
}
