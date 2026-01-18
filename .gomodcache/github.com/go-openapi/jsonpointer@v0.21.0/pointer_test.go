// Copyright 2013 sigu-399 ( https://github.com/sigu-399 )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author       sigu-399
// author-github  https://github.com/sigu-399
// author-mail    sigu.399@gmail.com
//
// repository-name  jsonpointer
// repository-desc  An implementation of JSON Pointer - Go language
//
// description    Automated tests on package.
//
// created        03-03-2013

package jsonpointer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	TestDocumentNBItems = 11
	TestNodeObjNBItems  = 4
	TestDocumentString  = `{
"foo": ["bar", "baz"],
"obj": { "a":1, "b":2, "c":[3,4], "d":[ {"e":9}, {"f":[50,51]} ] },
"": 0,
"a/b": 1,
"c%d": 2,
"e^f": 3,
"g|h": 4,
"i\\j": 5,
"k\"l": 6,
" ": 7,
"m~n": 8
}`
)

var testDocumentJSON any

type testStructJSON struct {
	Foo []string `json:"foo"`
	Obj struct {
		A int   `json:"a"`
		B int   `json:"b"`
		C []int `json:"c"`
		D []struct {
			E int   `json:"e"`
			F []int `json:"f"`
		} `json:"d"`
	} `json:"obj"`
}

type aliasedMap map[string]any

var testStructJSONDoc testStructJSON
var testStructJSONPtr *testStructJSON

func init() {
	if err := json.Unmarshal([]byte(TestDocumentString), &testDocumentJSON); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(TestDocumentString), &testStructJSONDoc); err != nil {
		panic(err)
	}

	testStructJSONPtr = &testStructJSONDoc
}

func TestEscaping(t *testing.T) {
	ins := []string{`/`, `/`, `/a~1b`, `/a~1b`, `/c%d`, `/e^f`, `/g|h`, `/i\j`, `/k"l`, `/ `, `/m~0n`}
	outs := []float64{0, 0, 1, 1, 2, 3, 4, 5, 6, 7, 8}

	for i := range ins {
		p, err := New(ins[i])
		require.NoError(t, err, "input: %v", ins[i])
		result, _, err := p.Get(testDocumentJSON)

		require.NoError(t, err, "input: %v", ins[i])
		assert.InDeltaf(t, outs[i], result, 1e-6, "input: %v", ins[i])
	}

}

func TestFullDocument(t *testing.T) {
	const in = ``

	p, err := New(in)
	require.NoErrorf(t, err, "New(%v) error %v", in, err)

	result, _, err := p.Get(testDocumentJSON)
	require.NoErrorf(t, err, "Get(%v) error %v", in, err)

	asMap, ok := result.(map[string]any)
	require.True(t, ok)
	require.Lenf(t, asMap, TestDocumentNBItems, "Get(%v) = %v, expect full document", in, result)

	result, _, err = p.get(testDocumentJSON, nil)
	require.NoErrorf(t, err, "Get(%v) error %v", in, err)

	asMap, ok = result.(map[string]any)
	require.True(t, ok)
	require.Lenf(t, asMap, TestDocumentNBItems, "Get(%v) = %v, expect full document", in, result)
}

func TestDecodedTokens(t *testing.T) {
	p, err := New("/obj/a~1b")
	require.NoError(t, err)
	assert.Equal(t, []string{"obj", "a/b"}, p.DecodedTokens())
}

func TestIsEmpty(t *testing.T) {
	p, err := New("")
	require.NoError(t, err)

	assert.True(t, p.IsEmpty())
	p, err = New("/obj")
	require.NoError(t, err)

	assert.False(t, p.IsEmpty())
}

