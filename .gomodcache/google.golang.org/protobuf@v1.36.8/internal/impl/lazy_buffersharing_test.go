// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl_test

import (
	"testing"

	mixedpb "google.golang.org/protobuf/internal/testprotos/mixed"
	"google.golang.org/protobuf/proto"
)

var enableLazy = proto.UnmarshalOptions{}
var disableLazy = proto.UnmarshalOptions{
	NoLazyDecoding: true,
}

func TestCopyTopLevelLazy(t *testing.T) {
	testCopyTopLevel(t, enableLazy)
}

func TestCopyTopLevelEager(t *testing.T) {
	testCopyTopLevel(t, disableLazy)
}

// testCopyTopLevel tests that the buffer is copied to a safe location
// when the opaque proto is the top level proto
func testCopyTopLevel(t *testing.T, unmarshalOpts proto.UnmarshalOptions) {
	m := mixedpb.OpaqueLazy_builder{
		Opaque: mixedpb.OpaqueLazy_builder{
			OptionalInt32: proto.Int32(23),
		}.Build(),
	}.Build()
	if got, want := m.GetOpaque().GetOptionalInt32(), int32(23); got != want {
		t.Errorf("Build(): unexpected optional_int32: got %v, want %v", got, want)
	}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Could not marshal healthy proto %v.", m)
	}
	m2 := &mixedpb.OpaqueLazy{}
	if err := unmarshalOpts.Unmarshal(b, m2); err != nil {
		t.Fatalf("Could not unmarshal healthy proto buffer: %v.", b)
	}
	for i := 0; i < len(b); i++ {
		b[i] = byte(0xFF)
	}
	if got, want := m2.GetOpaque().GetOptionalInt32(), int32(23); got != want {
		t.Errorf("Mixed proto referred to shared buffer: got %v, want %v", got, want)
	}
}

func TestCopyWhenContainedInOpenLazy(t *testing.T) {
	testCopyWhenContainedInOpen(t, enableLazy)
}

func TestCopyWhenContainedInOpenEager(t *testing.T) {
	testCopyWhenContainedInOpen(t, disableLazy)
}

// testCopyWhenContainedInOpen tests that the buffer is copied
// for opaque messages that are not on the top level
func testCopyWhenContainedInOpen(t *testing.T, unmarshalOpts proto.UnmarshalOptions) {
	m := &mixedpb.OpenLazy{
		Opaque: mixedpb.OpaqueLazy_builder{
			Opaque: mixedpb.OpaqueLazy_builder{
				OptionalInt32: proto.Int32(23),
			}.Build(),
		}.Build(),
	}
	if got, want := m.GetOpaque().GetOpaque().GetOptionalInt32(), int32(23); got != want {
		t.Errorf("Build(): unexpected optional_int32: got %v, want %v", got, want)
	}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Could not marshal healthy proto %v.", m)
	}
	m2 := &mixedpb.OpenLazy{}
	if err := unmarshalOpts.Unmarshal(b, m2); err != nil {
		t.Fatalf("Could not unmarshal healthy proto buffer: %v.", b)
	}
	for i := 0; i < len(b); i++ {
		b[i] = byte(0xFF)
	}
	if got, want := m2.GetOpaque().GetOpaque().GetOptionalInt32(), int32(23); got != want {
		t.Errorf("Build(): unexpected optional_int32: got %v, want %v", got, want)
	}
}

func TestNoExcessiveCopyLazy(t *testing.T) {
	testNoExcessiveCopy(t, enableLazy)
}

func TestNoExcessiveCopyEager(t *testing.T) {
	testNoExcessiveCopy(t, disableLazy)
}

// testNoExcessiveCopy tests that an opaque submessage does share the buffer
// if the message above already got it copied
func testNoExcessiveCopy(t *testing.T, unmarshalOpts proto.UnmarshalOptions) {
	m := &mixedpb.OpenLazy{
		Opaque: mixedpb.OpaqueLazy_builder{
			Opaque: mixedpb.OpaqueLazy_builder{
				OptionalInt32: proto.Int32(23),
			}.Build(),
		}.Build(),
	}
	if got, want := m.GetOpaque().GetOpaque().GetOptionalInt32(), int32(23); got != want {
		t.Errorf("Build(): unexpected optional_int32: got %v, want %v", got, want)
	}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Could not marshal healthy proto %v.", m)
	}
	mm := &mixedpb.OpenLazy{}
	if err := unmarshalOpts.Unmarshal(b, mm); err != nil {
		t.Fatalf("Could not unmarshal healthy proto buffer: %v.", b)
	}
	m2 := mm.GetOpaque()
	m3 := mm.GetOpaque().GetOpaque()
	// Now, if we deliberately destroy the OpaqueM2 buffer, the OpaqueM3 buffer should
	// be destroyed as well
	if m2.XXX_lazyUnmarshalInfo == nil {
		if m3.XXX_lazyUnmarshalInfo != nil {
			t.Errorf("Inconsistent lazyUnmarshalInfo for subprotos")
		}
		// nothing to check, we don't have backing store
		return
	}

	if m3.XXX_lazyUnmarshalInfo == nil {
		t.Errorf("Inconsistent lazyUnmarshalInfo for subprotos (2)")
		return
	}
	b = (*m2.XXX_lazyUnmarshalInfo).Protobuf
	m2len := len(b)
	for i := 0; i < len(b); i++ {
		b[i] = byte(0xFF)
	}
	b = (*m3.XXX_lazyUnmarshalInfo).Protobuf
	if m2len != 0 && len(b) == 0 {
		t.Errorf("The lazy backing store for submessage is empty when it is not for the surronding message: %v.", m2len)
	}
	for i, x := range b {
		if x != byte(0xFF) {
			t.Errorf("Backing store for protocol buffer is not shared (index = %d, x = 0x%x)", i, x)
		}
	}

}
