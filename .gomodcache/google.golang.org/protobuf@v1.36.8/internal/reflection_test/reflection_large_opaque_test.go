// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflection_test

import (
	"fmt"
	"testing"

	testpb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_opaque"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestLargeOpaqueConcrete(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newTestMessageLargeOpaque(nil).ProtoReflect().Type())
		})
	}
}

func TestLargeOpaqueReflection(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, (*testpb.TestManyMessageFieldsMessage)(nil).ProtoReflect().Type())
		})
	}
}

func TestLargeOpaqueShadow_GetConcrete_SetReflection(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
				m := &testpb.TestManyMessageFieldsMessage{}
				return newTestMessageLargeOpaque(m), m
			}).ProtoReflect().Type())
		})
	}
}

func TestLargeOpaqueShadow_GetReflection_SetConcrete(t *testing.T) {
	for _, tt := range lazyCombinations {
		t.Run(tt.desc, func(t *testing.T) {
			tt.ptm.Test(t, newShadow(func() (get, set protoreflect.ProtoMessage) {
				m := &testpb.TestManyMessageFieldsMessage{}
				return m, newTestMessageLargeOpaque(m)
			}).ProtoReflect().Type())
		})
	}
}

func newTestMessageLargeOpaque(m *testpb.TestManyMessageFieldsMessage) protoreflect.ProtoMessage {
	return &testProtoMessage{
		m:  m,
		md: m.ProtoReflect().Descriptor(),
		new: func() protoreflect.Message {
			return newTestMessageLargeOpaque(&testpb.TestManyMessageFieldsMessage{}).ProtoReflect()
		},
		has: func(num protoreflect.FieldNumber) bool {
			switch num {
			case largeFieldF1:
				return m.HasF1()
			case largeFieldF2:
				return m.HasF2()
			case largeFieldF3:
				return m.HasF3()
			case largeFieldF4:
				return m.HasF4()
			case largeFieldF5:
				return m.HasF5()
			case largeFieldF6:
				return m.HasF6()
			case largeFieldF7:
				return m.HasF7()
			case largeFieldF8:
				return m.HasF8()
			case largeFieldF9:
				return m.HasF9()
			case largeFieldF10:
				return m.HasF10()
			case largeFieldF11:
				return m.HasF11()
			case largeFieldF12:
				return m.HasF12()
			case largeFieldF13:
				return m.HasF13()
			case largeFieldF14:
				return m.HasF14()
			case largeFieldF15:
				return m.HasF15()
			case largeFieldF16:
				return m.HasF16()
			case largeFieldF17:
				return m.HasF17()
			case largeFieldF18:
				return m.HasF18()
			case largeFieldF19:
				return m.HasF19()
			case largeFieldF20:
				return m.HasF20()
			case largeFieldF21:
				return m.HasF21()
			case largeFieldF22:
				return m.HasF22()
			case largeFieldF23:
				return m.HasF23()
			case largeFieldF24:
				return m.HasF24()
			case largeFieldF25:
				return m.HasF25()
			case largeFieldF26:
				return m.HasF26()
			case largeFieldF27:
				return m.HasF27()
			case largeFieldF28:
				return m.HasF28()
			case largeFieldF29:
				return m.HasF29()
			case largeFieldF30:
				return m.HasF30()
			case largeFieldF31:
				return m.HasF31()
			case largeFieldF32:
				return m.HasF32()
			case largeFieldF33:
				return m.HasF33()
			case largeFieldF34:
				return m.HasF34()
			case largeFieldF35:
				return m.HasF35()
			case largeFieldF36:
				return m.HasF36()
			case largeFieldF37:
				return m.HasF37()
			case largeFieldF38:
				return m.HasF38()
			case largeFieldF39:
				return m.HasF39()
			case largeFieldF40:
				return m.HasF40()
			case largeFieldF41:
				return m.HasF41()
			case largeFieldF42:
				return m.HasF42()
			case largeFieldF43:
				return m.HasF43()
			case largeFieldF44:
				return m.HasF44()
			case largeFieldF45:
				return m.HasF45()
			case largeFieldF46:
				return m.HasF46()
			case largeFieldF47:
				return m.HasF47()
			case largeFieldF48:
				return m.HasF48()
			case largeFieldF49:
				return m.HasF49()
			case largeFieldF50:
				return m.HasF50()
			case largeFieldF51:
				return m.HasF51()
			case largeFieldF52:
				return m.HasF52()
			case largeFieldF53:
				return m.HasF53()
			case largeFieldF54:
				return m.HasF54()
			case largeFieldF55:
				return m.HasF55()
			case largeFieldF56:
				return m.HasF56()
			case largeFieldF57:
				return m.HasF57()
			case largeFieldF58:
				return m.HasF58()
			case largeFieldF59:
				return m.HasF59()
			case largeFieldF60:
				return m.HasF60()
			case largeFieldF60:
				return m.HasF60()
			case largeFieldF61:
				return m.HasF61()
			case largeFieldF62:
				return m.HasF62()
			case largeFieldF63:
				return m.HasF63()
			case largeFieldF64:
				return m.HasF64()
			case largeFieldF65:
				return m.HasF65()
			case largeFieldF66:
				return m.HasF66()
			case largeFieldF67:
				return m.HasF67()
			case largeFieldF68:
				return m.HasF68()
			case largeFieldF69:
				return m.HasF69()
			case largeFieldF70:
				return m.HasF70()
			case largeFieldF71:
				return m.HasF71()
			case largeFieldF72:
				return m.HasF72()
			case largeFieldF73:
				return m.HasF73()
			case largeFieldF74:
				return m.HasF74()
			case largeFieldF75:
				return m.HasF75()
			case largeFieldF76:
				return m.HasF76()
			case largeFieldF77:
				return m.HasF77()
			case largeFieldF78:
				return m.HasF78()
			case largeFieldF79:
				return m.HasF79()
			case largeFieldF80:
				return m.HasF80()
			case largeFieldF81:
				return m.HasF81()
			case largeFieldF82:
				return m.HasF82()
			case largeFieldF83:
				return m.HasF83()
			case largeFieldF84:
				return m.HasF84()
			case largeFieldF85:
				return m.HasF85()
			case largeFieldF86:
				return m.HasF86()
			case largeFieldF87:
				return m.HasF87()
			case largeFieldF88:
				return m.HasF88()
			case largeFieldF89:
				return m.HasF89()
			case largeFieldF90:
				return m.HasF90()
			case largeFieldF91:
				return m.HasF91()
			case largeFieldF92:
				return m.HasF92()
			case largeFieldF93:
				return m.HasF93()
			case largeFieldF94:
				return m.HasF94()
			case largeFieldF95:
				return m.HasF95()
			case largeFieldF96:
				return m.HasF96()
			case largeFieldF97:
				return m.HasF97()
			case largeFieldF98:
				return m.HasF98()
			case largeFieldF99:
				return m.HasF99()
			case largeFieldF100:
				return m.HasF100()

			default:
				panic(fmt.Sprintf("has: unknown field %d", num))
			}
		},
		get: func(num protoreflect.FieldNumber) any {
			switch num {
			case largeFieldF1:
				return m.GetF1()
			case largeFieldF2:
				return m.GetF2()
			case largeFieldF3:
				return m.GetF3()
			case largeFieldF4:
				return m.GetF4()
			case largeFieldF5:
				return m.GetF5()
			case largeFieldF6:
				return m.GetF6()
			case largeFieldF7:
				return m.GetF7()
			case largeFieldF8:
				return m.GetF8()
			case largeFieldF9:
				return m.GetF9()
			case largeFieldF10:
				return m.GetF10()
			case largeFieldF11:
				return m.GetF11()
			case largeFieldF12:
				return m.GetF12()
			case largeFieldF13:
				return m.GetF13()
			case largeFieldF14:
				return m.GetF14()
			case largeFieldF15:
				return m.GetF15()
			case largeFieldF16:
				return m.GetF16()
			case largeFieldF17:
				return m.GetF17()
			case largeFieldF18:
				return m.GetF18()
			case largeFieldF19:
				return m.GetF19()
			case largeFieldF20:
				return m.GetF20()
			case largeFieldF21:
				return m.GetF21()
			case largeFieldF22:
				return m.GetF22()
			case largeFieldF23:
				return m.GetF23()
			case largeFieldF24:
				return m.GetF24()
			case largeFieldF25:
				return m.GetF25()
			case largeFieldF26:
				return m.GetF26()
			case largeFieldF27:
				return m.GetF27()
			case largeFieldF28:
				return m.GetF28()
			case largeFieldF29:
				return m.GetF29()
			case largeFieldF30:
				return m.GetF30()
			case largeFieldF31:
				return m.GetF31()
			case largeFieldF32:
				return m.GetF32()
			case largeFieldF33:
				return m.GetF33()
			case largeFieldF34:
				return m.GetF34()
			case largeFieldF35:
				return m.GetF35()
			case largeFieldF36:
				return m.GetF36()
			case largeFieldF37:
				return m.GetF37()
			case largeFieldF38:
				return m.GetF38()
			case largeFieldF39:
				return m.GetF39()
			case largeFieldF40:
				return m.GetF40()
			case largeFieldF41:
				return m.GetF41()
			case largeFieldF42:
				return m.GetF42()
			case largeFieldF43:
				return m.GetF43()
			case largeFieldF44:
				return m.GetF44()
			case largeFieldF45:
				return m.GetF45()
			case largeFieldF46:
				return m.GetF46()
			case largeFieldF47:
				return m.GetF47()
			case largeFieldF48:
				return m.GetF48()
			case largeFieldF49:
				return m.GetF49()
			case largeFieldF50:
				return m.GetF50()
			case largeFieldF51:
				return m.GetF51()
			case largeFieldF52:
				return m.GetF52()
			case largeFieldF53:
				return m.GetF53()
			case largeFieldF54:
				return m.GetF54()
			case largeFieldF55:
				return m.GetF55()
			case largeFieldF56:
				return m.GetF56()
			case largeFieldF57:
				return m.GetF57()
			case largeFieldF58:
				return m.GetF58()
			case largeFieldF59:
				return m.GetF59()
			case largeFieldF60:
				return m.GetF60()
			case largeFieldF61:
				return m.GetF61()
			case largeFieldF62:
				return m.GetF62()
			case largeFieldF63:
				return m.GetF63()
			case largeFieldF64:
				return m.GetF64()
			case largeFieldF65:
				return m.GetF65()
			case largeFieldF66:
				return m.GetF66()
			case largeFieldF67:
				return m.GetF67()
			case largeFieldF68:
				return m.GetF68()
			case largeFieldF69:
				return m.GetF69()
			case largeFieldF70:
				return m.GetF70()
			case largeFieldF71:
				return m.GetF71()
			case largeFieldF72:
				return m.GetF72()
			case largeFieldF73:
				return m.GetF73()
			case largeFieldF74:
				return m.GetF74()
			case largeFieldF75:
				return m.GetF75()
			case largeFieldF76:
				return m.GetF76()
			case largeFieldF77:
				return m.GetF77()
			case largeFieldF78:
				return m.GetF78()
			case largeFieldF79:
				return m.GetF79()
			case largeFieldF80:
				return m.GetF80()
			case largeFieldF81:
				return m.GetF81()
			case largeFieldF82:
				return m.GetF82()
			case largeFieldF83:
				return m.GetF83()
			case largeFieldF84:
				return m.GetF84()
			case largeFieldF85:
				return m.GetF85()
			case largeFieldF86:
				return m.GetF86()
			case largeFieldF87:
				return m.GetF87()
			case largeFieldF88:
				return m.GetF88()
			case largeFieldF89:
				return m.GetF89()
			case largeFieldF90:
				return m.GetF90()
			case largeFieldF91:
				return m.GetF91()
			case largeFieldF92:
				return m.GetF92()
			case largeFieldF93:
				return m.GetF93()
			case largeFieldF94:
				return m.GetF94()
			case largeFieldF95:
				return m.GetF95()
			case largeFieldF96:
				return m.GetF96()
			case largeFieldF97:
				return m.GetF97()
			case largeFieldF98:
				return m.GetF98()
			case largeFieldF99:
				return m.GetF99()
			case largeFieldF100:
				return m.GetF100()

			default:
				panic(fmt.Sprintf("get: unknown field %d", num))
			}
		},
		set: func(num protoreflect.FieldNumber, v any) {
			switch num {
			case largeFieldF1:
				m.SetF1(v.(*testpb.TestAllTypes))
			case largeFieldF2:
				m.SetF2(v.(*testpb.TestAllTypes))
			case largeFieldF3:
				m.SetF3(v.(*testpb.TestAllTypes))
			case largeFieldF4:
				m.SetF4(v.(*testpb.TestAllTypes))
			case largeFieldF5:
				m.SetF5(v.(*testpb.TestAllTypes))
			case largeFieldF6:
				m.SetF6(v.(*testpb.TestAllTypes))
			case largeFieldF7:
				m.SetF7(v.(*testpb.TestAllTypes))
			case largeFieldF8:
				m.SetF8(v.(*testpb.TestAllTypes))
			case largeFieldF9:
				m.SetF9(v.(*testpb.TestAllTypes))
			case largeFieldF10:
				m.SetF10(v.(*testpb.TestAllTypes))
			case largeFieldF11:
				m.SetF11(v.(*testpb.TestAllTypes))
			case largeFieldF12:
				m.SetF12(v.(*testpb.TestAllTypes))
			case largeFieldF13:
				m.SetF13(v.(*testpb.TestAllTypes))
			case largeFieldF14:
				m.SetF14(v.(*testpb.TestAllTypes))
			case largeFieldF15:
				m.SetF15(v.(*testpb.TestAllTypes))
			case largeFieldF16:
				m.SetF16(v.(*testpb.TestAllTypes))
			case largeFieldF17:
				m.SetF17(v.(*testpb.TestAllTypes))
			case largeFieldF18:
				m.SetF18(v.(*testpb.TestAllTypes))
			case largeFieldF19:
				m.SetF19(v.(*testpb.TestAllTypes))
			case largeFieldF20:
				m.SetF20(v.(*testpb.TestAllTypes))
			case largeFieldF21:
				m.SetF21(v.(*testpb.TestAllTypes))
			case largeFieldF22:
				m.SetF22(v.(*testpb.TestAllTypes))
			case largeFieldF23:
				m.SetF23(v.(*testpb.TestAllTypes))
			case largeFieldF24:
				m.SetF24(v.(*testpb.TestAllTypes))
			case largeFieldF25:
				m.SetF25(v.(*testpb.TestAllTypes))
			case largeFieldF26:
				m.SetF26(v.(*testpb.TestAllTypes))
			case largeFieldF27:
				m.SetF27(v.(*testpb.TestAllTypes))
			case largeFieldF28:
				m.SetF28(v.(*testpb.TestAllTypes))
			case largeFieldF29:
				m.SetF29(v.(*testpb.TestAllTypes))
			case largeFieldF30:
				m.SetF30(v.(*testpb.TestAllTypes))
			case largeFieldF31:
				m.SetF31(v.(*testpb.TestAllTypes))
			case largeFieldF32:
				m.SetF32(v.(*testpb.TestAllTypes))
			case largeFieldF33:
				m.SetF33(v.(*testpb.TestAllTypes))
			case largeFieldF34:
				m.SetF34(v.(*testpb.TestAllTypes))
			case largeFieldF35:
				m.SetF35(v.(*testpb.TestAllTypes))
			case largeFieldF36:
				m.SetF36(v.(*testpb.TestAllTypes))
			case largeFieldF37:
				m.SetF37(v.(*testpb.TestAllTypes))
			case largeFieldF38:
				m.SetF38(v.(*testpb.TestAllTypes))
			case largeFieldF39:
				m.SetF39(v.(*testpb.TestAllTypes))
			case largeFieldF40:
				m.SetF40(v.(*testpb.TestAllTypes))
			case largeFieldF41:
				m.SetF41(v.(*testpb.TestAllTypes))
			case largeFieldF42:
				m.SetF42(v.(*testpb.TestAllTypes))
			case largeFieldF43:
				m.SetF43(v.(*testpb.TestAllTypes))
			case largeFieldF44:
				m.SetF44(v.(*testpb.TestAllTypes))
			case largeFieldF45:
				m.SetF45(v.(*testpb.TestAllTypes))
			case largeFieldF46:
				m.SetF46(v.(*testpb.TestAllTypes))
			case largeFieldF47:
				m.SetF47(v.(*testpb.TestAllTypes))
			case largeFieldF48:
				m.SetF48(v.(*testpb.TestAllTypes))
			case largeFieldF49:
				m.SetF49(v.(*testpb.TestAllTypes))
			case largeFieldF50:
				m.SetF50(v.(*testpb.TestAllTypes))
			case largeFieldF51:
				m.SetF51(v.(*testpb.TestAllTypes))
			case largeFieldF52:
				m.SetF52(v.(*testpb.TestAllTypes))
			case largeFieldF53:
				m.SetF53(v.(*testpb.TestAllTypes))
			case largeFieldF54:
				m.SetF54(v.(*testpb.TestAllTypes))
			case largeFieldF55:
				m.SetF55(v.(*testpb.TestAllTypes))
			case largeFieldF56:
				m.SetF56(v.(*testpb.TestAllTypes))
			case largeFieldF57:
				m.SetF57(v.(*testpb.TestAllTypes))
			case largeFieldF58:
				m.SetF58(v.(*testpb.TestAllTypes))
			case largeFieldF59:
				m.SetF59(v.(*testpb.TestAllTypes))
			case largeFieldF60:
				m.SetF60(v.(*testpb.TestAllTypes))
			case largeFieldF61:
				m.SetF61(v.(*testpb.TestAllTypes))
			case largeFieldF62:
				m.SetF62(v.(*testpb.TestAllTypes))
			case largeFieldF63:
				m.SetF63(v.(*testpb.TestAllTypes))
			case largeFieldF64:
				m.SetF64(v.(*testpb.TestAllTypes))
			case largeFieldF65:
				m.SetF65(v.(*testpb.TestAllTypes))
			case largeFieldF66:
				m.SetF66(v.(*testpb.TestAllTypes))
			case largeFieldF67:
				m.SetF67(v.(*testpb.TestAllTypes))
			case largeFieldF68:
				m.SetF68(v.(*testpb.TestAllTypes))
			case largeFieldF69:
				m.SetF69(v.(*testpb.TestAllTypes))
			case largeFieldF70:
				m.SetF70(v.(*testpb.TestAllTypes))
			case largeFieldF71:
				m.SetF71(v.(*testpb.TestAllTypes))
			case largeFieldF72:
				m.SetF72(v.(*testpb.TestAllTypes))
			case largeFieldF73:
				m.SetF73(v.(*testpb.TestAllTypes))
			case largeFieldF74:
				m.SetF74(v.(*testpb.TestAllTypes))
			case largeFieldF75:
				m.SetF75(v.(*testpb.TestAllTypes))
			case largeFieldF76:
				m.SetF76(v.(*testpb.TestAllTypes))
			case largeFieldF77:
				m.SetF77(v.(*testpb.TestAllTypes))
			case largeFieldF78:
				m.SetF78(v.(*testpb.TestAllTypes))
			case largeFieldF79:
				m.SetF79(v.(*testpb.TestAllTypes))
			case largeFieldF80:
				m.SetF80(v.(*testpb.TestAllTypes))
			case largeFieldF81:
				m.SetF81(v.(*testpb.TestAllTypes))
			case largeFieldF82:
				m.SetF82(v.(*testpb.TestAllTypes))
			case largeFieldF83:
				m.SetF83(v.(*testpb.TestAllTypes))
			case largeFieldF84:
				m.SetF84(v.(*testpb.TestAllTypes))
			case largeFieldF85:
				m.SetF85(v.(*testpb.TestAllTypes))
			case largeFieldF86:
				m.SetF86(v.(*testpb.TestAllTypes))
			case largeFieldF87:
				m.SetF87(v.(*testpb.TestAllTypes))
			case largeFieldF88:
				m.SetF88(v.(*testpb.TestAllTypes))
			case largeFieldF89:
				m.SetF89(v.(*testpb.TestAllTypes))
			case largeFieldF90:
				m.SetF90(v.(*testpb.TestAllTypes))
			case largeFieldF91:
				m.SetF91(v.(*testpb.TestAllTypes))
			case largeFieldF92:
				m.SetF92(v.(*testpb.TestAllTypes))
			case largeFieldF93:
				m.SetF93(v.(*testpb.TestAllTypes))
			case largeFieldF94:
				m.SetF94(v.(*testpb.TestAllTypes))
			case largeFieldF95:
				m.SetF95(v.(*testpb.TestAllTypes))
			case largeFieldF96:
				m.SetF96(v.(*testpb.TestAllTypes))
			case largeFieldF97:
				m.SetF97(v.(*testpb.TestAllTypes))
			case largeFieldF98:
				m.SetF98(v.(*testpb.TestAllTypes))
			case largeFieldF99:
				m.SetF99(v.(*testpb.TestAllTypes))
			case largeFieldF100:
				m.SetF100(v.(*testpb.TestAllTypes))

			default:
				panic(fmt.Sprintf("set: unknown field %d", num))
			}
		},
		clear: func(num protoreflect.FieldNumber) {
			switch num {
			case largeFieldF1:
				m.ClearF1()
			case largeFieldF2:
				m.ClearF2()
			case largeFieldF3:
				m.ClearF3()
			case largeFieldF4:
				m.ClearF4()
			case largeFieldF5:
				m.ClearF5()
			case largeFieldF6:
				m.ClearF6()
			case largeFieldF7:
				m.ClearF7()
			case largeFieldF8:
				m.ClearF8()
			case largeFieldF9:
				m.ClearF9()
			case largeFieldF10:
				m.ClearF10()
			case largeFieldF11:
				m.ClearF11()
			case largeFieldF12:
				m.ClearF12()
			case largeFieldF13:
				m.ClearF13()
			case largeFieldF14:
				m.ClearF14()
			case largeFieldF15:
				m.ClearF15()
			case largeFieldF16:
				m.ClearF16()
			case largeFieldF17:
				m.ClearF17()
			case largeFieldF18:
				m.ClearF18()
			case largeFieldF19:
				m.ClearF19()
			case largeFieldF20:
				m.ClearF20()
			case largeFieldF21:
				m.ClearF21()
			case largeFieldF22:
				m.ClearF22()
			case largeFieldF23:
				m.ClearF23()
			case largeFieldF24:
				m.ClearF24()
			case largeFieldF25:
				m.ClearF25()
			case largeFieldF26:
				m.ClearF26()
			case largeFieldF27:
				m.ClearF27()
			case largeFieldF28:
				m.ClearF28()
			case largeFieldF29:
				m.ClearF29()
			case largeFieldF30:
				m.ClearF30()
			case largeFieldF31:
				m.ClearF31()
			case largeFieldF32:
				m.ClearF32()
			case largeFieldF33:
				m.ClearF33()
			case largeFieldF34:
				m.ClearF34()
			case largeFieldF35:
				m.ClearF35()
			case largeFieldF36:
				m.ClearF36()
			case largeFieldF37:
				m.ClearF37()
			case largeFieldF38:
				m.ClearF38()
			case largeFieldF39:
				m.ClearF39()
			case largeFieldF40:
				m.ClearF40()
			case largeFieldF41:
				m.ClearF41()
			case largeFieldF42:
				m.ClearF42()
			case largeFieldF43:
				m.ClearF43()
			case largeFieldF44:
				m.ClearF44()
			case largeFieldF45:
				m.ClearF45()
			case largeFieldF46:
				m.ClearF46()
			case largeFieldF47:
				m.ClearF47()
			case largeFieldF48:
				m.ClearF48()
			case largeFieldF49:
				m.ClearF49()
			case largeFieldF50:
				m.ClearF50()
			case largeFieldF51:
				m.ClearF51()
			case largeFieldF52:
				m.ClearF52()
			case largeFieldF53:
				m.ClearF53()
			case largeFieldF54:
				m.ClearF54()
			case largeFieldF55:
				m.ClearF55()
			case largeFieldF56:
				m.ClearF56()
			case largeFieldF57:
				m.ClearF57()
			case largeFieldF58:
				m.ClearF58()
			case largeFieldF59:
				m.ClearF59()
			case largeFieldF60:
				m.ClearF60()
			case largeFieldF60:
				m.ClearF60()
			case largeFieldF61:
				m.ClearF61()
			case largeFieldF62:
				m.ClearF62()
			case largeFieldF63:
				m.ClearF63()
			case largeFieldF64:
				m.ClearF64()
			case largeFieldF65:
				m.ClearF65()
			case largeFieldF66:
				m.ClearF66()
			case largeFieldF67:
				m.ClearF67()
			case largeFieldF68:
				m.ClearF68()
			case largeFieldF69:
				m.ClearF69()
			case largeFieldF70:
				m.ClearF70()
			case largeFieldF71:
				m.ClearF71()
			case largeFieldF72:
				m.ClearF72()
			case largeFieldF73:
				m.ClearF73()
			case largeFieldF74:
				m.ClearF74()
			case largeFieldF75:
				m.ClearF75()
			case largeFieldF76:
				m.ClearF76()
			case largeFieldF77:
				m.ClearF77()
			case largeFieldF78:
				m.ClearF78()
			case largeFieldF79:
				m.ClearF79()
			case largeFieldF80:
				m.ClearF80()
			case largeFieldF81:
				m.ClearF81()
			case largeFieldF82:
				m.ClearF82()
			case largeFieldF83:
				m.ClearF83()
			case largeFieldF84:
				m.ClearF84()
			case largeFieldF85:
				m.ClearF85()
			case largeFieldF86:
				m.ClearF86()
			case largeFieldF87:
				m.ClearF87()
			case largeFieldF88:
				m.ClearF88()
			case largeFieldF89:
				m.ClearF89()
			case largeFieldF90:
				m.ClearF90()
			case largeFieldF91:
				m.ClearF91()
			case largeFieldF92:
				m.ClearF92()
			case largeFieldF93:
				m.ClearF93()
			case largeFieldF94:
				m.ClearF94()
			case largeFieldF95:
				m.ClearF95()
			case largeFieldF96:
				m.ClearF96()
			case largeFieldF97:
				m.ClearF97()
			case largeFieldF98:
				m.ClearF98()
			case largeFieldF99:
				m.ClearF99()
			case largeFieldF100:
				m.ClearF100()

			default:
				panic(fmt.Sprintf("clear: unknown field %d", num))
			}
		},
	}
}
