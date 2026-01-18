// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflection_test

import (
	"testing"

	testopaquepb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_opaque"
	"google.golang.org/protobuf/proto"
)

func TestOpaqueRepeated(t *testing.T) {
	m := testopaquepb.TestAllTypes_builder{
		RepeatedNestedMessage: []*testopaquepb.TestAllTypes_NestedMessage{
			testopaquepb.TestAllTypes_NestedMessage_builder{
				A: proto.Int32(42),
			}.Build(),
		},
	}.Build()

	// Clear the repeated_nested_message field. This should not clear the presence bit.
	mr := m.ProtoReflect()
	fd := mr.Descriptor().Fields().ByNumber(48)
	mr.Clear(fd)
	if len(m.GetRepeatedNestedMessage()) != 0 {
		t.Errorf("protoreflect Clear did not empty the repeated field: got %v, expected []", m.GetRepeatedNestedMessage())
	}

	// Append a new submessage to the input field and set its A field to 23.
	dst := mr.Mutable(fd).List()
	v := dst.NewElement()
	dst.Append(v)
	if len(m.GetRepeatedNestedMessage()) != 1 {
		t.Fatalf("unexpected number of elements in repeated field: got %v, expected 1", len(m.GetRepeatedNestedMessage()))
	}
	m.GetRepeatedNestedMessage()[0].SetA(23)

	if mr.Get(fd).List().Len() != 1 {
		t.Fatalf("presence bit (incorrectly) cleared")
	}
}
