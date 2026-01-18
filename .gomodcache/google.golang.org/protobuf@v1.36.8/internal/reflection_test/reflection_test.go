// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflection_test

import (
	"fmt"
	"reflect"
	"testing"

	testopenpb "google.golang.org/protobuf/internal/testprotos/testeditions"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/testing/prototest"
)

func Test(t *testing.T) {
	t.Skip()
	for _, m := range []protoreflect.ProtoMessage{
		&testopenpb.TestAllTypes{},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			prototest.Message{}.Test(t, m.ProtoReflect().Type())
		})
	}
}

// What follows is infrastructure for a complicated but useful set of tests
// of different views of a message.
//
// Every Protobuf message can be accessed in at least two ways:
//
//   - m:                a concrete open, hybrid or opaque message
//   - m.ProtoReflect(): reflective view of the message
//
// A mutation to one representation must be reflected in the others.
//
// To test the various views of a message, we construct an implementations of
// the protoreflect.Message interface for each. The simplest is the canonical
// reflective view provided by the ProtoReflect method. In addition, for each
// concrete representation we create another view backed by that concrete API.
// (i.e., m.ProtoReflect().KnownFields().Get(1) directly translates to a call
// to m.GetFieldOne().)
//
// Finally, we construct a "shadow" view in which read operations are backed
// by one implementation and write operations by another.
//
// Each of these various views may then be passed to the prototest package
// for validation.
//
// This approach separates the decision of what behaviors to test from the
// implementations being tested; new validation tests added to prototest
// apply to all the various views without additional effort. The disadvantage
// is that there is quite a bit of per-message boilerplate required.
//
// We could attempt to reduce that boilerplate by use of reflection or code
// generation, but both approaches replace simple-but-repetitive code with
// something quite complex. Since the purpose of all this is to test the
// complex, general-purpose canonical implementation, the simple approach
// is safer.

