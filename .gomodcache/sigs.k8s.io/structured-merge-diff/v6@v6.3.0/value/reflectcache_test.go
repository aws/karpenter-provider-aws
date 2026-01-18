/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package value

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type CustomValue struct {
	data []byte
}

// MarshalJSON has a value receiver on this type.
func (c CustomValue) MarshalJSON() ([]byte, error) {
	return c.data, nil
}

type CustomPointer struct {
	data []byte
}

// MarshalJSON has a pointer receiver on this type.
func (c *CustomPointer) MarshalJSON() ([]byte, error) {
	return c.data, nil
}

// Mimics https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/time.go.
type Time struct {
	time.Time
}

// ToUnstructured implements the value.UnstructuredConverter interface.
func (t Time) ToUnstructured() interface{} {
	if t.IsZero() {
		return nil
	}
	buf := make([]byte, 0, len(time.RFC3339))
	buf = t.UTC().AppendFormat(buf, time.RFC3339)
	return string(buf)
}

func TestToUnstructured(t *testing.T) {
	testcases := []struct {
		Data                 string
		Expected             interface{}
		ExpectedErrorMessage string
	}{
		{Data: `null`, Expected: nil},
		{Data: `true`, Expected: true},
		{Data: `false`, Expected: false},
		{Data: `[]`, Expected: []interface{}{}},
		{Data: `[1]`, Expected: []interface{}{int64(1)}},
		{Data: `{}`, Expected: map[string]interface{}{}},
		{Data: `{"a":1}`, Expected: map[string]interface{}{"a": int64(1)}},
		{Data: `0`, Expected: int64(0)},
		{Data: `0.0`, Expected: float64(0)},
		{Data: "{} \t\r\n", Expected: map[string]interface{}{}},
		{Data: "{} \t\r\n}", ExpectedErrorMessage: "error decoding object from json: unexpected trailing data at offset 6"},
		{Data: "{} \t\r\n{}", ExpectedErrorMessage: "error decoding object from json: unexpected trailing data at offset 6"},
		{Data: "[] \t\r\n", Expected: []interface{}{}},
		{Data: "[] \t\r\n]", ExpectedErrorMessage: "error decoding array from json: unexpected trailing data at offset 6"},
		{Data: "[] \t\r\n[]", ExpectedErrorMessage: "error decoding array from json: unexpected trailing data at offset 6"},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.Data, func(t *testing.T) {
			t.Parallel()
			custom := []interface{}{
				CustomValue{data: []byte(tc.Data)},
				&CustomValue{data: []byte(tc.Data)},
				&CustomPointer{data: []byte(tc.Data)},
			}
			for _, custom := range custom {
				rv := reflect.ValueOf(custom)
				result, err := TypeReflectEntryOf(rv.Type()).ToUnstructured(rv)
				if err != nil {
					if tc.ExpectedErrorMessage == "" {
						t.Fatal(err)
					} else if got := err.Error(); got != tc.ExpectedErrorMessage {
						t.Fatalf("expected error message %q but got %q", tc.ExpectedErrorMessage, got)
					}
				} else if tc.ExpectedErrorMessage != "" {
					t.Fatalf("expected error message %q but got nil error", tc.ExpectedErrorMessage)
				}
				if !reflect.DeepEqual(result, tc.Expected) {
					t.Errorf("expected %#v but got %#v", tc.Expected, result)
				}
			}
		})
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func TestTimeToUnstructured(t *testing.T) {
	testcases := []struct {
		Name     string
		Time     *time.Time
		Expected interface{}
	}{
		{Name: "nil", Time: nil, Expected: nil},
		{Name: "zero", Time: &time.Time{}, Expected: nil},
		{Name: "1", Time: timePtr(time.Time{}.Add(time.Second)), Expected: "0001-01-01T00:00:01Z"},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var time *Time
			rv := reflect.ValueOf(time)
			if tc.Time != nil {
				rv = reflect.ValueOf(Time{Time: *tc.Time})
			}
			result, err := TypeReflectEntryOf(rv.Type()).ToUnstructured(rv)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, tc.Expected) {
				t.Errorf("expected %#v but got %#v", tc.Expected, result)
			}
		})
	}
}

