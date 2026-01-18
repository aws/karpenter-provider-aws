/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package buffer

import (
	"testing"
)

func TestGrowthGrowing(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		ring        *TypedRingGrowing[int]
		initialSize int
	}{
		"implicit-zero": {
			ring: new(TypedRingGrowing[int]),
		},
		"explicit-zero": {
			ring:        NewTypedRingGrowing[int](RingGrowingOptions{InitialSize: 0}),
			initialSize: 0,
		},
		"nonzero": {
			ring:        NewTypedRingGrowing[int](RingGrowingOptions{InitialSize: 1}),
			initialSize: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			initialSize := test.initialSize
			g := test.ring

			if expected, actual := 0, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := initialSize, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}

			x := 10
			for i := 0; i < x; i++ {
				if e, a := i, g.readable; e != a {
					t.Fatalf("expected equal, got %#v, %#v", e, a)
				}
				g.WriteOne(i)
			}

			if expected, actual := x, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := 16, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}

			read := 0
			for g.readable > 0 {
				v, ok := g.ReadOne()
				if !ok {
					t.Fatal("expected true")
				}
				if read != v {
					t.Fatalf("expected %#v==%#v", read, v)
				}
				read++
			}
			if x != read {
				t.Fatalf("expected to have read %d items: %d", x, read)
			}
			if expected, actual := 0, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := 16, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}
		})
	}

}

func TestGrowth(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ring        *Ring[int]
		initialSize int
		normalSize  int
	}{
		"implicit-zero": {
			ring: new(Ring[int]),
		},
		"explicit-zero": {
			ring:        NewRing[int](RingOptions{InitialSize: 0, NormalSize: 0}),
			initialSize: 0,
			normalSize:  0,
		},
		"smaller-initial-size": {
			ring:        NewRing[int](RingOptions{InitialSize: 1, NormalSize: 2}),
			initialSize: 1,
			normalSize:  2,
		},
		"smaller-normal-size": {
			ring:        NewRing[int](RingOptions{InitialSize: 2, NormalSize: 1}),
			initialSize: 2,
			normalSize:  1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			initialSize := test.initialSize
			normalSize := test.normalSize
			g := test.ring

			if expected, actual := 0, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := initialSize, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}

			x := 10
			for i := 0; i < x; i++ {
				if e, a := i, g.growing.readable; e != a {
					t.Fatalf("expected equal, got %#v, %#v", e, a)
				}
				g.WriteOne(i)
			}

			if expected, actual := x, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := 16, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}

			read := 0
			for g.growing.readable > 0 {
				v, ok := g.ReadOne()
				if !ok {
					t.Fatal("expected true")
				}
				if read != v {
					t.Fatalf("expected %#v==%#v", read, v)
				}
				read++
			}
			if x != read {
				t.Fatalf("expected to have read %d items: %d", x, read)
			}
			if expected, actual := 0, g.Len(); expected != actual {
				t.Fatalf("expected Len to be %d, got %d", expected, actual)
			}
			if expected, actual := normalSize, g.Cap(); expected != actual {
				t.Fatalf("expected Cap to be %d, got %d", expected, actual)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	t.Parallel()
	g := NewTypedRingGrowing[struct{}](RingGrowingOptions{InitialSize: 1})
	_, ok := g.ReadOne()
	if ok != false {
		t.Fatal("expected false")
	}
}

const (
	spikeSize   = 100 // Number of items to write during a spike
	normalSize  = 64  // Normal capacity for the Ring type after shrinking
	initialSize = 16  // Initial capacity for buffers
)

func BenchmarkTypedRingGrowing_Spike(b *testing.B) {
	b.ReportAllocs()
	var item int // ensure item is used

	for i := 0; i < b.N; i++ {
		buffer := NewTypedRingGrowing[int](RingGrowingOptions{InitialSize: initialSize})

		for j := 0; j < spikeSize; j++ {
			buffer.WriteOne(j)
		}

		for buffer.Len() > 0 {
			item, _ = buffer.ReadOne()
		}
	}
	_ = item // use item
}

func BenchmarkRing_Spike_And_Shrink(b *testing.B) {
	b.ReportAllocs()
	var item int // ensure item is used

	for i := 0; i < b.N; i++ {
		// Create a new buffer for each benchmark iteration
		buffer := NewRing[int](RingOptions{
			InitialSize: initialSize,
			NormalSize:  normalSize,
		})

		for j := 0; j < spikeSize; j++ {
			buffer.WriteOne(j)
		}

		for buffer.Len() > 0 {
			item, _ = buffer.ReadOne()
		}
	}
	_ = item // use item
}