// Field numbers for the test messages.
var (
	largeMessageDesc protoreflect.MessageDescriptor = (&testopenpb.TestManyMessageFieldsMessage{}).ProtoReflect().Descriptor()

	largeFieldF1   = largeMessageDesc.Fields().ByName("f1").Number()
	largeFieldF2   = largeMessageDesc.Fields().ByName("f2").Number()
	largeFieldF3   = largeMessageDesc.Fields().ByName("f3").Number()
	largeFieldF4   = largeMessageDesc.Fields().ByName("f4").Number()
	largeFieldF5   = largeMessageDesc.Fields().ByName("f5").Number()
	largeFieldF6   = largeMessageDesc.Fields().ByName("f6").Number()
	largeFieldF7   = largeMessageDesc.Fields().ByName("f7").Number()
	largeFieldF8   = largeMessageDesc.Fields().ByName("f8").Number()
	largeFieldF9   = largeMessageDesc.Fields().ByName("f9").Number()
	largeFieldF10  = largeMessageDesc.Fields().ByName("f10").Number()
	largeFieldF11  = largeMessageDesc.Fields().ByName("f11").Number()
	largeFieldF12  = largeMessageDesc.Fields().ByName("f12").Number()
	largeFieldF13  = largeMessageDesc.Fields().ByName("f13").Number()
	largeFieldF14  = largeMessageDesc.Fields().ByName("f14").Number()
	largeFieldF15  = largeMessageDesc.Fields().ByName("f15").Number()
	largeFieldF16  = largeMessageDesc.Fields().ByName("f16").Number()
	largeFieldF17  = largeMessageDesc.Fields().ByName("f17").Number()
	largeFieldF18  = largeMessageDesc.Fields().ByName("f18").Number()
	largeFieldF19  = largeMessageDesc.Fields().ByName("f19").Number()
	largeFieldF20  = largeMessageDesc.Fields().ByName("f20").Number()
	largeFieldF21  = largeMessageDesc.Fields().ByName("f21").Number()
	largeFieldF22  = largeMessageDesc.Fields().ByName("f22").Number()
	largeFieldF23  = largeMessageDesc.Fields().ByName("f23").Number()
	largeFieldF24  = largeMessageDesc.Fields().ByName("f24").Number()
	largeFieldF25  = largeMessageDesc.Fields().ByName("f25").Number()
	largeFieldF26  = largeMessageDesc.Fields().ByName("f26").Number()
	largeFieldF27  = largeMessageDesc.Fields().ByName("f27").Number()
	largeFieldF28  = largeMessageDesc.Fields().ByName("f28").Number()
	largeFieldF29  = largeMessageDesc.Fields().ByName("f29").Number()
	largeFieldF30  = largeMessageDesc.Fields().ByName("f30").Number()
	largeFieldF31  = largeMessageDesc.Fields().ByName("f31").Number()
	largeFieldF32  = largeMessageDesc.Fields().ByName("f32").Number()
	largeFieldF33  = largeMessageDesc.Fields().ByName("f33").Number()
	largeFieldF34  = largeMessageDesc.Fields().ByName("f34").Number()
	largeFieldF35  = largeMessageDesc.Fields().ByName("f35").Number()
	largeFieldF36  = largeMessageDesc.Fields().ByName("f36").Number()
	largeFieldF37  = largeMessageDesc.Fields().ByName("f37").Number()
	largeFieldF38  = largeMessageDesc.Fields().ByName("f38").Number()
	largeFieldF39  = largeMessageDesc.Fields().ByName("f39").Number()
	largeFieldF40  = largeMessageDesc.Fields().ByName("f40").Number()
	largeFieldF41  = largeMessageDesc.Fields().ByName("f41").Number()
	largeFieldF42  = largeMessageDesc.Fields().ByName("f42").Number()
	largeFieldF43  = largeMessageDesc.Fields().ByName("f43").Number()
	largeFieldF44  = largeMessageDesc.Fields().ByName("f44").Number()
	largeFieldF45  = largeMessageDesc.Fields().ByName("f45").Number()
	largeFieldF46  = largeMessageDesc.Fields().ByName("f46").Number()
	largeFieldF47  = largeMessageDesc.Fields().ByName("f47").Number()
	largeFieldF48  = largeMessageDesc.Fields().ByName("f48").Number()
	largeFieldF49  = largeMessageDesc.Fields().ByName("f49").Number()
	largeFieldF50  = largeMessageDesc.Fields().ByName("f50").Number()
	largeFieldF51  = largeMessageDesc.Fields().ByName("f51").Number()
	largeFieldF52  = largeMessageDesc.Fields().ByName("f52").Number()
	largeFieldF53  = largeMessageDesc.Fields().ByName("f53").Number()
	largeFieldF54  = largeMessageDesc.Fields().ByName("f54").Number()
	largeFieldF55  = largeMessageDesc.Fields().ByName("f55").Number()
	largeFieldF56  = largeMessageDesc.Fields().ByName("f56").Number()
	largeFieldF57  = largeMessageDesc.Fields().ByName("f57").Number()
	largeFieldF58  = largeMessageDesc.Fields().ByName("f58").Number()
	largeFieldF59  = largeMessageDesc.Fields().ByName("f59").Number()
	largeFieldF60  = largeMessageDesc.Fields().ByName("f60").Number()
	largeFieldF61  = largeMessageDesc.Fields().ByName("f61").Number()
	largeFieldF62  = largeMessageDesc.Fields().ByName("f62").Number()
	largeFieldF63  = largeMessageDesc.Fields().ByName("f63").Number()
	largeFieldF64  = largeMessageDesc.Fields().ByName("f64").Number()
	largeFieldF65  = largeMessageDesc.Fields().ByName("f65").Number()
	largeFieldF66  = largeMessageDesc.Fields().ByName("f66").Number()
	largeFieldF67  = largeMessageDesc.Fields().ByName("f67").Number()
	largeFieldF68  = largeMessageDesc.Fields().ByName("f68").Number()
	largeFieldF69  = largeMessageDesc.Fields().ByName("f69").Number()
	largeFieldF70  = largeMessageDesc.Fields().ByName("f70").Number()
	largeFieldF71  = largeMessageDesc.Fields().ByName("f71").Number()
	largeFieldF72  = largeMessageDesc.Fields().ByName("f72").Number()
	largeFieldF73  = largeMessageDesc.Fields().ByName("f73").Number()
	largeFieldF74  = largeMessageDesc.Fields().ByName("f74").Number()
	largeFieldF75  = largeMessageDesc.Fields().ByName("f75").Number()
	largeFieldF76  = largeMessageDesc.Fields().ByName("f76").Number()
	largeFieldF77  = largeMessageDesc.Fields().ByName("f77").Number()
	largeFieldF78  = largeMessageDesc.Fields().ByName("f78").Number()
	largeFieldF79  = largeMessageDesc.Fields().ByName("f79").Number()
	largeFieldF80  = largeMessageDesc.Fields().ByName("f80").Number()
	largeFieldF81  = largeMessageDesc.Fields().ByName("f81").Number()
	largeFieldF82  = largeMessageDesc.Fields().ByName("f82").Number()
	largeFieldF83  = largeMessageDesc.Fields().ByName("f83").Number()
	largeFieldF84  = largeMessageDesc.Fields().ByName("f84").Number()
	largeFieldF85  = largeMessageDesc.Fields().ByName("f85").Number()
	largeFieldF86  = largeMessageDesc.Fields().ByName("f86").Number()
	largeFieldF87  = largeMessageDesc.Fields().ByName("f87").Number()
	largeFieldF88  = largeMessageDesc.Fields().ByName("f88").Number()
	largeFieldF89  = largeMessageDesc.Fields().ByName("f89").Number()
	largeFieldF90  = largeMessageDesc.Fields().ByName("f90").Number()
	largeFieldF91  = largeMessageDesc.Fields().ByName("f91").Number()
	largeFieldF92  = largeMessageDesc.Fields().ByName("f92").Number()
	largeFieldF93  = largeMessageDesc.Fields().ByName("f93").Number()
	largeFieldF94  = largeMessageDesc.Fields().ByName("f94").Number()
	largeFieldF95  = largeMessageDesc.Fields().ByName("f95").Number()
	largeFieldF96  = largeMessageDesc.Fields().ByName("f96").Number()
	largeFieldF97  = largeMessageDesc.Fields().ByName("f97").Number()
	largeFieldF98  = largeMessageDesc.Fields().ByName("f98").Number()
	largeFieldF99  = largeMessageDesc.Fields().ByName("f99").Number()
	largeFieldF100 = largeMessageDesc.Fields().ByName("f100").Number()
)

