package tracing

import (
	"context"
	"testing"
)

// nopSpan has no fields so all values are equal, adding an int allows us to
// differentiate
type mockSpan struct {
	nopSpan
	id int
}

func TestSpanContextAPIs(t *testing.T) {
	parent := mockSpan{id: 1}
	child := mockSpan{id: 2}

	ctx := context.Background()
	span, ok := GetSpan(ctx)
	if ok {
		t.Error("should have no span at the start but it did")
	}

	// set & expect parent
	ctx = WithSpan(ctx, parent)
	span, _ = GetSpan(ctx)
	if actual, ok := span.(mockSpan); !ok || parent != actual {
		t.Errorf("span %d != %d", parent.id, actual.id)
	}

	// set & expect child
	ctx = WithSpan(ctx, child)
	span, _ = GetSpan(ctx)
	if actual, ok := span.(mockSpan); !ok || child != actual {
		t.Errorf("span %d != %d", child.id, actual.id)
	}

	// pop, expect popped child, with parent remaining
	ctx, span = PopSpan(ctx)
	if actual, ok := span.(mockSpan); !ok || child != actual {
		t.Errorf("span %d != %d", child.id, actual.id)
	}
	span, _ = GetSpan(ctx)
	if actual, ok := span.(mockSpan); !ok || parent != actual {
		t.Errorf("span %d != %d", parent.id, actual.id)
	}

	// pop, expect popped parent, with no span remaining
	ctx, span = PopSpan(ctx)
	if actual, ok := span.(mockSpan); !ok || parent != actual {
		t.Errorf("span %d != %d", parent.id, actual.id)
	}
	span, ok = GetSpan(ctx)
	if ok {
		t.Error("should have no span at the end but it did")
	}

	// pop, expect it to be a nop since nothing is left
	ctx, span = PopSpan(ctx)
	if _, ok := span.(nopSpan); !ok {
		t.Errorf("should have been nop span on last pop but was %T", span)
	}
}
