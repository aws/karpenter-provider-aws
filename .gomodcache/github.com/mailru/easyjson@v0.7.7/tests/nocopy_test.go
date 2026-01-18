package tests

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/mailru/easyjson"
)

// verifies if string pointer belongs to the given buffer or outside of it
func strBelongsTo(s string, buf []byte) bool {
	sPtr := (*reflect.StringHeader)(unsafe.Pointer(&s)).Data
	bufPtr := (*reflect.SliceHeader)(unsafe.Pointer(&buf)).Data

	if bufPtr <= sPtr && sPtr < bufPtr+uintptr(len(buf)) {
		return true
	}
	return false
}

func TestNocopy(t *testing.T) {
	data := []byte(`{"a": "valueA", "b": "valueB"}`)
	exp := NocopyStruct{
		A: "valueA",
		B: "valueB",
	}
	res := NocopyStruct{}

	err := easyjson.Unmarshal(data, &res)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(exp, res) {
		t.Errorf("TestNocopy(): got=%+v, exp=%+v", res, exp)
	}

	if strBelongsTo(res.A, data) {
		t.Error("TestNocopy(): field A was not copied and refers to buffer")
	}
	if !strBelongsTo(res.B, data) {
		t.Error("TestNocopy(): field B was copied rather than refer to bufferr")
	}

	data = []byte(`{"b": "valueNoCopy"}`)
	res = NocopyStruct{}
	allocsPerRun := testing.AllocsPerRun(1000, func() {
		err := easyjson.Unmarshal(data, &res)
		if err != nil {
			t.Error(err)
		}
		if res.B != "valueNoCopy" {
			t.Fatalf("wrong value: %q", res.B)
		}
	})
	if allocsPerRun != 1 {
		t.Fatalf("noCopy field unmarshal: expected 1 allocs, got %f", allocsPerRun)
	}

	data = []byte(`{"a": "valueNoCopy"}`)
	allocsPerRun = testing.AllocsPerRun(1000, func() {
		err := easyjson.Unmarshal(data, &res)
		if err != nil {
			t.Error(err)
		}
		if res.A != "valueNoCopy" {
			t.Fatalf("wrong value: %q", res.A)
		}
	})
	if allocsPerRun != 2 {
		t.Fatalf("copy field unmarshal: expected 2 allocs, got %f", allocsPerRun)
	}
}
