// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package swag

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNameStruct struct {
	Name       string `json:"name"`
	NotTheSame int64  `json:"plain"`
	Ignored    string `json:"-"`
}

func TestNameProvider(t *testing.T) {

	provider := NewNameProvider()

	var obj = testNameStruct{}

	nm, ok := provider.GetGoName(obj, "name")
	assert.True(t, ok)
	assert.Equal(t, "Name", nm)

	nm, ok = provider.GetGoName(obj, "plain")
	assert.True(t, ok)
	assert.Equal(t, "NotTheSame", nm)

	nm, ok = provider.GetGoName(obj, "doesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetGoName(obj, "ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	tpe := reflect.TypeOf(obj)
	nm, ok = provider.GetGoNameForType(tpe, "name")
	assert.True(t, ok)
	assert.Equal(t, "Name", nm)

	nm, ok = provider.GetGoNameForType(tpe, "plain")
	assert.True(t, ok)
	assert.Equal(t, "NotTheSame", nm)

	nm, ok = provider.GetGoNameForType(tpe, "doesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetGoNameForType(tpe, "ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	ptr := &obj
	nm, ok = provider.GetGoName(ptr, "name")
	assert.True(t, ok)
	assert.Equal(t, "Name", nm)

	nm, ok = provider.GetGoName(ptr, "plain")
	assert.True(t, ok)
	assert.Equal(t, "NotTheSame", nm)

	nm, ok = provider.GetGoName(ptr, "doesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetGoName(ptr, "ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONName(obj, "Name")
	assert.True(t, ok)
	assert.Equal(t, "name", nm)

	nm, ok = provider.GetJSONName(obj, "NotTheSame")
	assert.True(t, ok)
	assert.Equal(t, "plain", nm)

	nm, ok = provider.GetJSONName(obj, "DoesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONName(obj, "Ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONNameForType(tpe, "Name")
	assert.True(t, ok)
	assert.Equal(t, "name", nm)

	nm, ok = provider.GetJSONNameForType(tpe, "NotTheSame")
	assert.True(t, ok)
	assert.Equal(t, "plain", nm)

	nm, ok = provider.GetJSONNameForType(tpe, "doesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONNameForType(tpe, "Ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONName(ptr, "Name")
	assert.True(t, ok)
	assert.Equal(t, "name", nm)

	nm, ok = provider.GetJSONName(ptr, "NotTheSame")
	assert.True(t, ok)
	assert.Equal(t, "plain", nm)

	nm, ok = provider.GetJSONName(ptr, "doesNotExist")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nm, ok = provider.GetJSONName(ptr, "Ignored")
	assert.False(t, ok)
	assert.Empty(t, nm)

	nms := provider.GetJSONNames(ptr)
	assert.Len(t, nms, 2)

	assert.Len(t, provider.index, 1)

}

func TestJSONConcatenation(t *testing.T) {
	assert.Nil(t, ConcatJSON())
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`)), []byte(`{"id":1}`))
	assert.Equal(t, ConcatJSON([]byte(`{}`), []byte(`{}`)), []byte(`{}`))
	assert.Equal(t, ConcatJSON([]byte(`[]`), []byte(`[]`)), []byte(`[]`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`), []byte(`{"name":"Rachel"}`)), []byte(`{"id":1,"name":"Rachel"}`))
	assert.Equal(t, ConcatJSON([]byte(`[{"id":1}]`), []byte(`[{"name":"Rachel"}]`)), []byte(`[{"id":1},{"name":"Rachel"}]`))
	assert.Equal(t, ConcatJSON([]byte(`{}`), []byte(`{"name":"Rachel"}`)), []byte(`{"name":"Rachel"}`))
	assert.Equal(t, ConcatJSON([]byte(`[]`), []byte(`[{"name":"Rachel"}]`)), []byte(`[{"name":"Rachel"}]`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`), []byte(`{}`)), []byte(`{"id":1}`))
	assert.Equal(t, ConcatJSON([]byte(`[{"id":1}]`), []byte(`[]`)), []byte(`[{"id":1}]`))
	assert.Equal(t, ConcatJSON([]byte(`{}`), []byte(`{}`), []byte(`{}`)), []byte(`{}`))
	assert.Equal(t, ConcatJSON([]byte(`[]`), []byte(`[]`), []byte(`[]`)), []byte(`[]`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`), []byte(`{"name":"Rachel"}`), []byte(`{"age":32}`)), []byte(`{"id":1,"name":"Rachel","age":32}`))
	assert.Equal(t, ConcatJSON([]byte(`[{"id":1}]`), []byte(`[{"name":"Rachel"}]`), []byte(`[{"age":32}]`)), []byte(`[{"id":1},{"name":"Rachel"},{"age":32}]`))
	assert.Equal(t, ConcatJSON([]byte(`{}`), []byte(`{"name":"Rachel"}`), []byte(`{"age":32}`)), []byte(`{"name":"Rachel","age":32}`))
	assert.Equal(t, ConcatJSON([]byte(`[]`), []byte(`[{"name":"Rachel"}]`), []byte(`[{"age":32}]`)), []byte(`[{"name":"Rachel"},{"age":32}]`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`), []byte(`{}`), []byte(`{"age":32}`)), []byte(`{"id":1,"age":32}`))
	assert.Equal(t, ConcatJSON([]byte(`[{"id":1}]`), []byte(`[]`), []byte(`[{"age":32}]`)), []byte(`[{"id":1},{"age":32}]`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":1}`), []byte(`{"name":"Rachel"}`), []byte(`{}`)), []byte(`{"id":1,"name":"Rachel"}`))
	assert.Equal(t, ConcatJSON([]byte(`[{"id":1}]`), []byte(`[{"name":"Rachel"}]`), []byte(`[]`)), []byte(`[{"id":1},{"name":"Rachel"}]`))

	// add test on null
	assert.Equal(t, ConcatJSON([]byte(nil)), []byte(nil))
	assert.Equal(t, ConcatJSON([]byte(`null`)), []byte(nil))
	assert.Equal(t, ConcatJSON([]byte(nil), []byte(`null`)), []byte(nil))
	assert.Equal(t, ConcatJSON([]byte(`{"id":null}`), []byte(`null`)), []byte(`{"id":null}`))
	assert.Equal(t, ConcatJSON([]byte(`{"id":null}`), []byte(`null`), []byte(`{"name":"Rachel"}`)), []byte(`{"id":null,"name":"Rachel"}`))
}