var (
	messageDesc protoreflect.MessageDescriptor = (&testopenpb.TestAllTypes{}).ProtoReflect().Descriptor()

	fieldSingularInt32       = messageDesc.Fields().ByName("singular_int32").Number()
	fieldSingularInt64       = messageDesc.Fields().ByName("singular_int64").Number()
	fieldSingularUint32      = messageDesc.Fields().ByName("singular_uint32").Number()
	fieldSingularUint64      = messageDesc.Fields().ByName("singular_uint64").Number()
	fieldSingularSint32      = messageDesc.Fields().ByName("singular_sint32").Number()
	fieldSingularSint64      = messageDesc.Fields().ByName("singular_sint64").Number()
	fieldSingularFixed32     = messageDesc.Fields().ByName("singular_fixed32").Number()
	fieldSingularFixed64     = messageDesc.Fields().ByName("singular_fixed64").Number()
	fieldSingularSfixed32    = messageDesc.Fields().ByName("singular_sfixed32").Number()
	fieldSingularSfixed64    = messageDesc.Fields().ByName("singular_sfixed64").Number()
	fieldSingularFloat       = messageDesc.Fields().ByName("singular_float").Number()
	fieldSingularDouble      = messageDesc.Fields().ByName("singular_double").Number()
	fieldSingularBool        = messageDesc.Fields().ByName("singular_bool").Number()
	fieldSingularString      = messageDesc.Fields().ByName("singular_string").Number()
	fieldSingularBytes       = messageDesc.Fields().ByName("singular_bytes").Number()
	fieldSingularNestedEnum  = messageDesc.Fields().ByName("singular_nested_enum").Number()
	fieldSingularForeignEnum = messageDesc.Fields().ByName("singular_foreign_enum").Number()
	fieldSingularImportEnum  = messageDesc.Fields().ByName("singular_import_enum").Number()

	fieldOptionalInt32             = messageDesc.Fields().ByName("optional_int32").Number()
	fieldOptionalInt64             = messageDesc.Fields().ByName("optional_int64").Number()
	fieldOptionalUint32            = messageDesc.Fields().ByName("optional_uint32").Number()
	fieldOptionalUint64            = messageDesc.Fields().ByName("optional_uint64").Number()
	fieldOptionalSint32            = messageDesc.Fields().ByName("optional_sint32").Number()
	fieldOptionalSint64            = messageDesc.Fields().ByName("optional_sint64").Number()
	fieldOptionalFixed32           = messageDesc.Fields().ByName("optional_fixed32").Number()
	fieldOptionalFixed64           = messageDesc.Fields().ByName("optional_fixed64").Number()
	fieldOptionalSfixed32          = messageDesc.Fields().ByName("optional_sfixed32").Number()
	fieldOptionalSfixed64          = messageDesc.Fields().ByName("optional_sfixed64").Number()
	fieldOptionalFloat             = messageDesc.Fields().ByName("optional_float").Number()
	fieldOptionalDouble            = messageDesc.Fields().ByName("optional_double").Number()
	fieldOptionalBool              = messageDesc.Fields().ByName("optional_bool").Number()
	fieldOptionalString            = messageDesc.Fields().ByName("optional_string").Number()
	fieldOptionalBytes             = messageDesc.Fields().ByName("optional_bytes").Number()
	fieldOptionalGroup             = messageDesc.Fields().ByName("optionalgroup").Number()
	fieldOptionalNestedMessage     = messageDesc.Fields().ByName("optional_nested_message").Number()
	fieldOptionalForeignMessage    = messageDesc.Fields().ByName("optional_foreign_message").Number()
	fieldOptionalImportMessage     = messageDesc.Fields().ByName("optional_import_message").Number()
	fieldOptionalNestedEnum        = messageDesc.Fields().ByName("optional_nested_enum").Number()
	fieldOptionalForeignEnum       = messageDesc.Fields().ByName("optional_foreign_enum").Number()
	fieldOptionalImportEnum        = messageDesc.Fields().ByName("optional_import_enum").Number()
	fieldOptionalLazyNestedMessage = messageDesc.Fields().ByName("optional_lazy_nested_message").Number()
	fieldNotGroupLikeDelimited     = messageDesc.Fields().ByName("not_group_like_delimited").Number()

	fieldRepeatedInt32          = messageDesc.Fields().ByName("repeated_int32").Number()
	fieldRepeatedInt64          = messageDesc.Fields().ByName("repeated_int64").Number()
	fieldRepeatedUint32         = messageDesc.Fields().ByName("repeated_uint32").Number()
	fieldRepeatedUint64         = messageDesc.Fields().ByName("repeated_uint64").Number()
	fieldRepeatedSint32         = messageDesc.Fields().ByName("repeated_sint32").Number()
	fieldRepeatedSint64         = messageDesc.Fields().ByName("repeated_sint64").Number()
	fieldRepeatedFixed32        = messageDesc.Fields().ByName("repeated_fixed32").Number()
	fieldRepeatedFixed64        = messageDesc.Fields().ByName("repeated_fixed64").Number()
	fieldRepeatedSfixed32       = messageDesc.Fields().ByName("repeated_sfixed32").Number()
	fieldRepeatedSfixed64       = messageDesc.Fields().ByName("repeated_sfixed64").Number()
	fieldRepeatedFloat          = messageDesc.Fields().ByName("repeated_float").Number()
	fieldRepeatedDouble         = messageDesc.Fields().ByName("repeated_double").Number()
	fieldRepeatedBool           = messageDesc.Fields().ByName("repeated_bool").Number()
	fieldRepeatedString         = messageDesc.Fields().ByName("repeated_string").Number()
	fieldRepeatedBytes          = messageDesc.Fields().ByName("repeated_bytes").Number()
	fieldRepeatedGroup          = messageDesc.Fields().ByName("repeatedgroup").Number()
	fieldRepeatedNestedMessage  = messageDesc.Fields().ByName("repeated_nested_message").Number()
	fieldRepeatedForeignMessage = messageDesc.Fields().ByName("repeated_foreign_message").Number()
	fieldRepeatedImportMessage  = messageDesc.Fields().ByName("repeated_importmessage").Number()
	fieldRepeatedNestedEnum     = messageDesc.Fields().ByName("repeated_nested_enum").Number()
	fieldRepeatedForeignEnum    = messageDesc.Fields().ByName("repeated_foreign_enum").Number()
	fieldRepeatedImportEnum     = messageDesc.Fields().ByName("repeated_importenum").Number()

	fieldMapInt32Int32          = messageDesc.Fields().ByName("map_int32_int32").Number()
	fieldMapInt64Int64          = messageDesc.Fields().ByName("map_int64_int64").Number()
	fieldMapUint32Uint32        = messageDesc.Fields().ByName("map_uint32_uint32").Number()
	fieldMapUint64Uint64        = messageDesc.Fields().ByName("map_uint64_uint64").Number()
	fieldMapSint32Sint32        = messageDesc.Fields().ByName("map_sint32_sint32").Number()
	fieldMapSint64Sint64        = messageDesc.Fields().ByName("map_sint64_sint64").Number()
	fieldMapFixed32Fixed32      = messageDesc.Fields().ByName("map_fixed32_fixed32").Number()
	fieldMapFixed64Fixed64      = messageDesc.Fields().ByName("map_fixed64_fixed64").Number()
	fieldMapSfixed32Sfixed32    = messageDesc.Fields().ByName("map_sfixed32_sfixed32").Number()
	fieldMapSfixed64Sfixed64    = messageDesc.Fields().ByName("map_sfixed64_sfixed64").Number()
	fieldMapInt32Float          = messageDesc.Fields().ByName("map_int32_float").Number()
	fieldMapInt32Double         = messageDesc.Fields().ByName("map_int32_double").Number()
	fieldMapBoolBool            = messageDesc.Fields().ByName("map_bool_bool").Number()
	fieldMapStringString        = messageDesc.Fields().ByName("map_string_string").Number()
	fieldMapStringBytes         = messageDesc.Fields().ByName("map_string_bytes").Number()
	fieldMapStringNestedMessage = messageDesc.Fields().ByName("map_string_nested_message").Number()
	fieldMapStringNestedEnum    = messageDesc.Fields().ByName("map_string_nested_enum").Number()

	fieldDefaultInt32       = messageDesc.Fields().ByName("default_int32").Number()
	fieldDefaultInt64       = messageDesc.Fields().ByName("default_int64").Number()
	fieldDefaultUint32      = messageDesc.Fields().ByName("default_uint32").Number()
	fieldDefaultUint64      = messageDesc.Fields().ByName("default_uint64").Number()
	fieldDefaultSint32      = messageDesc.Fields().ByName("default_sint32").Number()
	fieldDefaultSint64      = messageDesc.Fields().ByName("default_sint64").Number()
	fieldDefaultFixed32     = messageDesc.Fields().ByName("default_fixed32").Number()
	fieldDefaultFixed64     = messageDesc.Fields().ByName("default_fixed64").Number()
	fieldDefaultSfixed32    = messageDesc.Fields().ByName("default_sfixed32").Number()
	fieldDefaultSfixed64    = messageDesc.Fields().ByName("default_sfixed64").Number()
	fieldDefaultFloat       = messageDesc.Fields().ByName("default_float").Number()
	fieldDefaultDouble      = messageDesc.Fields().ByName("default_double").Number()
	fieldDefaultBool        = messageDesc.Fields().ByName("default_bool").Number()
	fieldDefaultString      = messageDesc.Fields().ByName("default_string").Number()
	fieldDefaultBytes       = messageDesc.Fields().ByName("default_bytes").Number()
	fieldDefaultNestedEnum  = messageDesc.Fields().ByName("default_nested_enum").Number()
	fieldDefaultForeignEnum = messageDesc.Fields().ByName("default_foreign_enum").Number()

	fieldOneofUint32         = messageDesc.Fields().ByName("oneof_uint32").Number()
	fieldOneofNestedMessage  = messageDesc.Fields().ByName("oneof_nested_message").Number()
	fieldOneofString         = messageDesc.Fields().ByName("oneof_string").Number()
	fieldOneofBytes          = messageDesc.Fields().ByName("oneof_bytes").Number()
	fieldOneofBool           = messageDesc.Fields().ByName("oneof_bool").Number()
	fieldOneofUint64         = messageDesc.Fields().ByName("oneof_uint64").Number()
	fieldOneofFloat          = messageDesc.Fields().ByName("oneof_float").Number()
	fieldOneofDouble         = messageDesc.Fields().ByName("oneof_double").Number()
	fieldOneofEnum           = messageDesc.Fields().ByName("oneof_enum").Number()
	fieldOneofGroup          = messageDesc.Fields().ByName("oneofgroup").Number()
	fieldOneofOptionalUint32 = messageDesc.Fields().ByName("oneof_optional_uint32").Number()
)

