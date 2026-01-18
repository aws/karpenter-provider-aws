package json

import (
	"bytes"
	"testing"
)

func TestArray(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	array := newArray(buffer, &scratch)
	array.Value().String("bar")
	array.Value().String("baz")
	array.Close()

	e := []byte(`["bar","baz"]`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}