func TestGetSingle(t *testing.T) {
	const in = `/obj`

	t.Run("should create a new JSON pointer", func(t *testing.T) {
		_, err := New(in)
		require.NoError(t, err)
	})

	t.Run(`should find token "obj" in JSON`, func(t *testing.T) {
		result, _, err := GetForToken(testDocumentJSON, "obj")
		require.NoError(t, err)
		assert.Len(t, result, TestNodeObjNBItems)
	})

	t.Run(`should find token "obj" in type alias interface`, func(t *testing.T) {
		type alias interface{}
		var in alias = testDocumentJSON
		result, _, err := GetForToken(in, "obj")
		require.NoError(t, err)
		assert.Len(t, result, TestNodeObjNBItems)
	})

	t.Run(`should find token "obj" in pointer to interface`, func(t *testing.T) {
		in := &testDocumentJSON
		result, _, err := GetForToken(in, "obj")
		require.NoError(t, err)
		assert.Len(t, result, TestNodeObjNBItems)
	})

	t.Run(`should not find token "Obj" in struct`, func(t *testing.T) {
		result, _, err := GetForToken(testStructJSONDoc, "Obj")
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run(`should not find token "Obj2" in struct`, func(t *testing.T) {
		result, _, err := GetForToken(testStructJSONDoc, "Obj2")
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run(`should not find token in nil`, func(t *testing.T) {
		result, _, err := GetForToken(nil, "obj")
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run(`should not find token in nil interface`, func(t *testing.T) {
		var in interface{}
		result, _, err := GetForToken(in, "obj")
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

type pointableImpl struct {
	a string
}

func (p pointableImpl) JSONLookup(token string) (any, error) {
	if token == "some" {
		return p.a, nil
	}
	return nil, fmt.Errorf("object has no field %q", token)
}

type pointableMap map[string]string

func (p pointableMap) JSONLookup(token string) (any, error) {
	if token == "swap" {
		return p["swapped"], nil
	}

	v, ok := p[token]
	if ok {
		return v, nil
	}

	return nil, fmt.Errorf("object has no key %q", token)
}

func TestPointableInterface(t *testing.T) {
	p := &pointableImpl{"hello"}

	result, _, err := GetForToken(p, "some")
	require.NoError(t, err)
	assert.Equal(t, p.a, result)

	result, _, err = GetForToken(p, "something")
	require.Error(t, err)
	assert.Nil(t, result)

	pm := pointableMap{"swapped": "hello", "a": "world"}
	result, _, err = GetForToken(pm, "swap")
	require.NoError(t, err)
	assert.Equal(t, pm["swapped"], result)

	result, _, err = GetForToken(pm, "a")
	require.NoError(t, err)
	assert.Equal(t, pm["a"], result)
}

func TestGetNode(t *testing.T) {
	const in = `/obj`

	p, err := New(in)
	require.NoError(t, err)

	result, _, err := p.Get(testDocumentJSON)
	require.NoError(t, err)
	assert.Len(t, result, TestNodeObjNBItems)

	result, _, err = p.Get(aliasedMap(testDocumentJSON.(map[string]any)))
	require.NoError(t, err)
	assert.Len(t, result, TestNodeObjNBItems)

	result, _, err = p.Get(testStructJSONDoc)
	require.NoError(t, err)
	assert.Equal(t, testStructJSONDoc.Obj, result)

	result, _, err = p.Get(testStructJSONPtr)
	require.NoError(t, err)
	assert.Equal(t, testStructJSONDoc.Obj, result)
}

func TestArray(t *testing.T) {
	ins := []string{`/foo/0`, `/foo/0`, `/foo/1`}
	outs := []string{"bar", "bar", "baz"}

	for i := range ins {
		p, err := New(ins[i])
		require.NoError(t, err)

		result, _, err := p.Get(testStructJSONDoc)
		require.NoError(t, err)
		assert.Equal(t, outs[i], result)

		result, _, err = p.Get(testStructJSONPtr)
		require.NoError(t, err)
		assert.Equal(t, outs[i], result)

		result, _, err = p.Get(testDocumentJSON)
		require.NoError(t, err)
		assert.Equal(t, outs[i], result)
	}
}

func TestOtherThings(t *testing.T) {
	_, err := New("abc")
	require.Error(t, err)

	p, err := New("")
	require.NoError(t, err)
	assert.Equal(t, "", p.String())

	p, err = New("/obj/a")
	require.NoError(t, err)
	assert.Equal(t, "/obj/a", p.String())

	s := Escape("m~n")
	assert.Equal(t, "m~0n", s)
	s = Escape("m/n")
	assert.Equal(t, "m~1n", s)

	p, err = New("/foo/3")
	require.NoError(t, err)
	_, _, err = p.Get(testDocumentJSON)
	require.Error(t, err)

	p, err = New("/foo/a")
	require.NoError(t, err)
	_, _, err = p.Get(testDocumentJSON)
	require.Error(t, err)

	p, err = New("/notthere")
	require.NoError(t, err)
	_, _, err = p.Get(testDocumentJSON)
	require.Error(t, err)

	p, err = New("/invalid")
	require.NoError(t, err)
	_, _, err = p.Get(1234)
	require.Error(t, err)

	p, err = New("/foo/1")
	require.NoError(t, err)
	expected := "hello"
	bbb := testDocumentJSON.(map[string]any)["foo"]
	bbb.([]any)[1] = "hello"

	v, _, err := p.Get(testDocumentJSON)
	require.NoError(t, err)
	assert.Equal(t, expected, v)

	esc := Escape("a/")
	assert.Equal(t, "a~1", esc)
	unesc := Unescape(esc)
	assert.Equal(t, "a/", unesc)

	unesc = Unescape("~01")
	assert.Equal(t, "~1", unesc)
	assert.Equal(t, "~0~1", Escape("~/"))
	assert.Equal(t, "~/", Unescape("~0~1"))
}

func TestObject(t *testing.T) {
	ins := []string{`/obj/a`, `/obj/b`, `/obj/c/0`, `/obj/c/1`, `/obj/c/1`, `/obj/d/1/f/0`}
	outs := []float64{1, 2, 3, 4, 4, 50}

	for i := range ins {
		p, err := New(ins[i])
		require.NoError(t, err)

		result, _, err := p.Get(testDocumentJSON)
		require.NoError(t, err)
		assert.InDelta(t, outs[i], result, 1e-6)

		result, _, err = p.Get(testStructJSONDoc)
		require.NoError(t, err)
		assert.InDelta(t, outs[i], result, 1e-6)

		result, _, err = p.Get(testStructJSONPtr)
		require.NoError(t, err)
		assert.InDelta(t, outs[i], result, 1e-6)
	}
}

/*
	type setJSONDocEle struct {
		B int `json:"b"`
		C int `json:"c"`
	}
*/
type setJSONDoc struct {
	A []struct {
		B int `json:"b"`
		C int `json:"c"`
	} `json:"a"`
	D int `json:"d"`
}

type settableDoc struct {
	Coll settableColl
	Int  settableInt
}

func (s settableDoc) MarshalJSON() ([]byte, error) {
	var res struct {
		A settableColl `json:"a"`
		D settableInt  `json:"d"`
	}
	res.A = s.Coll
	res.D = s.Int
	return json.Marshal(res)
}
func (s *settableDoc) UnmarshalJSON(data []byte) error {
	var res struct {
		A settableColl `json:"a"`
		D settableInt  `json:"d"`
	}

	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	s.Coll = res.A
	s.Int = res.D
	return nil
}

// JSONLookup implements an interface to customize json pointer lookup
func (s settableDoc) JSONLookup(token string) (any, error) {
	switch token {
	case "a":
		return &s.Coll, nil
	case "d":
		return &s.Int, nil
	default:
		return nil, fmt.Errorf("%s is not a known field", token)
	}
}

// JSONLookup implements an interface to customize json pointer lookup
func (s *settableDoc) JSONSet(token string, data any) error {
	switch token {
	case "a":
		switch dt := data.(type) {
		case settableColl:
			s.Coll = dt
			return nil
		case *settableColl:
			if dt != nil {
				s.Coll = *dt
			} else {
				s.Coll = settableColl{}
			}
			return nil
		case []settableCollItem:
			s.Coll.Items = dt
			return nil
		}
	case "d":
		switch dt := data.(type) {
		case settableInt:
			s.Int = dt
			return nil
		case int:
			s.Int.Value = dt
			return nil
		case int8:
			s.Int.Value = int(dt)
			return nil
		case int16:
			s.Int.Value = int(dt)
			return nil
		case int32:
			s.Int.Value = int(dt)
			return nil
		case int64:
			s.Int.Value = int(dt)
			return nil
		default:
			return fmt.Errorf("invalid type %T for %s", data, token)
		}
	}
	return fmt.Errorf("%s is not a known field", token)
}

type settableColl struct {
	Items []settableCollItem
}

func (s settableColl) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Items)
}
func (s *settableColl) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.Items)
}

// JSONLookup implements an interface to customize json pointer lookup
func (s settableColl) JSONLookup(token string) (any, error) {
	if tok, err := strconv.Atoi(token); err == nil {
		return &s.Items[tok], nil
	}
	return nil, fmt.Errorf("%s is not a valid index", token)
}

// JSONLookup implements an interface to customize json pointer lookup
func (s *settableColl) JSONSet(token string, data any) error {
	if _, err := strconv.Atoi(token); err == nil {
		_, err := SetForToken(s.Items, token, data)
		return err
	}
	return fmt.Errorf("%s is not a valid index", token)
}

type settableCollItem struct {
	B int `json:"b"`
	C int `json:"c"`
}

type settableInt struct {
	Value int
}

func (s settableInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Value)
}
func (s *settableInt) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.Value)
}

func TestSetNode(t *testing.T) {
	const jsonText = `{"a":[{"b": 1, "c": 2}], "d": 3}`

	var jsonDocument any
	require.NoError(t, json.Unmarshal([]byte(jsonText), &jsonDocument))

	t.Run("with set node c", func(t *testing.T) {
		const in = "/a/0/c"
		p, err := New(in)
		require.NoError(t, err)

		_, err = p.Set(jsonDocument, 999)
		require.NoError(t, err)

		firstNode, ok := jsonDocument.(map[string]any)
		require.True(t, ok)
		assert.Len(t, firstNode, 2)

		sliceNode, ok := firstNode["a"].([]any)
		require.True(t, ok)
		assert.Len(t, sliceNode, 1)

		changedNode, ok := sliceNode[0].(map[string]any)
		require.True(t, ok)
		chNodeVI := changedNode["c"]

		require.IsType(t, 0, chNodeVI)
		changedNodeValue := chNodeVI.(int)
		require.Equal(t, 999, changedNodeValue)
		assert.Len(t, sliceNode, 1)
	})

	t.Run("with set node 0 with map", func(t *testing.T) {
		v, err := New("/a/0")
		require.NoError(t, err)

		_, err = v.Set(jsonDocument, map[string]any{"b": 3, "c": 8})
		require.NoError(t, err)

		firstNode, ok := jsonDocument.(map[string]any)
		require.True(t, ok)
		assert.Len(t, firstNode, 2)

		sliceNode, ok := firstNode["a"].([]any)
		require.True(t, ok)
		assert.Len(t, sliceNode, 1)

		changedNode, ok := sliceNode[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 3, changedNode["b"])
		assert.Equal(t, 8, changedNode["c"])
	})

	t.Run("with struct", func(t *testing.T) {
		var structDoc setJSONDoc
		require.NoError(t, json.Unmarshal([]byte(jsonText), &structDoc))

		t.Run("with set array node", func(t *testing.T) {
			g, err := New("/a")
			require.NoError(t, err)

			_, err = g.Set(&structDoc, []struct {
				B int `json:"b"`
				C int `json:"c"`
			}{{B: 4, C: 7}})
			require.NoError(t, err)
			assert.Len(t, structDoc.A, 1)
			changedNode := structDoc.A[0]
			assert.Equal(t, 4, changedNode.B)
			assert.Equal(t, 7, changedNode.C)
		})

		t.Run("with set node 0 with struct", func(t *testing.T) {
			v, err := New("/a/0")
			require.NoError(t, err)

			_, err = v.Set(structDoc, struct {
				B int `json:"b"`
				C int `json:"c"`
			}{B: 3, C: 8})
			require.NoError(t, err)
			assert.Len(t, structDoc.A, 1)
			changedNode := structDoc.A[0]
			assert.Equal(t, 3, changedNode.B)
			assert.Equal(t, 8, changedNode.C)
		})

		t.Run("with set node c with struct", func(t *testing.T) {
			p, err := New("/a/0/c")
			require.NoError(t, err)

			_, err = p.Set(&structDoc, 999)
			require.NoError(t, err)

			require.Len(t, structDoc.A, 1)
			assert.Equal(t, 999, structDoc.A[0].C)
		})
	})

	t.Run("with Settable", func(t *testing.T) {
		var setDoc settableDoc
		require.NoError(t, json.Unmarshal([]byte(jsonText), &setDoc))

		t.Run("with array node a", func(t *testing.T) {
			g, err := New("/a")
			require.NoError(t, err)

			_, err = g.Set(&setDoc, []settableCollItem{{B: 4, C: 7}})
			require.NoError(t, err)
			assert.Len(t, setDoc.Coll.Items, 1)
			changedNode := setDoc.Coll.Items[0]
			assert.Equal(t, 4, changedNode.B)
			assert.Equal(t, 7, changedNode.C)
		})

		t.Run("with node 0", func(t *testing.T) {
			v, err := New("/a/0")
			require.NoError(t, err)

			_, err = v.Set(setDoc, settableCollItem{B: 3, C: 8})
			require.NoError(t, err)
			assert.Len(t, setDoc.Coll.Items, 1)
			changedNode := setDoc.Coll.Items[0]
			assert.Equal(t, 3, changedNode.B)
			assert.Equal(t, 8, changedNode.C)
		})

		t.Run("with node c", func(t *testing.T) {
			p, err := New("/a/0/c")
			require.NoError(t, err)
			_, err = p.Set(setDoc, 999)
			require.NoError(t, err)
			require.Len(t, setDoc.Coll.Items, 1)
			assert.Equal(t, 999, setDoc.Coll.Items[0].C)
		})
	})
}

func TestOffset(t *testing.T) {
	cases := []struct {
		name     string
		ptr      string
		input    string
		offset   int64
		hasError bool
	}{
		{
			name:   "object key",
			ptr:    "/foo/bar",
			input:  `{"foo": {"bar": 21}}`,
			offset: 9,
		},
		{
			name:   "complex object key",
			ptr:    "/paths/~1p~1{}/get",
			input:  `{"paths": {"foo": {"bar": 123, "baz": {}}, "/p/{}": {"get": {}}}}`,
			offset: 53,
		},
		{
			name:   "array index",
			ptr:    "/0/1",
			input:  `[[1,2], [3,4]]`,
			offset: 3,
		},
		{
			name:   "mix array index and object key",
			ptr:    "/0/1/foo/0",
			input:  `[[1, {"foo": ["a", "b"]}], [3, 4]]`,
			offset: 14,
		},
		{
			name:     "nonexist object key",
			ptr:      "/foo/baz",
			input:    `{"foo": {"bar": 21}}`,
			hasError: true,
		},
		{
			name:     "nonexist array index",
			ptr:      "/0/2",
			input:    `[[1,2], [3,4]]`,
			hasError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ptr, err := New(tt.ptr)
			require.NoError(t, err)

			offset, err := ptr.Offset(tt.input)
			if tt.hasError {
				require.Error(t, err)
				return
			}

			t.Log(offset, err)
			require.NoError(t, err)
			assert.Equal(t, tt.offset, offset)
		})
	}
}