// testMessageType is an implementation of protoreflect.MessageType.
type testMessageType struct {
	protoreflect.MessageDescriptor
	new func() protoreflect.Message
}

func (m *testMessageType) New() protoreflect.Message                  { return m.new() }
func (m *testMessageType) Zero() protoreflect.Message                 { return m.new() }
func (m *testMessageType) GoType() reflect.Type                       { panic("unimplemented") }
func (m *testMessageType) Descriptor() protoreflect.MessageDescriptor { return m.MessageDescriptor }

// testProtoMessage adapts the concrete API for a message to the ProtoReflect interface.
type testProtoMessage struct {
	m     protoreflect.ProtoMessage
	md    protoreflect.MessageDescriptor
	new   func() protoreflect.Message
	has   func(protoreflect.FieldNumber) bool
	get   func(protoreflect.FieldNumber) any
	set   func(protoreflect.FieldNumber, any)
	clear func(protoreflect.FieldNumber)
}

func (m *testProtoMessage) ProtoReflect() protoreflect.Message { return (*testMessage)(m) }

// testMessage implements protoreflect.Message.
type testMessage testProtoMessage

func (m *testMessage) Interface() protoreflect.ProtoMessage       { return (*testProtoMessage)(m) }
func (m *testMessage) ProtoMethods() *protoiface.Methods          { return nil }
func (m *testMessage) Descriptor() protoreflect.MessageDescriptor { return m.md }
func (m *testMessage) Type() protoreflect.MessageType             { return &testMessageType{m.md, m.new} }
func (m *testMessage) New() protoreflect.Message                  { return m.new() }
func (m *testMessage) Range(f func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	fields := m.md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !m.Has(fd) {
			continue
		}
		if !f(fd, m.Get(fd)) {
			break
		}
	}
}
func (m *testMessage) Has(fd protoreflect.FieldDescriptor) bool {
	return m.has(fd.Number())
}
func (m *testMessage) Clear(fd protoreflect.FieldDescriptor) {
	m.clear(fd.Number())
}
func (m *testMessage) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	num := fd.Number()
	switch {
	case fd.IsMap():
		if !m.has(num) {
			return protoreflect.ValueOfMap(&testMap{reflect.Zero(reflect.TypeOf(m.get(num))), fd})
		}
		return protoreflect.ValueOfMap(&testMap{reflect.ValueOf(m.get(num)), fd})
	case fd.IsList():
		if !m.has(num) {
			return protoreflect.ValueOfList(&zeroList{reflect.TypeOf(m.get(num)).Elem(), fd})
		}
		return protoreflect.ValueOfList(&testList{m: (*testProtoMessage)(m), fd: fd})
	case fd.Message() != nil:
		if !m.has(fd.Number()) {
			return protoreflect.Value{}
		}
	}
	return singularValueOf(m.get(num))
}
func (m *testMessage) Set(fd protoreflect.FieldDescriptor, v protoreflect.Value) {
	num := fd.Number()
	switch {
	case fd.IsMap():
		if !v.Map().IsValid() {
			panic("set with invalid map")
		}
		m.set(num, v.Map().(*testMap).val.Interface())
	case fd.IsList():
		if !v.List().IsValid() {
			panic("set with invalid list")
		}
		m.set(num, v.List().(*testList).field().Interface())
	case fd.Message() != nil:
		i := v.Message().Interface()
		if p, ok := i.(*testProtoMessage); ok {
			i = p.m
		}
		m.set(num, i)
	default:
		m.set(num, v.Interface())
	}
}
func (m *testMessage) Mutable(fd protoreflect.FieldDescriptor) protoreflect.Value {
	num := fd.Number()
	if !m.Has(fd) && (fd.IsMap() || fd.IsList() || fd.Message() != nil) {
		switch {
		case fd.IsMap():
			typ := reflect.ValueOf(m.get(num)).Type()
			m.set(num, reflect.MakeMap(typ).Interface())
			return protoreflect.ValueOfMap(&testMap{reflect.ValueOf(m.get(num)), fd})
		case fd.IsList():
			return protoreflect.ValueOfList(&testList{m: (*testProtoMessage)(m), fd: fd})
		case fd.Message() != nil:
			typ := reflect.ValueOf(m.get(num)).Type()
			m.set(num, reflect.New(typ.Elem()).Interface())
		}
	}
	return m.Get(fd)
}
func (m *testMessage) NewMessage(fd protoreflect.FieldDescriptor) protoreflect.Message {
	return singularValueOf(m.NewField(fd)).Message()
}
func (m *testMessage) NewField(fd protoreflect.FieldDescriptor) protoreflect.Value {
	num := fd.Number()
	switch {
	case fd.IsMap():
		typ := reflect.ValueOf(m.get(num)).Type()
		return protoreflect.ValueOf(&testMap{reflect.MakeMap(typ), fd})
	case fd.IsList():
		typ := reflect.ValueOf(m.get(num)).Type()
		return protoreflect.ValueOf(&testList{val: reflect.Zero(typ), fd: fd})
	case fd.Message() != nil:
		typ := reflect.ValueOf(m.get(num)).Type()
		return singularValueOf(reflect.New(typ.Elem()).Interface())
	default:
		// Obtain the default value of the field by creating an empty message
		// and calling the getter.
		n := m.new().(*testMessage)
		return singularValueOf(n.get(num))
	}
}
func (m *testMessage) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	for i := 0; i < od.Fields().Len(); i++ {
		fd := od.Fields().Get(i)
		if m.has(fd.Number()) {
			return fd
		}
	}
	return nil
}
func (m *testMessage) GetUnknown() protoreflect.RawFields {
	return m.m.ProtoReflect().GetUnknown()
}
func (m *testMessage) SetUnknown(raw protoreflect.RawFields) {
	m.m.ProtoReflect().SetUnknown(raw)
}
func (m *testMessage) IsValid() bool {
	return !reflect.ValueOf(m.m).IsNil()
}

