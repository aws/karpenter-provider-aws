// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflection_test

import (
	"fmt"
	"math"
	"testing"

	testpb "google.golang.org/protobuf/internal/testprotos/testeditions"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/prototest"
)

func TestOpenConcrete(t *testing.T) {
	prototest.Message{}.Test(t, newTestMessageOpen(nil).ProtoReflect().Type())
}

func TestOpenReflection(t *testing.T) {
	prototest.Message{}.Test(t, (*testpb.TestAllTypes)(nil).ProtoReflect().Type())
}

func TestOpenShadow_GetConcrete_SetReflection(t *testing.T) {
	prototest.Message{}.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
		m := &testpb.TestAllTypes{}
		return newTestMessageOpen(m), m
	}).ProtoReflect().Type())
}

func TestOpenShadow_GetReflection_SetConcrete(t *testing.T) {
	prototest.Message{}.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
		m := &testpb.TestAllTypes{}
		return m, newTestMessageOpen(m)
	}).ProtoReflect().Type())
}

func newTestMessageOpen(m *testpb.TestAllTypes) protoreflect.ProtoMessage {
	return &testProtoMessage{
		m:  m,
		md: m.ProtoReflect().Descriptor(),
		new: func() protoreflect.Message {
			return newTestMessageOpen(&testpb.TestAllTypes{}).ProtoReflect()
		},
		has: func(num protoreflect.FieldNumber) bool {
			switch num {
			case fieldSingularInt32:
				return m.GetSingularInt32() != 0
			case fieldSingularInt64:
				return m.GetSingularInt64() != 0
			case fieldSingularUint32:
				return m.GetSingularUint32() != 0
			case fieldSingularUint64:
				return m.GetSingularUint64() != 0
			case fieldSingularSint32:
				return m.GetSingularSint32() != 0
			case fieldSingularSint64:
				return m.GetSingularSint64() != 0
			case fieldSingularFixed32:
				return m.GetSingularFixed32() != 0
			case fieldSingularFixed64:
				return m.GetSingularFixed64() != 0
			case fieldSingularSfixed32:
				return m.GetSingularSfixed32() != 0
			case fieldSingularSfixed64:
				return m.GetSingularSfixed64() != 0
			case fieldSingularFloat:
				return m.GetSingularFloat() != 0 || math.Signbit(float64(m.GetSingularFloat()))
			case fieldSingularDouble:
				return m.GetSingularDouble() != 0 || math.Signbit(m.GetSingularDouble())
			case fieldSingularBool:
				return m.GetSingularBool() != false
			case fieldSingularString:
				return m.GetSingularString() != ""
			case fieldSingularBytes:
				return m.SingularBytes != nil
			case fieldSingularNestedEnum:
				return m.GetSingularNestedEnum() != testpb.TestAllTypes_FOO
			case fieldSingularForeignEnum:
				return m.GetSingularForeignEnum() != testpb.ForeignEnum_FOREIGN_ZERO
			case fieldSingularImportEnum:
				return m.GetSingularImportEnum() != testpb.ImportEnum_IMPORT_ZERO

			case fieldOptionalInt32:
				return m.OptionalInt32 != nil
			case fieldOptionalInt64:
				return m.OptionalInt64 != nil
			case fieldOptionalUint32:
				return m.OptionalUint32 != nil
			case fieldOptionalUint64:
				return m.OptionalUint64 != nil
			case fieldOptionalSint32:
				return m.OptionalSint32 != nil
			case fieldOptionalSint64:
				return m.OptionalSint64 != nil
			case fieldOptionalFixed32:
				return m.OptionalFixed32 != nil
			case fieldOptionalFixed64:
				return m.OptionalFixed64 != nil
			case fieldOptionalSfixed32:
				return m.OptionalSfixed32 != nil
			case fieldOptionalSfixed64:
				return m.OptionalSfixed64 != nil
			case fieldOptionalFloat:
				return m.OptionalFloat != nil
			case fieldOptionalDouble:
				return m.OptionalDouble != nil
			case fieldOptionalBool:
				return m.OptionalBool != nil
			case fieldOptionalString:
				return m.OptionalString != nil
			case fieldOptionalBytes:
				return m.OptionalBytes != nil
			case fieldOptionalGroup:
				return m.Optionalgroup != nil
			case fieldNotGroupLikeDelimited:
				return m.NotGroupLikeDelimited != nil
			case fieldOptionalNestedMessage:
				return m.OptionalNestedMessage != nil
			case fieldOptionalForeignMessage:
				return m.OptionalForeignMessage != nil
			case fieldOptionalImportMessage:
				return m.OptionalImportMessage != nil
			case fieldOptionalNestedEnum:
				return m.OptionalNestedEnum != nil
			case fieldOptionalForeignEnum:
				return m.OptionalForeignEnum != nil
			case fieldOptionalImportEnum:
				return m.OptionalImportEnum != nil
			case fieldOptionalLazyNestedMessage:
				return m.OptionalLazyNestedMessage != nil

			case fieldRepeatedInt32:
				return len(m.GetRepeatedInt32()) > 0
			case fieldRepeatedInt64:
				return len(m.GetRepeatedInt64()) > 0
			case fieldRepeatedUint32:
				return len(m.GetRepeatedUint32()) > 0
			case fieldRepeatedUint64:
				return len(m.GetRepeatedUint64()) > 0
			case fieldRepeatedSint32:
				return len(m.GetRepeatedSint32()) > 0
			case fieldRepeatedSint64:
				return len(m.GetRepeatedSint64()) > 0
			case fieldRepeatedFixed32:
				return len(m.GetRepeatedFixed32()) > 0
			case fieldRepeatedFixed64:
				return len(m.GetRepeatedFixed64()) > 0
			case fieldRepeatedSfixed32:
				return len(m.GetRepeatedSfixed32()) > 0
			case fieldRepeatedSfixed64:
				return len(m.GetRepeatedSfixed64()) > 0
			case fieldRepeatedFloat:
				return len(m.GetRepeatedFloat()) > 0
			case fieldRepeatedDouble:
				return len(m.GetRepeatedDouble()) > 0
			case fieldRepeatedBool:
				return len(m.GetRepeatedBool()) > 0
			case fieldRepeatedString:
				return len(m.GetRepeatedString()) > 0
			case fieldRepeatedBytes:
				return len(m.GetRepeatedBytes()) > 0
			case fieldRepeatedGroup:
				return len(m.GetRepeatedgroup()) > 0
			case fieldRepeatedNestedMessage:
				return len(m.GetRepeatedNestedMessage()) > 0
			case fieldRepeatedForeignMessage:
				return len(m.GetRepeatedForeignMessage()) > 0
			case fieldRepeatedImportMessage:
				return len(m.GetRepeatedImportmessage()) > 0
			case fieldRepeatedNestedEnum:
				return len(m.GetRepeatedNestedEnum()) > 0
			case fieldRepeatedForeignEnum:
				return len(m.GetRepeatedForeignEnum()) > 0
			case fieldRepeatedImportEnum:
				return len(m.GetRepeatedImportenum()) > 0

			case fieldMapInt32Int32:
				return len(m.GetMapInt32Int32()) > 0
			case fieldMapInt64Int64:
				return len(m.GetMapInt64Int64()) > 0
			case fieldMapUint32Uint32:
				return len(m.GetMapUint32Uint32()) > 0
			case fieldMapUint64Uint64:
				return len(m.GetMapUint64Uint64()) > 0
			case fieldMapSint32Sint32:
				return len(m.GetMapSint32Sint32()) > 0
			case fieldMapSint64Sint64:
				return len(m.GetMapSint64Sint64()) > 0
			case fieldMapFixed32Fixed32:
				return len(m.GetMapFixed32Fixed32()) > 0
			case fieldMapFixed64Fixed64:
				return len(m.GetMapFixed64Fixed64()) > 0
			case fieldMapSfixed32Sfixed32:
				return len(m.GetMapSfixed32Sfixed32()) > 0
			case fieldMapSfixed64Sfixed64:
				return len(m.GetMapSfixed64Sfixed64()) > 0
			case fieldMapInt32Float:
				return len(m.GetMapInt32Float()) > 0
			case fieldMapInt32Double:
				return len(m.GetMapInt32Double()) > 0
			case fieldMapBoolBool:
				return len(m.GetMapBoolBool()) > 0
			case fieldMapStringString:
				return len(m.GetMapStringString()) > 0
			case fieldMapStringBytes:
				return len(m.GetMapStringBytes()) > 0
			case fieldMapStringNestedMessage:
				return len(m.GetMapStringNestedMessage()) > 0
			case fieldMapStringNestedEnum:
				return len(m.GetMapStringNestedEnum()) > 0

			case fieldDefaultInt32:
				return m.DefaultInt32 != nil
			case fieldDefaultInt64:
				return m.DefaultInt64 != nil
			case fieldDefaultUint32:
				return m.DefaultUint32 != nil
			case fieldDefaultUint64:
				return m.DefaultUint64 != nil
			case fieldDefaultSint32:
				return m.DefaultSint32 != nil
			case fieldDefaultSint64:
				return m.DefaultSint64 != nil
			case fieldDefaultFixed32:
				return m.DefaultFixed32 != nil
			case fieldDefaultFixed64:
				return m.DefaultFixed64 != nil
			case fieldDefaultSfixed32:
				return m.DefaultSfixed32 != nil
			case fieldDefaultSfixed64:
				return m.DefaultSfixed64 != nil
			case fieldDefaultFloat:
				return m.DefaultFloat != nil
			case fieldDefaultDouble:
				return m.DefaultDouble != nil
			case fieldDefaultBool:
				return m.DefaultBool != nil
			case fieldDefaultString:
				return m.DefaultString != nil
			case fieldDefaultBytes:
				return m.DefaultBytes != nil
			case fieldDefaultNestedEnum:
				return m.DefaultNestedEnum != nil
			case fieldDefaultForeignEnum:
				return m.DefaultForeignEnum != nil

			case fieldOneofUint32:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofUint32)
				return ok
			case fieldOneofNestedMessage:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofNestedMessage)
				return ok
			case fieldOneofString:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofString)
				return ok
			case fieldOneofBytes:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofBytes)
				return ok
			case fieldOneofBool:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofBool)
				return ok
			case fieldOneofUint64:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofUint64)
				return ok
			case fieldOneofFloat:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofFloat)
				return ok
			case fieldOneofDouble:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofDouble)
				return ok
			case fieldOneofEnum:
				_, ok := m.OneofField.(*testpb.TestAllTypes_OneofEnum)
				return ok
			case fieldOneofGroup:
				_, ok := m.OneofField.(*testpb.TestAllTypes_Oneofgroup)
				return ok
			case fieldOneofOptionalUint32:
				_, ok := m.OneofOptional.(*testpb.TestAllTypes_OneofOptionalUint32)
				return ok

			default:
				panic(fmt.Sprintf("has: unknown field %d", num))
			}
		},
		get: func(num protoreflect.FieldNumber) any {
			switch num {
			case fieldSingularInt32:
				return m.GetSingularInt32()
			case fieldSingularInt64:
				return m.GetSingularInt64()
			case fieldSingularUint32:
				return m.GetSingularUint32()
			case fieldSingularUint64:
				return m.GetSingularUint64()
			case fieldSingularSint32:
				return m.GetSingularSint32()
			case fieldSingularSint64:
				return m.GetSingularSint64()
			case fieldSingularFixed32:
				return m.GetSingularFixed32()
			case fieldSingularFixed64:
				return m.GetSingularFixed64()
			case fieldSingularSfixed32:
				return m.GetSingularSfixed32()
			case fieldSingularSfixed64:
				return m.GetSingularSfixed64()
			case fieldSingularFloat:
				return m.GetSingularFloat()
			case fieldSingularDouble:
				return m.GetSingularDouble()
			case fieldSingularBool:
				return m.GetSingularBool()
			case fieldSingularString:
				return m.GetSingularString()
			case fieldSingularBytes:
				return m.GetSingularBytes()
			case fieldSingularNestedEnum:
				return m.GetSingularNestedEnum()
			case fieldSingularForeignEnum:
				return m.GetSingularForeignEnum()
			case fieldSingularImportEnum:
				return m.GetSingularImportEnum()

			case fieldOptionalInt32:
				return m.GetOptionalInt32()
			case fieldOptionalInt64:
				return m.GetOptionalInt64()
			case fieldOptionalUint32:
				return m.GetOptionalUint32()
			case fieldOptionalUint64:
				return m.GetOptionalUint64()
			case fieldOptionalSint32:
				return m.GetOptionalSint32()
			case fieldOptionalSint64:
				return m.GetOptionalSint64()
			case fieldOptionalFixed32:
				return m.GetOptionalFixed32()
			case fieldOptionalFixed64:
				return m.GetOptionalFixed64()
			case fieldOptionalSfixed32:
				return m.GetOptionalSfixed32()
			case fieldOptionalSfixed64:
				return m.GetOptionalSfixed64()
			case fieldOptionalFloat:
				return m.GetOptionalFloat()
			case fieldOptionalDouble:
				return m.GetOptionalDouble()
			case fieldOptionalBool:
				return m.GetOptionalBool()
			case fieldOptionalString:
				return m.GetOptionalString()
			case fieldOptionalBytes:
				return m.GetOptionalBytes()
			case fieldOptionalGroup:
				return m.GetOptionalgroup()
			case fieldNotGroupLikeDelimited:
				return m.GetNotGroupLikeDelimited()
			case fieldOptionalNestedMessage:
				return m.GetOptionalNestedMessage()
			case fieldOptionalForeignMessage:
				return m.GetOptionalForeignMessage()
			case fieldOptionalImportMessage:
				return m.GetOptionalImportMessage()
			case fieldOptionalNestedEnum:
				return m.GetOptionalNestedEnum()
			case fieldOptionalForeignEnum:
				return m.GetOptionalForeignEnum()
			case fieldOptionalImportEnum:
				return m.GetOptionalImportEnum()
			case fieldOptionalLazyNestedMessage:
				return m.GetOptionalLazyNestedMessage()

			case fieldRepeatedInt32:
				return m.GetRepeatedInt32()
			case fieldRepeatedInt64:
				return m.GetRepeatedInt64()
			case fieldRepeatedUint32:
				return m.GetRepeatedUint32()
			case fieldRepeatedUint64:
				return m.GetRepeatedUint64()
			case fieldRepeatedSint32:
				return m.GetRepeatedSint32()
			case fieldRepeatedSint64:
				return m.GetRepeatedSint64()
			case fieldRepeatedFixed32:
				return m.GetRepeatedFixed32()
			case fieldRepeatedFixed64:
				return m.GetRepeatedFixed64()
			case fieldRepeatedSfixed32:
				return m.GetRepeatedSfixed32()
			case fieldRepeatedSfixed64:
				return m.GetRepeatedSfixed64()
			case fieldRepeatedFloat:
				return m.GetRepeatedFloat()
			case fieldRepeatedDouble:
				return m.GetRepeatedDouble()
			case fieldRepeatedBool:
				return m.GetRepeatedBool()
			case fieldRepeatedString:
				return m.GetRepeatedString()
			case fieldRepeatedBytes:
				return m.GetRepeatedBytes()
			case fieldRepeatedGroup:
				return m.GetRepeatedgroup()
			case fieldRepeatedNestedMessage:
				return m.GetRepeatedNestedMessage()
			case fieldRepeatedForeignMessage:
				return m.GetRepeatedForeignMessage()
			case fieldRepeatedImportMessage:
				return m.GetRepeatedImportmessage()
			case fieldRepeatedNestedEnum:
				return m.GetRepeatedNestedEnum()
			case fieldRepeatedForeignEnum:
				return m.GetRepeatedForeignEnum()
			case fieldRepeatedImportEnum:
				return m.GetRepeatedImportenum()

			case fieldMapInt32Int32:
				return m.GetMapInt32Int32()
			case fieldMapInt64Int64:
				return m.GetMapInt64Int64()
			case fieldMapUint32Uint32:
				return m.GetMapUint32Uint32()
			case fieldMapUint64Uint64:
				return m.GetMapUint64Uint64()
			case fieldMapSint32Sint32:
				return m.GetMapSint32Sint32()
			case fieldMapSint64Sint64:
				return m.GetMapSint64Sint64()
			case fieldMapFixed32Fixed32:
				return m.GetMapFixed32Fixed32()
			case fieldMapFixed64Fixed64:
				return m.GetMapFixed64Fixed64()
			case fieldMapSfixed32Sfixed32:
				return m.GetMapSfixed32Sfixed32()
			case fieldMapSfixed64Sfixed64:
				return m.GetMapSfixed64Sfixed64()
			case fieldMapInt32Float:
				return m.GetMapInt32Float()
			case fieldMapInt32Double:
				return m.GetMapInt32Double()
			case fieldMapBoolBool:
				return m.GetMapBoolBool()
			case fieldMapStringString:
				return m.GetMapStringString()
			case fieldMapStringBytes:
				return m.GetMapStringBytes()
			case fieldMapStringNestedMessage:
				return m.GetMapStringNestedMessage()
			case fieldMapStringNestedEnum:
				return m.GetMapStringNestedEnum()

			case fieldDefaultInt32:
				return m.GetDefaultInt32()
			case fieldDefaultInt64:
				return m.GetDefaultInt64()
			case fieldDefaultUint32:
				return m.GetDefaultUint32()
			case fieldDefaultUint64:
				return m.GetDefaultUint64()
			case fieldDefaultSint32:
				return m.GetDefaultSint32()
			case fieldDefaultSint64:
				return m.GetDefaultSint64()
			case fieldDefaultFixed32:
				return m.GetDefaultFixed32()
			case fieldDefaultFixed64:
				return m.GetDefaultFixed64()
			case fieldDefaultSfixed32:
				return m.GetDefaultSfixed32()
			case fieldDefaultSfixed64:
				return m.GetDefaultSfixed64()
			case fieldDefaultFloat:
				return m.GetDefaultFloat()
			case fieldDefaultDouble:
				return m.GetDefaultDouble()
			case fieldDefaultBool:
				return m.GetDefaultBool()
			case fieldDefaultString:
				return m.GetDefaultString()
			case fieldDefaultBytes:
				return m.GetDefaultBytes()
			case fieldDefaultNestedEnum:
				return m.GetDefaultNestedEnum()
			case fieldDefaultForeignEnum:
				return m.GetDefaultForeignEnum()

			case fieldOneofUint32:
				return m.GetOneofUint32()
			case fieldOneofNestedMessage:
				return m.GetOneofNestedMessage()
			case fieldOneofString:
				return m.GetOneofString()
			case fieldOneofBytes:
				return m.GetOneofBytes()
			case fieldOneofBool:
				return m.GetOneofBool()
			case fieldOneofUint64:
				return m.GetOneofUint64()
			case fieldOneofFloat:
				return m.GetOneofFloat()
			case fieldOneofDouble:
				return m.GetOneofDouble()
			case fieldOneofEnum:
				return protoreflect.EnumNumber(m.GetOneofEnum())
			case fieldOneofGroup:
				return m.GetOneofgroup()
			case fieldOneofOptionalUint32:
				return m.GetOneofOptionalUint32()

			default:
				panic(fmt.Sprintf("get: unknown field %d", num))
			}
		},
		set: func(num protoreflect.FieldNumber, v any) {
			switch num {
			case fieldSingularInt32:
				m.SingularInt32 = v.(int32)
			case fieldSingularInt64:
				m.SingularInt64 = v.(int64)
			case fieldSingularUint32:
				m.SingularUint32 = v.(uint32)
			case fieldSingularUint64:
				m.SingularUint64 = v.(uint64)
			case fieldSingularSint32:
				m.SingularSint32 = v.(int32)
			case fieldSingularSint64:
				m.SingularSint64 = v.(int64)
			case fieldSingularFixed32:
				m.SingularFixed32 = v.(uint32)
			case fieldSingularFixed64:
				m.SingularFixed64 = v.(uint64)
			case fieldSingularSfixed32:
				m.SingularSfixed32 = v.(int32)
			case fieldSingularSfixed64:
				m.SingularSfixed64 = v.(int64)
			case fieldSingularFloat:
				m.SingularFloat = v.(float32)
			case fieldSingularDouble:
				m.SingularDouble = v.(float64)
			case fieldSingularBool:
				m.SingularBool = v.(bool)
			case fieldSingularString:
				m.SingularString = v.(string)
			case fieldSingularBytes:
				m.SingularBytes = v.([]byte)
			case fieldSingularNestedEnum:
				m.SingularNestedEnum = testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber))
			case fieldSingularForeignEnum:
				m.SingularForeignEnum = testpb.ForeignEnum(v.(protoreflect.EnumNumber))
			case fieldSingularImportEnum:
				m.SingularImportEnum = testpb.ImportEnum(v.(protoreflect.EnumNumber))

			case fieldOptionalInt32:
				m.OptionalInt32 = proto.Int32(v.(int32))
			case fieldOptionalInt64:
				m.OptionalInt64 = proto.Int64(v.(int64))
			case fieldOptionalUint32:
				m.OptionalUint32 = proto.Uint32(v.(uint32))
			case fieldOptionalUint64:
				m.OptionalUint64 = proto.Uint64(v.(uint64))
			case fieldOptionalSint32:
				m.OptionalSint32 = proto.Int32(v.(int32))
			case fieldOptionalSint64:
				m.OptionalSint64 = proto.Int64(v.(int64))
			case fieldOptionalFixed32:
				m.OptionalFixed32 = proto.Uint32(v.(uint32))
			case fieldOptionalFixed64:
				m.OptionalFixed64 = proto.Uint64(v.(uint64))
			case fieldOptionalSfixed32:
				m.OptionalSfixed32 = proto.Int32(v.(int32))
			case fieldOptionalSfixed64:
				m.OptionalSfixed64 = proto.Int64(v.(int64))
			case fieldOptionalFloat:
				m.OptionalFloat = proto.Float32(v.(float32))
			case fieldOptionalDouble:
				m.OptionalDouble = proto.Float64(v.(float64))
			case fieldOptionalBool:
				m.OptionalBool = proto.Bool(v.(bool))
			case fieldOptionalString:
				m.OptionalString = proto.String(v.(string))
			case fieldOptionalBytes:
				if v.([]byte) == nil {
					v = []byte{}
				}
				m.OptionalBytes = v.([]byte)
			case fieldNotGroupLikeDelimited:
				m.NotGroupLikeDelimited = v.(*testpb.TestAllTypes_OptionalGroup)
			case fieldOptionalGroup:
				m.Optionalgroup = v.(*testpb.TestAllTypes_OptionalGroup)
			case fieldOptionalNestedMessage:
				m.OptionalNestedMessage = v.(*testpb.TestAllTypes_NestedMessage)
			case fieldOptionalForeignMessage:
				m.OptionalForeignMessage = v.(*testpb.ForeignMessage)
			case fieldOptionalImportMessage:
				m.OptionalImportMessage = v.(*testpb.ImportMessage)
			case fieldOptionalNestedEnum:
				m.OptionalNestedEnum = testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)).Enum()
			case fieldOptionalForeignEnum:
				m.OptionalForeignEnum = testpb.ForeignEnum(v.(protoreflect.EnumNumber)).Enum()
			case fieldOptionalImportEnum:
				m.OptionalImportEnum = testpb.ImportEnum(v.(protoreflect.EnumNumber)).Enum()
			case fieldOptionalLazyNestedMessage:
				m.OptionalLazyNestedMessage = v.(*testpb.TestAllTypes_NestedMessage)

			case fieldRepeatedInt32:
				m.RepeatedInt32 = v.([]int32)
			case fieldRepeatedInt64:
				m.RepeatedInt64 = v.([]int64)
			case fieldRepeatedUint32:
				m.RepeatedUint32 = v.([]uint32)
			case fieldRepeatedUint64:
				m.RepeatedUint64 = v.([]uint64)
			case fieldRepeatedSint32:
				m.RepeatedSint32 = v.([]int32)
			case fieldRepeatedSint64:
				m.RepeatedSint64 = v.([]int64)
			case fieldRepeatedFixed32:
				m.RepeatedFixed32 = v.([]uint32)
			case fieldRepeatedFixed64:
				m.RepeatedFixed64 = v.([]uint64)
			case fieldRepeatedSfixed32:
				m.RepeatedSfixed32 = v.([]int32)
			case fieldRepeatedSfixed64:
				m.RepeatedSfixed64 = v.([]int64)
			case fieldRepeatedFloat:
				m.RepeatedFloat = v.([]float32)
			case fieldRepeatedDouble:
				m.RepeatedDouble = v.([]float64)
			case fieldRepeatedBool:
				m.RepeatedBool = v.([]bool)
			case fieldRepeatedString:
				m.RepeatedString = v.([]string)
			case fieldRepeatedBytes:
				m.RepeatedBytes = v.([][]byte)
			case fieldRepeatedGroup:
				m.Repeatedgroup = v.([]*testpb.TestAllTypes_RepeatedGroup)
			case fieldRepeatedNestedMessage:
				m.RepeatedNestedMessage = v.([]*testpb.TestAllTypes_NestedMessage)
			case fieldRepeatedForeignMessage:
				m.RepeatedForeignMessage = v.([]*testpb.ForeignMessage)
			case fieldRepeatedImportMessage:
				m.RepeatedImportmessage = v.([]*testpb.ImportMessage)
			case fieldRepeatedNestedEnum:
				m.RepeatedNestedEnum = v.([]testpb.TestAllTypes_NestedEnum)
			case fieldRepeatedForeignEnum:
				m.RepeatedForeignEnum = v.([]testpb.ForeignEnum)
			case fieldRepeatedImportEnum:
				m.RepeatedImportenum = v.([]testpb.ImportEnum)

			case fieldMapInt32Int32:
				m.MapInt32Int32 = v.(map[int32]int32)
			case fieldMapInt64Int64:
				m.MapInt64Int64 = v.(map[int64]int64)
			case fieldMapUint32Uint32:
				m.MapUint32Uint32 = v.(map[uint32]uint32)
			case fieldMapUint64Uint64:
				m.MapUint64Uint64 = v.(map[uint64]uint64)
			case fieldMapSint32Sint32:
				m.MapSint32Sint32 = v.(map[int32]int32)
			case fieldMapSint64Sint64:
				m.MapSint64Sint64 = v.(map[int64]int64)
			case fieldMapFixed32Fixed32:
				m.MapFixed32Fixed32 = v.(map[uint32]uint32)
			case fieldMapFixed64Fixed64:
				m.MapFixed64Fixed64 = v.(map[uint64]uint64)
			case fieldMapSfixed32Sfixed32:
				m.MapSfixed32Sfixed32 = v.(map[int32]int32)
			case fieldMapSfixed64Sfixed64:
				m.MapSfixed64Sfixed64 = v.(map[int64]int64)
			case fieldMapInt32Float:
				m.MapInt32Float = v.(map[int32]float32)
			case fieldMapInt32Double:
				m.MapInt32Double = v.(map[int32]float64)
			case fieldMapBoolBool:
				m.MapBoolBool = v.(map[bool]bool)
			case fieldMapStringString:
				m.MapStringString = v.(map[string]string)
			case fieldMapStringBytes:
				m.MapStringBytes = v.(map[string][]byte)
			case fieldMapStringNestedMessage:
				m.MapStringNestedMessage = v.(map[string]*testpb.TestAllTypes_NestedMessage)
			case fieldMapStringNestedEnum:
				m.MapStringNestedEnum = v.(map[string]testpb.TestAllTypes_NestedEnum)

			case fieldDefaultInt32:
				m.DefaultInt32 = proto.Int32(v.(int32))
			case fieldDefaultInt64:
				m.DefaultInt64 = proto.Int64(v.(int64))
			case fieldDefaultUint32:
				m.DefaultUint32 = proto.Uint32(v.(uint32))
			case fieldDefaultUint64:
				m.DefaultUint64 = proto.Uint64(v.(uint64))
			case fieldDefaultSint32:
				m.DefaultSint32 = proto.Int32(v.(int32))
			case fieldDefaultSint64:
				m.DefaultSint64 = proto.Int64(v.(int64))
			case fieldDefaultFixed32:
				m.DefaultFixed32 = proto.Uint32(v.(uint32))
			case fieldDefaultFixed64:
				m.DefaultFixed64 = proto.Uint64(v.(uint64))
			case fieldDefaultSfixed32:
				m.DefaultSfixed32 = proto.Int32(v.(int32))
			case fieldDefaultSfixed64:
				m.DefaultSfixed64 = proto.Int64(v.(int64))
			case fieldDefaultFloat:
				m.DefaultFloat = proto.Float32(v.(float32))
			case fieldDefaultDouble:
				m.DefaultDouble = proto.Float64(v.(float64))
			case fieldDefaultBool:
				m.DefaultBool = proto.Bool(v.(bool))
			case fieldDefaultString:
				m.DefaultString = proto.String(v.(string))
			case fieldDefaultBytes:
				if v.([]byte) == nil {
					v = []byte{}
				}
				m.DefaultBytes = v.([]byte)
			case fieldDefaultNestedEnum:
				m.DefaultNestedEnum = testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)).Enum()
			case fieldDefaultForeignEnum:
				m.DefaultForeignEnum = testpb.ForeignEnum(v.(protoreflect.EnumNumber)).Enum()

			case fieldOneofUint32:
				m.OneofField = &testpb.TestAllTypes_OneofUint32{v.(uint32)}
			case fieldOneofNestedMessage:
				m.OneofField = &testpb.TestAllTypes_OneofNestedMessage{v.(*testpb.TestAllTypes_NestedMessage)}
			case fieldOneofString:
				m.OneofField = &testpb.TestAllTypes_OneofString{v.(string)}
			case fieldOneofBytes:
				m.OneofField = &testpb.TestAllTypes_OneofBytes{v.([]byte)}
			case fieldOneofBool:
				m.OneofField = &testpb.TestAllTypes_OneofBool{v.(bool)}
			case fieldOneofUint64:
				m.OneofField = &testpb.TestAllTypes_OneofUint64{v.(uint64)}
			case fieldOneofFloat:
				m.OneofField = &testpb.TestAllTypes_OneofFloat{v.(float32)}
			case fieldOneofDouble:
				m.OneofField = &testpb.TestAllTypes_OneofDouble{v.(float64)}
			case fieldOneofEnum:
				m.OneofField = &testpb.TestAllTypes_OneofEnum{testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber))}
			case fieldOneofGroup:
				m.OneofField = &testpb.TestAllTypes_Oneofgroup{v.(*testpb.TestAllTypes_OneofGroup)}
			case fieldOneofOptionalUint32:
				m.OneofOptional = &testpb.TestAllTypes_OneofOptionalUint32{v.(uint32)}

			default:
				panic(fmt.Sprintf("set: unknown field %d", num))
			}
		},
		clear: func(num protoreflect.FieldNumber) {
			switch num {
			case fieldSingularInt32:
				m.SingularInt32 = 0
			case fieldSingularInt64:
				m.SingularInt64 = 0
			case fieldSingularUint32:
				m.SingularUint32 = 0
			case fieldSingularUint64:
				m.SingularUint64 = 0
			case fieldSingularSint32:
				m.SingularSint32 = 0
			case fieldSingularSint64:
				m.SingularSint64 = 0
			case fieldSingularFixed32:
				m.SingularFixed32 = 0
			case fieldSingularFixed64:
				m.SingularFixed64 = 0
			case fieldSingularSfixed32:
				m.SingularSfixed32 = 0
			case fieldSingularSfixed64:
				m.SingularSfixed64 = 0
			case fieldSingularFloat:
				m.SingularFloat = 0
			case fieldSingularDouble:
				m.SingularDouble = 0
			case fieldSingularBool:
				m.SingularBool = false
			case fieldSingularString:
				m.SingularString = ""
			case fieldSingularBytes:
				m.SingularBytes = nil
			case fieldSingularNestedEnum:
				m.SingularNestedEnum = testpb.TestAllTypes_FOO
			case fieldSingularForeignEnum:
				m.SingularForeignEnum = testpb.ForeignEnum_FOREIGN_ZERO
			case fieldSingularImportEnum:
				m.SingularImportEnum = testpb.ImportEnum_IMPORT_ZERO

			case fieldOptionalInt32:
				m.OptionalInt32 = nil
			case fieldOptionalInt64:
				m.OptionalInt64 = nil
			case fieldOptionalUint32:
				m.OptionalUint32 = nil
			case fieldOptionalUint64:
				m.OptionalUint64 = nil
			case fieldOptionalSint32:
				m.OptionalSint32 = nil
			case fieldOptionalSint64:
				m.OptionalSint64 = nil
			case fieldOptionalFixed32:
				m.OptionalFixed32 = nil
			case fieldOptionalFixed64:
				m.OptionalFixed64 = nil
			case fieldOptionalSfixed32:
				m.OptionalSfixed32 = nil
			case fieldOptionalSfixed64:
				m.OptionalSfixed64 = nil
			case fieldOptionalFloat:
				m.OptionalFloat = nil
			case fieldOptionalDouble:
				m.OptionalDouble = nil
			case fieldOptionalBool:
				m.OptionalBool = nil
			case fieldOptionalString:
				m.OptionalString = nil
			case fieldOptionalBytes:
				m.OptionalBytes = nil
			case fieldOptionalGroup:
				m.Optionalgroup = nil
			case fieldNotGroupLikeDelimited:
				m.NotGroupLikeDelimited = nil
			case fieldOptionalNestedMessage:
				m.OptionalNestedMessage = nil
			case fieldOptionalForeignMessage:
				m.OptionalForeignMessage = nil
			case fieldOptionalImportMessage:
				m.OptionalImportMessage = nil
			case fieldOptionalNestedEnum:
				m.OptionalNestedEnum = nil
			case fieldOptionalForeignEnum:
				m.OptionalForeignEnum = nil
			case fieldOptionalImportEnum:
				m.OptionalImportEnum = nil
			case fieldOptionalLazyNestedMessage:
				m.OptionalLazyNestedMessage = nil

			case fieldRepeatedInt32:
				m.RepeatedInt32 = nil
			case fieldRepeatedInt64:
				m.RepeatedInt64 = nil
			case fieldRepeatedUint32:
				m.RepeatedUint32 = nil
			case fieldRepeatedUint64:
				m.RepeatedUint64 = nil
			case fieldRepeatedSint32:
				m.RepeatedSint32 = nil
			case fieldRepeatedSint64:
				m.RepeatedSint64 = nil
			case fieldRepeatedFixed32:
				m.RepeatedFixed32 = nil
			case fieldRepeatedFixed64:
				m.RepeatedFixed64 = nil
			case fieldRepeatedSfixed32:
				m.RepeatedSfixed32 = nil
			case fieldRepeatedSfixed64:
				m.RepeatedSfixed64 = nil
			case fieldRepeatedFloat:
				m.RepeatedFloat = nil
			case fieldRepeatedDouble:
				m.RepeatedDouble = nil
			case fieldRepeatedBool:
				m.RepeatedBool = nil
			case fieldRepeatedString:
				m.RepeatedString = nil
			case fieldRepeatedBytes:
				m.RepeatedBytes = nil
			case fieldRepeatedGroup:
				m.Repeatedgroup = nil
			case fieldRepeatedNestedMessage:
				m.RepeatedNestedMessage = nil
			case fieldRepeatedForeignMessage:
				m.RepeatedForeignMessage = nil
			case fieldRepeatedImportMessage:
				m.RepeatedImportmessage = nil
			case fieldRepeatedNestedEnum:
				m.RepeatedNestedEnum = nil
			case fieldRepeatedForeignEnum:
				m.RepeatedForeignEnum = nil
			case fieldRepeatedImportEnum:
				m.RepeatedImportenum = nil

			case fieldMapInt32Int32:
				m.MapInt32Int32 = nil
			case fieldMapInt64Int64:
				m.MapInt64Int64 = nil
			case fieldMapUint32Uint32:
				m.MapUint32Uint32 = nil
			case fieldMapUint64Uint64:
				m.MapUint64Uint64 = nil
			case fieldMapSint32Sint32:
				m.MapSint32Sint32 = nil
			case fieldMapSint64Sint64:
				m.MapSint64Sint64 = nil
			case fieldMapFixed32Fixed32:
				m.MapFixed32Fixed32 = nil
			case fieldMapFixed64Fixed64:
				m.MapFixed64Fixed64 = nil
			case fieldMapSfixed32Sfixed32:
				m.MapSfixed32Sfixed32 = nil
			case fieldMapSfixed64Sfixed64:
				m.MapSfixed64Sfixed64 = nil
			case fieldMapInt32Float:
				m.MapInt32Float = nil
			case fieldMapInt32Double:
				m.MapInt32Double = nil
			case fieldMapBoolBool:
				m.MapBoolBool = nil
			case fieldMapStringString:
				m.MapStringString = nil
			case fieldMapStringBytes:
				m.MapStringBytes = nil
			case fieldMapStringNestedMessage:
				m.MapStringNestedMessage = nil
			case fieldMapStringNestedEnum:
				m.MapStringNestedEnum = nil

			case fieldDefaultInt32:
				m.DefaultInt32 = nil
			case fieldDefaultInt64:
				m.DefaultInt64 = nil
			case fieldDefaultUint32:
				m.DefaultUint32 = nil
			case fieldDefaultUint64:
				m.DefaultUint64 = nil
			case fieldDefaultSint32:
				m.DefaultSint32 = nil
			case fieldDefaultSint64:
				m.DefaultSint64 = nil
			case fieldDefaultFixed32:
				m.DefaultFixed32 = nil
			case fieldDefaultFixed64:
				m.DefaultFixed64 = nil
			case fieldDefaultSfixed32:
				m.DefaultSfixed32 = nil
			case fieldDefaultSfixed64:
				m.DefaultSfixed64 = nil
			case fieldDefaultFloat:
				m.DefaultFloat = nil
			case fieldDefaultDouble:
				m.DefaultDouble = nil
			case fieldDefaultBool:
				m.DefaultBool = nil
			case fieldDefaultString:
				m.DefaultString = nil
			case fieldDefaultBytes:
				m.DefaultBytes = nil
			case fieldDefaultNestedEnum:
				m.DefaultNestedEnum = nil
			case fieldDefaultForeignEnum:
				m.DefaultForeignEnum = nil

			case fieldOneofUint32:
				m.OneofField = nil
			case fieldOneofNestedMessage:
				m.OneofField = nil
			case fieldOneofString:
				m.OneofField = nil
			case fieldOneofBytes:
				m.OneofField = nil
			case fieldOneofBool:
				m.OneofField = nil
			case fieldOneofUint64:
				m.OneofField = nil
			case fieldOneofFloat:
				m.OneofField = nil
			case fieldOneofDouble:
				m.OneofField = nil
			case fieldOneofEnum:
				m.OneofField = nil
			case fieldOneofGroup:
				m.OneofField = nil
			case fieldOneofOptionalUint32:
				m.OneofOptional = nil

			default:
				panic(fmt.Sprintf("clear: unknown field %d", num))
			}
		},
	}
}
