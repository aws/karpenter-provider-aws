// +build !race

// When -race is enabled, sync.Pool is a no-op,
// which will cause these tests to fail
// and these benchmarks to be misleading.

package intern

import (
	"bytes"
	"reflect"
	"testing"
	"unsafe"
)

func TestString(t *testing.T) {
	s := "abcde"
	sub := String(s[1:4])
	interned := String("bcd")
	want := (*reflect.StringHeader)(unsafe.Pointer(&sub)).Data
	got := (*reflect.StringHeader)(unsafe.Pointer(&interned)).Data
	if want != got {
		t.Errorf("failed to intern string")
	}
}

func TestBytes(t *testing.T) {
	s := bytes.Repeat([]byte("abc"), 100)
	n := testing.AllocsPerRun(100, func() {
		for i := 0; i < 100; i++ {
			_ = Bytes(s[i*len("abc") : (i+1)*len("abc")])
		}
	})
	if n > 0 {
		t.Errorf("Bytes allocated %d, want 0", int(n))
	}
}

func BenchmarkString(b *testing.B) {
	in := "hello brad"
	b.ReportAllocs()
	b.SetBytes(int64(len(in)))
	b.RunParallel(func(pb *testing.PB) {
		var s string
		for pb.Next() {
			s = String(in[1:5])
		}
		_ = s
	})
}

func BenchmarkBytes(b *testing.B) {
	in := []byte("hello brad")
	b.ReportAllocs()
	b.SetBytes(int64(len(in)))
	b.RunParallel(func(pb *testing.PB) {
		var s string
		for pb.Next() {
			s = Bytes(in[1:5])
		}
		_ = s
	})
}