func singularValueOf(v any) protoreflect.Value {
	switch v := v.(type) {
	case protoreflect.ProtoMessage:
		return protoreflect.ValueOf(v.ProtoReflect())
	case protoreflect.Enum:
		return protoreflect.ValueOf(v.Number())
	default:
		return protoreflect.ValueOf(v)
	}
}

// testList implements protoreflect.List over a concrete slice of values.
type testList struct {
	m   *testProtoMessage
	val reflect.Value
	fd  protoreflect.FieldDescriptor
}

func (x *testList) field() reflect.Value {
	if x.m == nil {
		return x.val
	}
	return reflect.ValueOf(x.m.get(x.fd.Number()))
}
func (x *testList) setField(v reflect.Value) {
	if x.m == nil {
		x.val = v
		return
	}
	x.m.set(x.fd.Number(), v.Interface())
}
func (x *testList) Len() int { return x.field().Len() }
func (x *testList) Get(n int) protoreflect.Value {
	return singularValueOf(x.field().Index(n).Interface())
}
func (x *testList) Set(n int, v protoreflect.Value) {
	switch x.fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		x.field().Index(n).Set(reflect.ValueOf(v.Message().Interface()))
	case protoreflect.EnumKind:
		x.field().Index(n).SetInt(int64(v.Enum()))
	default:
		x.field().Index(n).Set(reflect.ValueOf(v.Interface()))
	}
}
func (x *testList) Append(v protoreflect.Value) {
	f := x.field()
	x.setField(reflect.Append(f, reflect.Zero(f.Type().Elem())))
	x.Set(f.Len(), v)
}
func (x *testList) AppendMutable() protoreflect.Value {
	if x.fd.Message() == nil {
		panic("invalid AppendMutable on list with non-message value type")
	}
	v := x.NewElement()
	x.Append(v)
	return v
}
func (x *testList) Truncate(n int) {
	x.setField(x.field().Slice(0, n))
}
func (x *testList) NewMessage() protoreflect.Message {
	return x.NewElement().Message()
}
func (x *testList) NewElement() protoreflect.Value {
	// For enums, List.NewElement returns the first enum value.
	if ee := newEnumElement(x.fd); ee.IsValid() {
		return ee
	}
	var v reflect.Value
	typ := x.field().Type().Elem()
	if typ.Kind() == reflect.Ptr {
		v = reflect.New(typ.Elem())
	} else {
		v = reflect.Zero(typ)
	}
	return singularValueOf(v.Interface())
}
func (x *testList) IsValid() bool {
	return true
}

