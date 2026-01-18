// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filedesc_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	testpb "google.golang.org/protobuf/internal/testprotos/test"
	"google.golang.org/protobuf/types/descriptorpb"
)

var testFile = new(testpb.TestAllTypes).ProtoReflect().Descriptor().ParentFile()

func TestInit(t *testing.T) {
	// Compare the FileDescriptorProto for the same test file from two different sources:
	//
	// 1. The result of passing the filedesc-produced FileDescriptor through protodesc.
	// 2. The protoc-generated wire-encoded message.
	//
	// This serves as a test of both filedesc and protodesc.
	got := protodesc.ToFileDescriptorProto(testFile)

	want := &descriptorpb.FileDescriptorProto{}
	zb, _ := (&testpb.TestAllTypes{}).Descriptor()
	r, _ := gzip.NewReader(bytes.NewBuffer(zb))
	b, _ := io.ReadAll(r)
	if err := proto.Unmarshal(b, want); err != nil {
		t.Fatal(err)
	}

	if !proto.Equal(got, want) {
		t.Errorf("protodesc.ToFileDescriptorProto(testpb.Test_protoFile) is not equal to the protoc-generated FileDescriptorProto for internal/testprotos/test/test.proto")
	}

	// Verify that the test proto file provides exhaustive coverage of all descriptor fields.
	seen := make(map[protoreflect.FullName]bool)
	visitFields(want.ProtoReflect(), func(field protoreflect.FieldDescriptor) {
		seen[field.FullName()] = true
	})
	descFile := new(descriptorpb.DescriptorProto).ProtoReflect().Descriptor().ParentFile()
	descPkg := descFile.Package()
	ignore := map[protoreflect.FullName]bool{
		// The protoreflect descriptors don't include source info.
		descPkg.Append("FileDescriptorProto.source_code_info"): true,
		descPkg.Append("FileDescriptorProto.syntax"):           true,
		// Nothing is using edition yet.
		descPkg.Append("FileDescriptorProto.edition"):            true,
		descPkg.Append("FileDescriptorProto.edition_enum"):       true,
		descPkg.Append("FileDescriptorProto.edition_deprecated"): true,
		// Support for weak fields has been removed.
		descPkg.Append("FileDescriptorProto.weak_dependency"): true,

		// TODO: Test option_dependency.
		descPkg.Append("FileDescriptorProto.option_dependency"): true,

		// Visibility is enforced in protoc, runtimes should not inspect this.
		descPkg.Append("DescriptorProto.visibility"):     true,
		descPkg.Append("EnumDescriptorProto.visibility"): true,

		// Impossible to test proto3 optional in a proto2 file.
		descPkg.Append("FieldDescriptorProto.proto3_optional"): true,

		// TODO: Test oneof and extension options. Testing these requires extending the
		// options messages (because they contain no user-settable fields), but importing
		// descriptor.proto from test.proto currently causes an import cycle. Add test
		// cases when that import cycle has been fixed.
		descPkg.Append("OneofDescriptorProto.options"): true,
	}
	for _, messageName := range []protoreflect.Name{
		"FileDescriptorProto",
		"DescriptorProto",
		"FieldDescriptorProto",
		"OneofDescriptorProto",
		"EnumDescriptorProto",
		"EnumValueDescriptorProto",
	} {
		message := descFile.Messages().ByName(messageName)
		for i, fields := 0, message.Fields(); i < fields.Len(); i++ {
			if name := fields.Get(i).FullName(); !seen[name] && !ignore[name] {
				t.Errorf("No test for descriptor field: %v", name)
			}
		}
	}

	// Verify that message descriptors for map entries have no Go type info.
	mapEntryName := protoreflect.FullName("goproto.proto.test.TestAllTypes.MapInt32Int32Entry")
	d := testFile.Messages().ByName("TestAllTypes").Fields().ByName("map_int32_int32").Message()
	if gotName, wantName := d.FullName(), mapEntryName; gotName != wantName {
		t.Fatalf("looked up wrong descriptor: got %v, want %v", gotName, wantName)
	}
	if _, ok := d.(protoreflect.MessageType); ok {
		t.Errorf("message descriptor for %v must not implement protoreflect.MessageType", mapEntryName)
	}
}

// visitFields calls f for every field set in m and its children.
func visitFields(m protoreflect.Message, f func(protoreflect.FieldDescriptor)) {
	m.Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		f(fd)
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			if fd.IsList() {
				for i, list := 0, value.List(); i < list.Len(); i++ {
					visitFields(list.Get(i).Message(), f)
				}
			} else {
				visitFields(value.Message(), f)
			}
		}
		return true
	})
}