type SharedCounters struct {
	Counter1 int64 `json:"counter1,omitempty"`
	Counter2 int64 `json:"counter2:,omitempty"`
}

type AggregationObject struct {
	SharedCounters
	Count int64 `json:"count,omitempty"`
}

func (m *AggregationObject) UnmarshalJSON(raw []byte) error {
	// AO0
	var aO0 SharedCounters
	if err := ReadJSON(raw, &aO0); err != nil {
		return err
	}
	m.SharedCounters = aO0

	// now for regular properties
	var propsAggregationObject struct {
		Count int64 `json:"count,omitempty"`
	}
	if err := ReadJSON(raw, &propsAggregationObject); err != nil {
		return err
	}
	m.Count = propsAggregationObject.Count

	return nil
}

// MarshalJSON marshals this object to a JSON structure
func (m AggregationObject) MarshalJSON() ([]byte, error) {
	_parts := make([][]byte, 0, 1)

	aO0, err := WriteJSON(m.SharedCounters)
	if err != nil {
		return nil, err
	}
	_parts = append(_parts, aO0)

	// now for regular properties
	var propsAggregationObject struct {
		Count int64 `json:"count,omitempty"`
	}
	propsAggregationObject.Count = m.Count

	jsonDataPropsAggregationObject, errAggregationObject := WriteJSON(propsAggregationObject)
	if errAggregationObject != nil {
		return nil, errAggregationObject
	}
	_parts = append(_parts, jsonDataPropsAggregationObject)
	return ConcatJSON(_parts...), nil
}

func TestIssue2350(t *testing.T) {
	obj := AggregationObject{Count: 290, SharedCounters: SharedCounters{Counter1: 304, Counter2: 948}}

	rtjson, err := WriteJSON(obj)
	require.NoError(t, err)

	otjson, err := obj.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, string(rtjson), string(otjson))

	var obj1 AggregationObject
	require.NoError(t, ReadJSON(rtjson, &obj1))
	require.Equal(t, obj, obj1)

	var obj11 AggregationObject
	require.NoError(t, obj11.UnmarshalJSON(rtjson))
	require.Equal(t, obj, obj11)

	jsons := `{"counter1":123,"counter2:":456,"count":999}`
	var obj2 AggregationObject
	require.NoError(t, ReadJSON([]byte(jsons), &obj2))
	require.Equal(t, AggregationObject{SharedCounters: SharedCounters{Counter1: 123, Counter2: 456}, Count: 999}, obj2)
}