func newEnumElement(fd protoreflect.FieldDescriptor) protoreflect.Value {
	if fd.Kind() != protoreflect.EnumKind {
		return protoreflect.Value{}
	}
	if val := fd.Enum().Values(); val.Len() > 0 {
		return protoreflect.ValueOfEnum(val.Get(0).Number())
	}
	return protoreflect.Value{}
}

// testList implements protoreflect.List over a concrete slice of values.
type zeroList struct {
	typ reflect.Type
	fd  protoreflect.FieldDescriptor
}

func (x *zeroList) Len() int                          { return 0 }
func (x *zeroList) Get(n int) protoreflect.Value      { panic("get on zero list") }
func (x *zeroList) Set(n int, v protoreflect.Value)   { panic("set on zero list") }
func (x *zeroList) Append(v protoreflect.Value)       { panic("append on zero list") }
func (x *zeroList) AppendMutable() protoreflect.Value { panic("append on zero list") }
func (x *zeroList) Truncate(n int)                    { panic("truncate on zero list") }
func (x *zeroList) NewMessage() protoreflect.Message {
	return x.NewElement().Message()
}
func (x *zeroList) NewElement() protoreflect.Value {
	// For enums, List.NewElement returns the first enum value.
	if ee := newEnumElement(x.fd); ee.IsValid() {
		return ee
	}
	var v reflect.Value
	if x.typ.Kind() == reflect.Ptr {
		v = reflect.New(x.typ.Elem())
	} else {
		v = reflect.Zero(x.typ)
	}
	return singularValueOf(v.Interface())
}
func (x *zeroList) IsValid() bool {
	return false
}

