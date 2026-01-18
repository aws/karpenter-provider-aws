package retry

import (
	"sync"
	"testing"
)

func TestAdaptiveTokenBucket(t *testing.T) {
	b := newAdaptiveTokenBucket(100)

	// Initial retrieve
	avail, ok := b.Retrieve(10)
	if !ok {
		t.Fatalf("expect tokens to be retrieved")
	}
	if e, a := 90., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	avail, ok = b.Retrieve(91)
	if ok {
		t.Fatalf("expect no tokens to be retrieved")
	}
	if e, a := 90., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	// Refunding
	b.Refund(1)

	// Retrieve all
	avail, ok = b.Retrieve(92)
	if ok {
		t.Fatalf("expect no tokens to be retrieved")
	}
	if e, a := 91., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}
	avail, ok = b.Retrieve(91)
	if !ok {
		t.Fatalf("expect tokens to be retrieved")
	}
	if e, a := 0., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	// Retrieve empty
	avail, ok = b.Retrieve(1)
	if ok {
		t.Fatalf("expect no tokens to be retrieved")
	}
	if e, a := 0., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	// Retrieve after refund
	b.Refund(1)
	avail, ok = b.Retrieve(1)
	if !ok {
		t.Fatalf("expect tokens to be retrieved")
	}
	if e, a := 0., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	// Resize
	b.Refund(50)
	avail = b.Resize(110)
	if e, a := 50., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}
	avail, ok = b.Retrieve(1)
	if !ok {
		t.Fatalf("expect tokens to be retrieved")
	}
	if e, a := 49., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}

	avail = b.Resize(25)
	if e, a := 25., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}
	avail, ok = b.Retrieve(1)
	if !ok {
		t.Fatalf("expect tokens to be retrieved")
	}
	if e, a := 24., avail; e != a {
		t.Fatalf("expect %v available, got %v", e, a)
	}
}

func TestAdaptiveTokenBucketParallel(t *testing.T) {
	bucket := newAdaptiveTokenBucket(100)
	var wg sync.WaitGroup
	wg.Add(3)

	count := 1000
	go func() {
		for i := 0; i < count; i++ {
			bucket.Retrieve(1)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < count; i++ {
			bucket.Refund(1)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < count; i++ {
			bucket.Resize(float64(i))
		}
		wg.Done()
	}()

	wg.Wait()
}
