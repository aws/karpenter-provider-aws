// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflection_test

import (
	"fmt"
	"math"
	"testing"

	testpb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_opaque"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/prototest"
)

var enableLazy = proto.UnmarshalOptions{}
var disableLazy = proto.UnmarshalOptions{
	NoLazyDecoding: true,
}

var lazyCombinations = []struct {
	desc string
	ptm  prototest.Message
}{
	{
		desc: "lazy decoding",
		ptm: prototest.Message{
			UnmarshalOptions: enableLazy,
		},
	},

	{
		desc: "no lazy decoding",
		ptm: prototest.Message{
			UnmarshalOptions: disableLazy,
		},
	},
}

func TestOpaqueConcrete(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newTestMessageOpaque(nil).ProtoReflect().Type())
		})
	}
}

func TestOpaqueReflection(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, (*testpb.TestAllTypes)(nil).ProtoReflect().Type())
		})
	}
}

func TestOpaqueShadow_GetConcrete_SetReflection(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
				m := &testpb.TestAllTypes{}
				return newTestMessageOpaque(m), m
			}).ProtoReflect().Type())
		})
	}
}

func TestOpaqueShadow_GetReflection_SetConcrete(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
				m := &testpb.TestAllTypes{}
				return m, newTestMessageOpaque(m)
			}).ProtoReflect().Type())
		})
	}
}