// testMap implements a protoreflect.Map over a concrete map.
type testMap struct {
	val reflect.Value
	fd  protoreflect.FieldDescriptor
}

func (x *testMap) key(k protoreflect.MapKey) reflect.Value { return reflect.ValueOf(k.Interface()) }
func (x *testMap) valueToProto(v reflect.Value) protoreflect.Value {
	if !v.IsValid() {
		return protoreflect.Value{}
	}
	switch x.fd.Message().Fields().ByNumber(2).Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return protoreflect.ValueOf(v.Interface().(protoreflect.ProtoMessage).ProtoReflect())
	case protoreflect.EnumKind:
		return protoreflect.ValueOf(protoreflect.EnumNumber(v.Int()))
	default:
		return protoreflect.ValueOf(v.Interface())
	}
}
func (x *testMap) Len() int                       { return x.val.Len() }
func (x *testMap) Has(k protoreflect.MapKey) bool { return x.val.MapIndex(x.key(k)).IsValid() }
func (x *testMap) Get(k protoreflect.MapKey) protoreflect.Value {
	return x.valueToProto(x.val.MapIndex(x.key(k)))
}
func (x *testMap) Set(k protoreflect.MapKey, v protoreflect.Value) {
	f := x.val
	switch x.fd.MapValue().Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		f.SetMapIndex(x.key(k), reflect.ValueOf(v.Message().Interface()))
	case protoreflect.EnumKind:
		rv := reflect.New(f.Type().Elem()).Elem()
		rv.SetInt(int64(v.Enum()))
		f.SetMapIndex(x.key(k), rv)
	default:
		f.SetMapIndex(x.key(k), reflect.ValueOf(v.Interface()))
	}
}
func (x *testMap) Mutable(k protoreflect.MapKey) protoreflect.Value {
	if x.fd.MapValue().Message() == nil {
		panic("invalid Mutable on map with non-message value type")
	}
	v := x.Get(k)
	if !v.IsValid() {
		v = x.NewValue()
		x.Set(k, v)
	}
	return v
}
func (x *testMap) Clear(k protoreflect.MapKey) { x.val.SetMapIndex(x.key(k), reflect.Value{}) }
func (x *testMap) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) {
	iter := x.val.MapRange()
	for iter.Next() {
		if !f(protoreflect.ValueOf(iter.Key().Interface()).MapKey(), x.valueToProto(iter.Value())) {
			return
		}
	}
}
func (x *testMap) NewMessage() protoreflect.Message {
	return x.NewValue().Message()
}
func (x *testMap) NewValue() protoreflect.Value {
	var v reflect.Value
	if x.fd.MapValue().Message() != nil {
		v = reflect.New(x.val.Type().Elem().Elem())
	} else {
		v = reflect.Zero(x.val.Type().Elem())
	}
	return singularValueOf(v.Interface())
}
func (x *testMap) IsValid() bool {
	return !x.val.IsNil()
}

// A shadow message is a wrapper around two protoreflect.Message implementations
// presenting different views of the same underlying data. Read operations
// are directed to one implementation and write operations to the other.

// shadowProtoMessage implements protoreflect.ProtoMessage as a shadow.
type shadowProtoMessage struct {
	get, set protoreflect.Message
	new      func() (get, set protoreflect.ProtoMessage)
}

func newShadow(newf func() (get, set protoreflect.ProtoMessage)) protoreflect.ProtoMessage {
	get, set := newf()
	return &shadowProtoMessage{
		get.ProtoReflect(),
		set.ProtoReflect(),
		newf,
	}
}

func (m *shadowProtoMessage) ProtoReflect() protoreflect.Message { return (*shadowMessage)(m) }

// shadowMessage implements protoreflect.Message as a shadow.
type shadowMessage shadowProtoMessage

