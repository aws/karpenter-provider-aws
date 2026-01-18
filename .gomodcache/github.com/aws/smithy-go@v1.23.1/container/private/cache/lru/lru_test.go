package lru

import "testing"

func TestCache(t *testing.T) {
	cache := New(4).(*lru)

	// fill cache
	cache.Put(1, 2)
	cache.Put(2, 3)
	cache.Put(3, 4)
	cache.Put(4, 5)
	assertEntry(t, cache, 1, 2)
	assertEntry(t, cache, 2, 3)
	assertEntry(t, cache, 3, 4)
	assertEntry(t, cache, 4, 5)

	// touch the last entry
	cache.Get(1)
	cache.Put(5, 6)
	assertNoEntry(t, cache, 2)
	assertEntry(t, cache, 3, 4)
	assertEntry(t, cache, 4, 5)
	assertEntry(t, cache, 1, 2)
	assertEntry(t, cache, 5, 6)

	// put something new, 3 should now be the oldest
	cache.Put(6, 7)
	assertNoEntry(t, cache, 3)
	assertEntry(t, cache, 4, 5)
	assertEntry(t, cache, 1, 2)
	assertEntry(t, cache, 5, 6)
	assertEntry(t, cache, 6, 7)

	// touch something in the middle
	cache.Get(5)
	assertEntry(t, cache, 4, 5)
	assertEntry(t, cache, 1, 2)
	assertEntry(t, cache, 5, 6)
	assertEntry(t, cache, 6, 7)

	// put 3 new things, 5 should remain after the touch
	cache.Put(7, 8)
	cache.Put(8, 9)
	cache.Put(9, 0)
	assertNoEntry(t, cache, 4)
	assertNoEntry(t, cache, 1)
	assertNoEntry(t, cache, 6)
	assertEntry(t, cache, 5, 6)
	assertEntry(t, cache, 7, 8)
	assertEntry(t, cache, 8, 9)
	assertEntry(t, cache, 9, 0)
}

func assertEntry(t *testing.T, c *lru, k interface{}, v interface{}) {
	e, ok := c.entries[k]
	if !ok {
		t.Errorf("expected entry %v=%v, but no entry found", k, v)
	}
	if actual := e.Value.(*element).value; actual != v {
		t.Errorf("expected entry %v=%v, but got entry value %v", k, v, actual)
	}
}

func assertNoEntry(t *testing.T, c *lru, k interface{}) {
	if _, ok := c.Get(k); ok {
		t.Errorf("expected no entry for %v, but one was found", k)
	}
}
