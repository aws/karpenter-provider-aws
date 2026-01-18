// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"text/template"
)

func generateImplMessageOpaque() string {
	return mustExecute(messageOpaqueTemplate, GoTypes)
}

var messageOpaqueTemplate = template.Must(template.New("").Parse(`
func getterForOpaqueNullableScalar(mi *MessageInfo, index uint32, fd protoreflect.FieldDescriptor, fs reflect.StructField, conv Converter, fieldOffset offset) func(p pointer) protoreflect.Value {
	ft := fs.Type
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	if fd.Kind() == protoreflect.EnumKind {
		// Enums for nullable opaque types.
		return func(p pointer) protoreflect.Value {
			if p.IsNil() || !mi.present(p, index) {
				return conv.Zero()
			}
			rv := p.Apply(fieldOffset).AsValueOf(fs.Type).Elem()
			return conv.PBValueOf(rv)
		}
	}
	switch ft.Kind() {
{{range . }}
{{- if eq . "string"}}	case reflect.String:
{{- /* Handle string GoType -> bytes proto type specially */}}
		if fd.Kind() == protoreflect.BytesKind {
			return func(p pointer) protoreflect.Value {
				if p.IsNil() || !mi.present(p, index) {
					return conv.Zero()
				}
				x := p.Apply(fieldOffset).StringPtr()
				if *x == nil {
					return conv.Zero()
				}
				if len(**x) == 0 {
					return protoreflect.ValueOfBytes(nil)
				}
				return protoreflect.ValueOfBytes([]byte(**x))
			}
		}
{{else if eq . "[]byte" }}	case reflect.Slice:
{{- /* Handle []byte GoType -> string proto type specially */}}
		if fd.Kind() == protoreflect.StringKind {
			return func(p pointer) protoreflect.Value {
				if p.IsNil() || !mi.present(p, index) {
					return conv.Zero()
				}
				x := p.Apply(fieldOffset).Bytes()
				return protoreflect.ValueOfString(string(*x))
			}
		}
{{else}}	case {{.Kind}}:
{{end}}		       return func(p pointer) protoreflect.Value {
			if p.IsNil() || !mi.present(p, index) {
				return conv.Zero()
			}
			x := p.Apply(fieldOffset).{{.OpaqueNullablePointerMethod}}()
{{- if eq . "string"}}
			if *x == nil {
				return conv.Zero()
			}
{{- end}}
			return protoreflect.ValueOf{{.PointerMethod}}({{.OpaqueNullableStar}}*x)
		}
{{end}}	}
	panic("unexpected protobuf kind: "+ft.Kind().String())
}
`))