func (m *shadowMessage) Interface() protoreflect.ProtoMessage       { return (*shadowProtoMessage)(m) }
func (m *shadowMessage) ProtoMethods() *protoiface.Methods          { return nil }
func (m *shadowMessage) Descriptor() protoreflect.MessageDescriptor { return m.get.Descriptor() }
func (m *shadowMessage) Type() protoreflect.MessageType {
	return &testMessageType{m.Descriptor(), m.New}
}
func (m *shadowMessage) New() protoreflect.Message {
	get, set := m.new()
	return &shadowMessage{
		get: get.ProtoReflect(),
		set: set.ProtoReflect(),
		new: m.new,
	}
}

// TODO: Implement these.
func (m *shadowMessage) Range(f func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	m.get.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		return f(fd, m.Get(fd))
	})
}
func (m *shadowMessage) Has(fd protoreflect.FieldDescriptor) bool {
	return m.get.Has(fd)
}
func (m *shadowMessage) Clear(fd protoreflect.FieldDescriptor) {
	m.set.Clear(fd)
}
func (m *shadowMessage) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	v := m.get.Get(fd)
	switch {
	case fd.IsList():
		return protoreflect.ValueOfList(&shadowList{v.List(), m.set.Get(fd).List()})
	case fd.IsMap():
		return protoreflect.ValueOfMap(&shadowMap{v.Map(), m.set.Get(fd).Map()})
	default:
		return v
	}
}
func (m *shadowMessage) Set(fd protoreflect.FieldDescriptor, v protoreflect.Value) {
	switch x := v.Interface().(type) {
	case *shadowList:
		m.set.Set(fd, protoreflect.ValueOf(x.set))
	case *shadowMap:
		m.set.Set(fd, protoreflect.ValueOf(x.set))
	default:
		m.set.Set(fd, v)
	}
}
func (m *shadowMessage) Mutable(fd protoreflect.FieldDescriptor) protoreflect.Value {
	v := m.get.Mutable(fd)
	switch {
	case fd.IsList():
		return protoreflect.ValueOf(&shadowList{v.List(), m.set.Mutable(fd).List()})
	case fd.IsMap():
		return protoreflect.ValueOf(&shadowMap{v.Map(), m.set.Mutable(fd).Map()})
	default:
		return v
	}
}
func (m *shadowMessage) NewMessage(fd protoreflect.FieldDescriptor) protoreflect.Message {
	return m.NewField(fd).Message()
}
func (m *shadowMessage) NewField(fd protoreflect.FieldDescriptor) protoreflect.Value {
	return m.set.NewField(fd)
}
func (m *shadowMessage) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	return m.get.WhichOneof(od)
}
func (m *shadowMessage) GetUnknown() protoreflect.RawFields {
	return m.get.GetUnknown()
}
func (m *shadowMessage) SetUnknown(raw protoreflect.RawFields) {
	m.set.SetUnknown(raw)
}
func (m *shadowMessage) IsValid() bool {
	return m.get.IsValid()
}

// shadowList implements protoreflect.List as a shadow.
type shadowList struct {
	get, set protoreflect.List
}

func (x *shadowList) Len() int                          { return x.get.Len() }
func (x *shadowList) Get(n int) protoreflect.Value      { return x.get.Get(n) }
func (x *shadowList) Set(n int, v protoreflect.Value)   { x.set.Set(n, v) }
func (x *shadowList) Append(v protoreflect.Value)       { x.set.Append(v) }
func (x *shadowList) AppendMutable() protoreflect.Value { return x.set.AppendMutable() }
func (x *shadowList) Truncate(n int)                    { x.set.Truncate(n) }
func (x *shadowList) NewMessage() protoreflect.Message  { return x.set.NewElement().Message() }
func (x *shadowList) NewElement() protoreflect.Value    { return x.set.NewElement() }
func (x *shadowList) IsValid() bool                     { return x.get.IsValid() }

// shadowMap implements protoreflect.Map as a shadow.
type shadowMap struct {
	get, set protoreflect.Map
}

func (x *shadowMap) Len() int                                                   { return x.get.Len() }
func (x *shadowMap) Has(k protoreflect.MapKey) bool                             { return x.get.Has(k) }
func (x *shadowMap) Get(k protoreflect.MapKey) protoreflect.Value               { return x.get.Get(k) }
func (x *shadowMap) Set(k protoreflect.MapKey, v protoreflect.Value)            { x.set.Set(k, v) }
func (x *shadowMap) Mutable(k protoreflect.MapKey) protoreflect.Value           { return x.set.Mutable(k) }
func (x *shadowMap) Clear(k protoreflect.MapKey)                                { x.set.Clear(k) }
func (x *shadowMap) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) { x.get.Range(f) }
func (x *shadowMap) NewMessage() protoreflect.Message                           { return x.set.NewValue().Message() }
func (x *shadowMap) NewValue() protoreflect.Value                               { return x.set.NewValue() }
func (x *shadowMap) IsValid() bool                                              { return x.get.IsValid() }
