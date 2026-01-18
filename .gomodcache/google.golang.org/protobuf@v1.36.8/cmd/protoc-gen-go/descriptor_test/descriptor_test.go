// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package descriptor_test

import (
	"testing"

	testopenpb "google.golang.org/protobuf/internal/testprotos/test"
	testhybridpb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_hybrid"
	testopaquepb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_opaque"
)

func TestFileModeEnum(t *testing.T) {
	var e any = testopenpb.ForeignEnum_FOREIGN_FOO
	if _, ok := e.(interface{ EnumDescriptor() ([]byte, []int) }); !ok {
		t.Errorf("Open V1 proto did not have deprecated method EnumDescriptor")
	}
	var oe any = testopaquepb.ForeignEnum_FOREIGN_FOO
	if _, ok := oe.(interface{ EnumDescriptor() ([]byte, []int) }); ok {
		t.Errorf("Opaque V0 proto did have deprecated method EnumDescriptor")
	}
	var he any = testhybridpb.ForeignEnum_FOREIGN_FOO
	if _, ok := he.(interface{ EnumDescriptor() ([]byte, []int) }); ok {
		t.Errorf("Hybrid proto did have deprecated method EnumDescriptor")
	}
}

func TestFileModeMessage(t *testing.T) {
	var p any = &testopenpb.TestAllTypes{}
	if _, ok := p.(interface{ Descriptor() ([]byte, []int) }); !ok {
		t.Errorf("Open V1 proto did not have deprecated method Descriptor")
	}
	var op any = &testopaquepb.TestAllTypes{}
	if _, ok := op.(interface{ Descriptor() ([]byte, []int) }); ok {
		t.Errorf("Opaque V0 mode proto unexpectedly has deprecated Descriptor() method")
	}
	var hp any = &testhybridpb.TestAllTypes{}
	if _, ok := hp.(interface{ EnumDescriptor() ([]byte, []int) }); ok {
		t.Errorf("Hybrid proto did have deprecated method EnumDescriptor")
	}
}