func newTestMessageOpaque(m *testpb.TestAllTypes) protoreflect.ProtoMessage {
	return &testProtoMessage{
		m:  m,
		md: m.ProtoReflect().Descriptor(),
		new: func() protoreflect.Message {
			return newTestMessageOpaque(&testpb.TestAllTypes{}).ProtoReflect()
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
				return len(m.GetSingularBytes()) != 0
			case fieldSingularNestedEnum:
				return m.GetSingularNestedEnum() != testpb.TestAllTypes_FOO
			case fieldSingularForeignEnum:
				return m.GetSingularForeignEnum() != testpb.ForeignEnum_FOREIGN_ZERO
			case fieldSingularImportEnum:
				return m.GetSingularImportEnum() != testpb.ImportEnum_IMPORT_ZERO

			case fieldOptionalInt32:
				return m.HasOptionalInt32()
			case fieldOptionalInt64:
				return m.HasOptionalInt64()
			case fieldOptionalUint32:
				return m.HasOptionalUint32()
			case fieldOptionalUint64:
				return m.HasOptionalUint64()
			case fieldOptionalSint32:
				return m.HasOptionalSint32()
			case fieldOptionalSint64:
				return m.HasOptionalSint64()
			case fieldOptionalFixed32:
				return m.HasOptionalFixed32()
			case fieldOptionalFixed64:
				return m.HasOptionalFixed64()
			case fieldOptionalSfixed32:
				return m.HasOptionalSfixed32()
			case fieldOptionalSfixed64:
				return m.HasOptionalSfixed64()
			case fieldOptionalFloat:
				return m.HasOptionalFloat()
			case fieldOptionalDouble:
				return m.HasOptionalDouble()
			case fieldOptionalBool:
				return m.HasOptionalBool()
			case fieldOptionalString:
				return m.HasOptionalString()
			case fieldOptionalBytes:
				return m.HasOptionalBytes()
			case fieldOptionalGroup:
				return m.HasOptionalgroup()
			case fieldNotGroupLikeDelimited:
				return m.HasNotGroupLikeDelimited()
			case fieldOptionalGroup:
				return m.HasOptionalgroup()
			case fieldOptionalNestedMessage:
				return m.HasOptionalNestedMessage()
			case fieldOptionalForeignMessage:
				return m.HasOptionalForeignMessage()
			case fieldOptionalImportMessage:
				return m.HasOptionalImportMessage()
			case fieldOptionalNestedEnum:
				return m.HasOptionalNestedEnum()
			case fieldOptionalForeignEnum:
				return m.HasOptionalForeignEnum()
			case fieldOptionalImportEnum:
				return m.HasOptionalImportEnum()
			case fieldOptionalLazyNestedMessage:
				return m.HasOptionalLazyNestedMessage()

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
				return m.HasDefaultInt32()
			case fieldDefaultInt64:
				return m.HasDefaultInt64()
			case fieldDefaultUint32:
				return m.HasDefaultUint32()
			case fieldDefaultUint64:
				return m.HasDefaultUint64()
			case fieldDefaultSint32:
				return m.HasDefaultSint32()
			case fieldDefaultSint64:
				return m.HasDefaultSint64()
			case fieldDefaultFixed32:
				return m.HasDefaultFixed32()
			case fieldDefaultFixed64:
				return m.HasDefaultFixed64()
			case fieldDefaultSfixed32:
				return m.HasDefaultSfixed32()
			case fieldDefaultSfixed64:
				return m.HasDefaultSfixed64()
			case fieldDefaultFloat:
				return m.HasDefaultFloat()
			case fieldDefaultDouble:
				return m.HasDefaultDouble()
			case fieldDefaultBool:
				return m.HasDefaultBool()
			case fieldDefaultString:
				return m.HasDefaultString()
			case fieldDefaultBytes:
				return m.HasDefaultBytes()
			case fieldDefaultNestedEnum:
				return m.HasDefaultNestedEnum()
			case fieldDefaultForeignEnum:
				return m.HasDefaultForeignEnum()

			case fieldDefaultInt32:
				return m.HasDefaultInt32()
			case fieldDefaultInt64:
				return m.HasDefaultInt64()
			case fieldDefaultUint32:
				return m.HasDefaultUint32()
			case fieldDefaultUint64:
				return m.HasDefaultUint64()
			case fieldDefaultSint32:
				return m.HasDefaultSint32()
			case fieldDefaultSint64:
				return m.HasDefaultSint64()
			case fieldDefaultFixed32:
				return m.HasDefaultFixed32()
			case fieldDefaultFixed64:
				return m.HasDefaultFixed64()
			case fieldDefaultSfixed32:
				return m.HasDefaultSfixed32()
			case fieldDefaultSfixed64:
				return m.HasDefaultSfixed64()
			case fieldDefaultFloat:
				return m.HasDefaultFloat()
			case fieldDefaultDouble:
				return m.HasDefaultDouble()
			case fieldDefaultBool:
				return m.HasDefaultBool()
			case fieldDefaultString:
				return m.HasDefaultString()
			case fieldDefaultBytes:
				return m.HasDefaultBytes()
			case fieldDefaultNestedEnum:
				return m.HasDefaultNestedEnum()
			case fieldDefaultForeignEnum:
				return m.HasDefaultForeignEnum()

			case fieldOneofUint32:
				return m.HasOneofUint32()
			case fieldOneofNestedMessage:
				return m.HasOneofNestedMessage()
			case fieldOneofString:
				return m.HasOneofString()
			case fieldOneofBytes:
				return m.HasOneofBytes()
			case fieldOneofBool:
				return m.HasOneofBool()
			case fieldOneofUint64:
				return m.HasOneofUint64()
			case fieldOneofFloat:
				return m.HasOneofFloat()
			case fieldOneofDouble:
				return m.HasOneofDouble()
			case fieldOneofEnum:
				return m.HasOneofEnum()
			case fieldOneofGroup:
				return m.HasOneofgroup()
			case fieldOneofOptionalUint32:
				return m.HasOneofOptionalUint32()

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
				return m.GetOneofEnum()
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
				m.SetSingularInt32(v.(int32))
			case fieldSingularInt64:
				m.SetSingularInt64(v.(int64))
			case fieldSingularUint32:
				m.SetSingularUint32(v.(uint32))
			case fieldSingularUint64:
				m.SetSingularUint64(v.(uint64))
			case fieldSingularSint32:
				m.SetSingularSint32(v.(int32))
			case fieldSingularSint64:
				m.SetSingularSint64(v.(int64))
			case fieldSingularFixed32:
				m.SetSingularFixed32(v.(uint32))
			case fieldSingularFixed64:
				m.SetSingularFixed64(v.(uint64))
			case fieldSingularSfixed32:
				m.SetSingularSfixed32(v.(int32))
			case fieldSingularSfixed64:
				m.SetSingularSfixed64(v.(int64))
			case fieldSingularFloat:
				m.SetSingularFloat(v.(float32))
			case fieldSingularDouble:
				m.SetSingularDouble(v.(float64))
			case fieldSingularBool:
				m.SetSingularBool(v.(bool))
			case fieldSingularString:
				m.SetSingularString(v.(string))
			case fieldSingularBytes:
				m.SetSingularBytes(v.([]byte))
			case fieldSingularNestedEnum:
				m.SetSingularNestedEnum(testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)))
			case fieldSingularForeignEnum:
				m.SetSingularForeignEnum(testpb.ForeignEnum(v.(protoreflect.EnumNumber)))
			case fieldSingularImportEnum:
				m.SetSingularImportEnum(testpb.ImportEnum(v.(protoreflect.EnumNumber)))

			case fieldOptionalInt32:
				m.SetOptionalInt32(v.(int32))
			case fieldOptionalInt64:
				m.SetOptionalInt64(v.(int64))
			case fieldOptionalUint32:
				m.SetOptionalUint32(v.(uint32))
			case fieldOptionalUint64:
				m.SetOptionalUint64(v.(uint64))
			case fieldOptionalSint32:
				m.SetOptionalSint32(v.(int32))
			case fieldOptionalSint64:
				m.SetOptionalSint64(v.(int64))
			case fieldOptionalFixed32:
				m.SetOptionalFixed32(v.(uint32))
			case fieldOptionalFixed64:
				m.SetOptionalFixed64(v.(uint64))
			case fieldOptionalSfixed32:
				m.SetOptionalSfixed32(v.(int32))
			case fieldOptionalSfixed64:
				m.SetOptionalSfixed64(v.(int64))
			case fieldOptionalFloat:
				m.SetOptionalFloat(v.(float32))
			case fieldOptionalDouble:
				m.SetOptionalDouble(v.(float64))
			case fieldOptionalBool:
				m.SetOptionalBool(v.(bool))
			case fieldOptionalString:
				m.SetOptionalString(v.(string))
			case fieldOptionalBytes:
				m.SetOptionalBytes(v.([]byte))
			case fieldOptionalGroup:
				m.SetOptionalgroup(v.(*testpb.TestAllTypes_OptionalGroup))
			case fieldNotGroupLikeDelimited:
				m.SetNotGroupLikeDelimited(v.(*testpb.TestAllTypes_OptionalGroup))
			case fieldOptionalNestedMessage:
				m.SetOptionalNestedMessage(v.(*testpb.TestAllTypes_NestedMessage))
			case fieldOptionalForeignMessage:
				m.SetOptionalForeignMessage(v.(*testpb.ForeignMessage))
			case fieldOptionalImportMessage:
				m.SetOptionalImportMessage(v.(*testpb.ImportMessage))
			case fieldOptionalNestedEnum:
				m.SetOptionalNestedEnum(testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)))
			case fieldOptionalForeignEnum:
				m.SetOptionalForeignEnum(testpb.ForeignEnum(v.(protoreflect.EnumNumber)))
			case fieldOptionalImportEnum:
				m.SetOptionalImportEnum(testpb.ImportEnum(v.(protoreflect.EnumNumber)))
			case fieldOptionalLazyNestedMessage:
				m.SetOptionalLazyNestedMessage(v.(*testpb.TestAllTypes_NestedMessage))

			case fieldRepeatedInt32:
				m.SetRepeatedInt32(v.([]int32))
			case fieldRepeatedInt64:
				m.SetRepeatedInt64(v.([]int64))
			case fieldRepeatedUint32:
				m.SetRepeatedUint32(v.([]uint32))
			case fieldRepeatedUint64:
				m.SetRepeatedUint64(v.([]uint64))
			case fieldRepeatedSint32:
				m.SetRepeatedSint32(v.([]int32))
			case fieldRepeatedSint64:
				m.SetRepeatedSint64(v.([]int64))
			case fieldRepeatedFixed32:
				m.SetRepeatedFixed32(v.([]uint32))
			case fieldRepeatedFixed64:
				m.SetRepeatedFixed64(v.([]uint64))
			case fieldRepeatedSfixed32:
				m.SetRepeatedSfixed32(v.([]int32))
			case fieldRepeatedSfixed64:
				m.SetRepeatedSfixed64(v.([]int64))
			case fieldRepeatedFloat:
				m.SetRepeatedFloat(v.([]float32))
			case fieldRepeatedDouble:
				m.SetRepeatedDouble(v.([]float64))
			case fieldRepeatedBool:
				m.SetRepeatedBool(v.([]bool))
			case fieldRepeatedString:
				m.SetRepeatedString(v.([]string))
			case fieldRepeatedBytes:
				m.SetRepeatedBytes(v.([][]byte))
			case fieldRepeatedGroup:
				m.SetRepeatedgroup(v.([]*testpb.TestAllTypes_RepeatedGroup))
			case fieldRepeatedNestedMessage:
				m.SetRepeatedNestedMessage(v.([]*testpb.TestAllTypes_NestedMessage))
			case fieldRepeatedForeignMessage:
				m.SetRepeatedForeignMessage(v.([]*testpb.ForeignMessage))
			case fieldRepeatedImportMessage:
				m.SetRepeatedImportmessage(v.([]*testpb.ImportMessage))
			case fieldRepeatedNestedEnum:
				m.SetRepeatedNestedEnum(v.([]testpb.TestAllTypes_NestedEnum))
			case fieldRepeatedForeignEnum:
				m.SetRepeatedForeignEnum(v.([]testpb.ForeignEnum))
			case fieldRepeatedImportEnum:
				m.SetRepeatedImportenum(v.([]testpb.ImportEnum))

			case fieldMapInt32Int32:
				m.SetMapInt32Int32(v.(map[int32]int32))
			case fieldMapInt64Int64:
				m.SetMapInt64Int64(v.(map[int64]int64))
			case fieldMapUint32Uint32:
				m.SetMapUint32Uint32(v.(map[uint32]uint32))
			case fieldMapUint64Uint64:
				m.SetMapUint64Uint64(v.(map[uint64]uint64))
			case fieldMapSint32Sint32:
				m.SetMapSint32Sint32(v.(map[int32]int32))
			case fieldMapSint64Sint64:
				m.SetMapSint64Sint64(v.(map[int64]int64))
			case fieldMapFixed32Fixed32:
				m.SetMapFixed32Fixed32(v.(map[uint32]uint32))
			case fieldMapFixed64Fixed64:
				m.SetMapFixed64Fixed64(v.(map[uint64]uint64))
			case fieldMapSfixed32Sfixed32:
				m.SetMapSfixed32Sfixed32(v.(map[int32]int32))
			case fieldMapSfixed64Sfixed64:
				m.SetMapSfixed64Sfixed64(v.(map[int64]int64))
			case fieldMapInt32Float:
				m.SetMapInt32Float(v.(map[int32]float32))
			case fieldMapInt32Double:
				m.SetMapInt32Double(v.(map[int32]float64))
			case fieldMapBoolBool:
				m.SetMapBoolBool(v.(map[bool]bool))
			case fieldMapStringString:
				m.SetMapStringString(v.(map[string]string))
			case fieldMapStringBytes:
				m.SetMapStringBytes(v.(map[string][]byte))
			case fieldMapStringNestedMessage:
				m.SetMapStringNestedMessage(v.(map[string]*testpb.TestAllTypes_NestedMessage))
			case fieldMapStringNestedEnum:
				m.SetMapStringNestedEnum(v.(map[string]testpb.TestAllTypes_NestedEnum))

			case fieldDefaultInt32:
				m.SetDefaultInt32(v.(int32))
			case fieldDefaultInt64:
				m.SetDefaultInt64(v.(int64))
			case fieldDefaultUint32:
				m.SetDefaultUint32(v.(uint32))
			case fieldDefaultUint64:
				m.SetDefaultUint64(v.(uint64))
			case fieldDefaultSint32:
				m.SetDefaultSint32(v.(int32))
			case fieldDefaultSint64:
				m.SetDefaultSint64(v.(int64))
			case fieldDefaultFixed32:
				m.SetDefaultFixed32(v.(uint32))
			case fieldDefaultFixed64:
				m.SetDefaultFixed64(v.(uint64))
			case fieldDefaultSfixed32:
				m.SetDefaultSfixed32(v.(int32))
			case fieldDefaultSfixed64:
				m.SetDefaultSfixed64(v.(int64))
			case fieldDefaultFloat:
				m.SetDefaultFloat(v.(float32))
			case fieldDefaultDouble:
				m.SetDefaultDouble(v.(float64))
			case fieldDefaultBool:
				m.SetDefaultBool(v.(bool))
			case fieldDefaultString:
				m.SetDefaultString(v.(string))
			case fieldDefaultBytes:
				m.SetDefaultBytes(v.([]byte))
			case fieldDefaultNestedEnum:
				m.SetDefaultNestedEnum(testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)))
			case fieldDefaultForeignEnum:
				m.SetDefaultForeignEnum(testpb.ForeignEnum(v.(protoreflect.EnumNumber)))

			case fieldOneofUint32:
				m.SetOneofUint32(v.(uint32))
			case fieldOneofNestedMessage:
				m.SetOneofNestedMessage(v.(*testpb.TestAllTypes_NestedMessage))
			case fieldOneofString:
				m.SetOneofString(v.(string))
			case fieldOneofBytes:
				m.SetOneofBytes(v.([]byte))
			case fieldOneofBool:
				m.SetOneofBool(v.(bool))
			case fieldOneofUint64:
				m.SetOneofUint64(v.(uint64))
			case fieldOneofFloat:
				m.SetOneofFloat(v.(float32))
			case fieldOneofDouble:
				m.SetOneofDouble(v.(float64))
			case fieldOneofEnum:
				m.SetOneofEnum(testpb.TestAllTypes_NestedEnum(v.(protoreflect.EnumNumber)))
			case fieldOneofGroup:
				m.SetOneofgroup(v.(*testpb.TestAllTypes_OneofGroup))
			case fieldOneofOptionalUint32:
				m.SetOneofOptionalUint32(v.(uint32))

			default:
				panic(fmt.Sprintf("set: unknown field %d", num))
			}
		},
		clear: func(num protoreflect.FieldNumber) {
			switch num {
			case fieldSingularInt32:
				m.SetSingularInt32(0)
			case fieldSingularInt64:
				m.SetSingularInt64(0)
			case fieldSingularUint32:
				m.SetSingularUint32(0)
			case fieldSingularUint64:
				m.SetSingularUint64(0)
			case fieldSingularSint32:
				m.SetSingularSint32(0)
			case fieldSingularSint64:
				m.SetSingularSint64(0)
			case fieldSingularFixed32:
				m.SetSingularFixed32(0)
			case fieldSingularFixed64:
				m.SetSingularFixed64(0)
			case fieldSingularSfixed32:
				m.SetSingularSfixed32(0)
			case fieldSingularSfixed64:
				m.SetSingularSfixed64(0)
			case fieldSingularFloat:
				m.SetSingularFloat(0)
			case fieldSingularDouble:
				m.SetSingularDouble(0)
			case fieldSingularBool:
				m.SetSingularBool(false)
			case fieldSingularString:
				m.SetSingularString("")
			case fieldSingularBytes:
				m.SetSingularBytes(nil)
			case fieldSingularNestedEnum:
				m.SetSingularNestedEnum(testpb.TestAllTypes_FOO)
			case fieldSingularForeignEnum:
				m.SetSingularForeignEnum(testpb.ForeignEnum_FOREIGN_ZERO)
			case fieldSingularImportEnum:
				m.SetSingularImportEnum(testpb.ImportEnum_IMPORT_ZERO)

			case fieldOptionalInt32:
				m.ClearOptionalInt32()
			case fieldOptionalInt64:
				m.ClearOptionalInt64()
			case fieldOptionalUint32:
				m.ClearOptionalUint32()
			case fieldOptionalUint64:
				m.ClearOptionalUint64()
			case fieldOptionalSint32:
				m.ClearOptionalSint32()
			case fieldOptionalSint64:
				m.ClearOptionalSint64()
			case fieldOptionalFixed32:
				m.ClearOptionalFixed32()
			case fieldOptionalFixed64:
				m.ClearOptionalFixed64()
			case fieldOptionalSfixed32:
				m.ClearOptionalSfixed32()
			case fieldOptionalSfixed64:
				m.ClearOptionalSfixed64()
			case fieldOptionalFloat:
				m.ClearOptionalFloat()
			case fieldOptionalDouble:
				m.ClearOptionalDouble()
			case fieldOptionalBool:
				m.ClearOptionalBool()
			case fieldOptionalString:
				m.ClearOptionalString()
			case fieldOptionalBytes:
				m.ClearOptionalBytes()
			case fieldOptionalGroup:
				m.ClearOptionalgroup()
			case fieldNotGroupLikeDelimited:
				m.ClearNotGroupLikeDelimited()
			case fieldOptionalNestedMessage:
				m.ClearOptionalNestedMessage()
			case fieldOptionalForeignMessage:
				m.ClearOptionalForeignMessage()
			case fieldOptionalImportMessage:
				m.ClearOptionalImportMessage()
			case fieldOptionalNestedEnum:
				m.ClearOptionalNestedEnum()
			case fieldOptionalForeignEnum:
				m.ClearOptionalForeignEnum()
			case fieldOptionalImportEnum:
				m.ClearOptionalImportEnum()
			case fieldOptionalLazyNestedMessage:
				m.ClearOptionalLazyNestedMessage()

			case fieldRepeatedInt32:
				m.SetRepeatedInt32(nil)
			case fieldRepeatedInt64:
				m.SetRepeatedInt64(nil)
			case fieldRepeatedUint32:
				m.SetRepeatedUint32(nil)
			case fieldRepeatedUint64:
				m.SetRepeatedUint64(nil)
			case fieldRepeatedSint32:
				m.SetRepeatedSint32(nil)
			case fieldRepeatedSint64:
				m.SetRepeatedSint64(nil)
			case fieldRepeatedFixed32:
				m.SetRepeatedFixed32(nil)
			case fieldRepeatedFixed64:
				m.SetRepeatedFixed64(nil)
			case fieldRepeatedSfixed32:
				m.SetRepeatedSfixed32(nil)
			case fieldRepeatedSfixed64:
				m.SetRepeatedSfixed64(nil)
			case fieldRepeatedFloat:
				m.SetRepeatedFloat(nil)
			case fieldRepeatedDouble:
				m.SetRepeatedDouble(nil)
			case fieldRepeatedBool:
				m.SetRepeatedBool(nil)
			case fieldRepeatedString:
				m.SetRepeatedString(nil)
			case fieldRepeatedBytes:
				m.SetRepeatedBytes(nil)
			case fieldRepeatedGroup:
				m.SetRepeatedgroup(nil)
			case fieldRepeatedNestedMessage:
				m.SetRepeatedNestedMessage(nil)
			case fieldRepeatedForeignMessage:
				m.SetRepeatedForeignMessage(nil)
			case fieldRepeatedImportMessage:
				m.SetRepeatedImportmessage(nil)
			case fieldRepeatedNestedEnum:
				m.SetRepeatedNestedEnum(nil)
			case fieldRepeatedForeignEnum:
				m.SetRepeatedForeignEnum(nil)
			case fieldRepeatedImportEnum:
				m.SetRepeatedImportenum(nil)

			case fieldMapInt32Int32:
				m.SetMapInt32Int32(nil)
			case fieldMapInt64Int64:
				m.SetMapInt64Int64(nil)
			case fieldMapUint32Uint32:
				m.SetMapUint32Uint32(nil)
			case fieldMapUint64Uint64:
				m.SetMapUint64Uint64(nil)
			case fieldMapSint32Sint32:
				m.SetMapSint32Sint32(nil)
			case fieldMapSint64Sint64:
				m.SetMapSint64Sint64(nil)
			case fieldMapFixed32Fixed32:
				m.SetMapFixed32Fixed32(nil)
			case fieldMapFixed64Fixed64:
				m.SetMapFixed64Fixed64(nil)
			case fieldMapSfixed32Sfixed32:
				m.SetMapSfixed32Sfixed32(nil)
			case fieldMapSfixed64Sfixed64:
				m.SetMapSfixed64Sfixed64(nil)
			case fieldMapInt32Float:
				m.SetMapInt32Float(nil)
			case fieldMapInt32Double:
				m.SetMapInt32Double(nil)
			case fieldMapBoolBool:
				m.SetMapBoolBool(nil)
			case fieldMapStringString:
				m.SetMapStringString(nil)
			case fieldMapStringBytes:
				m.SetMapStringBytes(nil)
			case fieldMapStringNestedMessage:
				m.SetMapStringNestedMessage(nil)
			case fieldMapStringNestedEnum:
				m.SetMapStringNestedEnum(nil)

			case fieldDefaultInt32:
				m.ClearDefaultInt32()
			case fieldDefaultInt64:
				m.ClearDefaultInt64()
			case fieldDefaultUint32:
				m.ClearDefaultUint32()
			case fieldDefaultUint64:
				m.ClearDefaultUint64()
			case fieldDefaultSint32:
				m.ClearDefaultSint32()
			case fieldDefaultSint64:
				m.ClearDefaultSint64()
			case fieldDefaultFixed32:
				m.ClearDefaultFixed32()
			case fieldDefaultFixed64:
				m.ClearDefaultFixed64()
			case fieldDefaultSfixed32:
				m.ClearDefaultSfixed32()
			case fieldDefaultSfixed64:
				m.ClearDefaultSfixed64()
			case fieldDefaultFloat:
				m.ClearDefaultFloat()
			case fieldDefaultDouble:
				m.ClearDefaultDouble()
			case fieldDefaultBool:
				m.ClearDefaultBool()
			case fieldDefaultString:
				m.ClearDefaultString()
			case fieldDefaultBytes:
				m.ClearDefaultBytes()
			case fieldDefaultNestedEnum:
				m.ClearDefaultNestedEnum()
			case fieldDefaultForeignEnum:
				m.ClearDefaultForeignEnum()

			case fieldOneofUint32:
				m.ClearOneofUint32()
			case fieldOneofNestedMessage:
				m.ClearOneofNestedMessage()
			case fieldOneofString:
				m.ClearOneofString()
			case fieldOneofBytes:
				m.ClearOneofBytes()
			case fieldOneofBool:
				m.ClearOneofBool()
			case fieldOneofUint64:
				m.ClearOneofUint64()
			case fieldOneofFloat:
				m.ClearOneofFloat()
			case fieldOneofDouble:
				m.ClearOneofDouble()
			case fieldOneofEnum:
				m.ClearOneofEnum()
			case fieldOneofGroup:
				m.ClearOneofgroup()
			case fieldOneofOptionalUint32:
				m.ClearOneofOptionalUint32()

			default:
				panic(fmt.Sprintf("clear: unknown field %d", num))
			}
		},
	}
}