func TestTypeReflectEntryOf(t *testing.T) {
	testString := ""
	testCustomType := customOmitZeroType{}
	tests := map[string]struct {
		arg  interface{}
		want *TypeReflectCacheEntry
	}{
		"StructWithStringField": {
			arg: struct {
				F1 string `json:"f1"`
			}{},
			want: &TypeReflectCacheEntry{
				structFields: map[string]*FieldCacheEntry{
					"f1": {
						JsonName:  "f1",
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(testString),
						TypeEntry: &TypeReflectCacheEntry{},
					},
				},
				orderedStructFields: []*FieldCacheEntry{
					{
						JsonName:  "f1",
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(testString),
						TypeEntry: &TypeReflectCacheEntry{},
					},
				},
			},
		},
		"StructWith*StringFieldOmitempty": {
			arg: struct {
				F1 *string `json:"f1,omitempty"`
			}{},
			want: &TypeReflectCacheEntry{
				structFields: map[string]*FieldCacheEntry{
					"f1": {
						JsonName:    "f1",
						isOmitEmpty: true,
						fieldPath:   [][]int{{0}},
						fieldType:   reflect.TypeOf(&testString),
						TypeEntry:   &TypeReflectCacheEntry{},
					},
				},
				orderedStructFields: []*FieldCacheEntry{
					{
						JsonName:    "f1",
						isOmitEmpty: true,
						fieldPath:   [][]int{{0}},
						fieldType:   reflect.TypeOf(&testString),
						TypeEntry:   &TypeReflectCacheEntry{},
					},
				},
			},
		},
		"StructWith*StringFieldOmitzero": {
			arg: struct {
				F1 *string `json:"f1,omitzero"`
			}{},
			want: &TypeReflectCacheEntry{
				structFields: map[string]*FieldCacheEntry{
					"f1": {
						JsonName:  "f1",
						omitzero:  func(v reflect.Value) bool { return v.IsZero() },
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(&testString),
						TypeEntry: &TypeReflectCacheEntry{},
					},
				},
				orderedStructFields: []*FieldCacheEntry{
					{
						JsonName:  "f1",
						omitzero:  func(v reflect.Value) bool { return v.IsZero() },
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(&testString),
						TypeEntry: &TypeReflectCacheEntry{},
					},
				},
			},
		},
		"StructWith*CustomFieldOmitzero": {
			arg: struct {
				F1 customOmitZeroType `json:"f1,omitzero"`
			}{},
			want: &TypeReflectCacheEntry{
				structFields: map[string]*FieldCacheEntry{
					"f1": {
						JsonName:  "f1",
						omitzero:  func(v reflect.Value) bool { return false },
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(testCustomType),
						TypeEntry: &TypeReflectCacheEntry{
							structFields:        map[string]*FieldCacheEntry{},
							orderedStructFields: []*FieldCacheEntry{},
						},
					},
				},
				orderedStructFields: []*FieldCacheEntry{
					{
						JsonName:  "f1",
						omitzero:  func(v reflect.Value) bool { return false },
						fieldPath: [][]int{{0}},
						fieldType: reflect.TypeOf(testCustomType),
						TypeEntry: &TypeReflectCacheEntry{
							structFields:        map[string]*FieldCacheEntry{},
							orderedStructFields: []*FieldCacheEntry{},
						},
					},
				},
			},
		},
		"StructWithInlinedField": {
			arg: struct {
				F1 string `json:",inline"`
			}{},
			want: &TypeReflectCacheEntry{
				structFields:        map[string]*FieldCacheEntry{},
				orderedStructFields: []*FieldCacheEntry{},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := TypeReflectEntryOf(reflect.TypeOf(tt.arg))

			// evaluate non-comparable omitzero functions
			for k, v := range got.structFields {
				compareOmitZero(t, v.fieldType, v.omitzero, tt.want.structFields[k].omitzero)
			}
			for i, v := range got.orderedStructFields {
				compareOmitZero(t, v.fieldType, v.omitzero, tt.want.orderedStructFields[i].omitzero)
			}

			// clear non-comparable omitzero functions
			for k, v := range got.structFields {
				v.omitzero = nil
				tt.want.structFields[k].omitzero = nil
			}
			for i, v := range got.orderedStructFields {
				v.omitzero = nil
				tt.want.orderedStructFields[i].omitzero = nil
			}

			// compare remaining fields
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeReflectEntryOf() got\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

type customOmitZeroType struct {
}

func (c *customOmitZeroType) IsZero() bool {
	return false
}

func compareOmitZero(t *testing.T, fieldType reflect.Type, got, want func(reflect.Value) bool) {
	t.Helper()
	if (want == nil) != (got == nil) {
		t.Fatalf("wanted omitzero=%v, got omitzero=%v", (want == nil), (got == nil))
	}
	if want == nil {
		return
	}
	v := reflect.New(fieldType).Elem()
	if e, a := want(v), got(v); e != a {
		t.Fatalf("wanted omitzero()=%v, got omitzero()=%v", e, a)
	}
}

func TestUnmarshal(t *testing.T) {
	for _, tc := range []struct {
		JSON      string
		IntoType  reflect.Type
		Want      interface{}
		WantError bool
	}{
		{
			JSON:      "{}}",
			IntoType:  reflect.TypeOf([0]interface{}{}).Elem(),
			Want:      map[string]interface{}{},
			WantError: true,
		},
		{
			JSON:     `1.0`,
			IntoType: reflect.TypeOf(json.Number("")),
			Want:     json.Number("1.0"),
		},
		{
			JSON:     `1`,
			IntoType: reflect.TypeOf(json.Number("")),
			Want:     json.Number("1"),
		},
		{
			JSON:     `1.0`,
			IntoType: reflect.TypeOf(float64(0)),
			Want:     float64(1),
		},
		{
			JSON:     `1`,
			IntoType: reflect.TypeOf(float64(0)),
			Want:     float64(1),
		},
		{
			JSON:      `1.0`,
			IntoType:  reflect.TypeOf(int64(0)),
			Want:      int64(0),
			WantError: true,
		},
		{
			JSON:     `1`,
			IntoType: reflect.TypeOf(int64(0)),
			Want:     int64(1),
		},
		{
			JSON:     `1.0`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want:     float64(1),
		},
		{
			JSON:     `1`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want:     int64(1),
		},
		{
			JSON:     `[1.0,[1.0],{"":1.0}]`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want: []interface{}{
				float64(1),
				[]interface{}{float64(1)},
				map[string]interface{}{"": float64(1)},
			},
		},
		{
			JSON:     `[1.0,[1.0],{"":1.0}]`,
			IntoType: reflect.TypeOf([]interface{}{}),
			Want: []interface{}{
				float64(1),
				[]interface{}{float64(1)},
				map[string]interface{}{"": float64(1)},
			},
		},
		{
			JSON:     `[1,[1],{"":1}]`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want: []interface{}{
				int64(1),
				[]interface{}{int64(1)},
				map[string]interface{}{"": int64(1)},
			},
		},
		{
			JSON:     `[1,[1],{"":1}]`,
			IntoType: reflect.TypeOf([]interface{}{}),
			Want: []interface{}{
				int64(1),
				[]interface{}{int64(1)},
				map[string]interface{}{"": int64(1)},
			},
		},
		{
			JSON:     `{"x":1.0,"y":[1.0],"z":{"":1.0}}`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want: map[string]interface{}{
				"x": float64(1),
				"y": []interface{}{float64(1)},
				"z": map[string]interface{}{"": float64(1)},
			},
		},
		{
			JSON:     `{"x":1.0,"y":[1.0],"z":{"":1.0}}`,
			IntoType: reflect.TypeOf(map[string]interface{}{}),
			Want: map[string]interface{}{
				"x": float64(1),
				"y": []interface{}{float64(1)},
				"z": map[string]interface{}{"": float64(1)},
			},
		},
		{
			JSON:     `{"x":1,"y":[1],"z":{"":1}}`,
			IntoType: reflect.TypeOf([0]interface{}{}).Elem(),
			Want: map[string]interface{}{
				"x": int64(1),
				"y": []interface{}{int64(1)},
				"z": map[string]interface{}{"": int64(1)},
			},
		},
		{
			JSON:     `{"x":1,"y":[1],"z":{"":1}}`,
			IntoType: reflect.TypeOf(map[string]interface{}{}),
			Want: map[string]interface{}{
				"x": int64(1),
				"y": []interface{}{int64(1)},
				"z": map[string]interface{}{"": int64(1)},
			},
		},
	} {
		t.Run(fmt.Sprintf("%s into %v", tc.JSON, reflect.PointerTo(tc.IntoType)), func(t *testing.T) {
			into := reflect.New(tc.IntoType)
			if err := unmarshal([]byte(tc.JSON), into.Interface()); tc.WantError != (err != nil) {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := into.Elem().Interface(); !reflect.DeepEqual(tc.Want, got) {
				t.Errorf("want %#v (%T), got %#v (%T)", tc.Want, tc.Want, got, got)
			}
		})
	}
}
