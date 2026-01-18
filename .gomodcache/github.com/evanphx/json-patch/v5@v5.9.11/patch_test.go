package jsonpatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func reformatJSON(j string) string {
	buf := new(bytes.Buffer)

	json.Indent(buf, []byte(j), "", "  ")

	return buf.String()
}

func compareJSON(a, b string) bool {
	// return Equal([]byte(a), []byte(b))

	var objA, objB interface{}
	json.Unmarshal([]byte(a), &objA)
	json.Unmarshal([]byte(b), &objB)

	// fmt.Printf("Comparing %#v\nagainst %#v\n", objA, objB)
	return reflect.DeepEqual(objA, objB)
}

func applyPatch(doc, patch string) (string, error) {
	obj, err := DecodePatch([]byte(patch))

	if err != nil {
		return "", err
	}

	out, err := obj.Apply([]byte(doc))

	if err != nil {
		return "", err
	}

	return string(out), nil
}

func applyPatchIndented(doc, patch string) (string, error) {
	obj, err := DecodePatch([]byte(patch))

	if err != nil {
		return "", err
	}

	out, err := obj.ApplyIndent([]byte(doc), "  ")

	if err != nil {
		return "", err
	}

	return string(out), nil
}

func applyPatchWithOptions(doc, patch string, options *ApplyOptions) (string, error) {
	obj, err := DecodePatch([]byte(patch))

	if err != nil {
		return "", err
	}

	out, err := obj.ApplyWithOptions([]byte(doc), options)

	if err != nil {
		return "", err
	}

	return string(out), nil
}

type Case struct {
	doc, patch, result       string
	allowMissingPathOnRemove bool
	ensurePathExistsOnAdd    bool
}

func repeatedA(r int) string {
	var s string
	for i := 0; i < r; i++ {
		s += "A"
	}
	return s
}

var Cases = []Case{
	{
		``,
		`[
         { "op": "add", "path": "/baz", "value": "qux" }
     ]`,
		``,
		false,
		false,
	},
	{
		`{ "foo": "bar"}`,
		`[
	       { "op": "add", "path": "/baz", "value": "qux" }
	   ]`,
		`{
	     "baz": "qux",
	     "foo": "bar"
		 }`,
		false,
		false,
	},
	{
		`{ "foo": [ "bar", "baz" ] }`,
		`[
	   { "op": "add", "path": "/foo/1", "value": "qux" }
	  ]`,
		`{ "foo": [ "bar", "qux", "baz" ] }`,
		false,
		false,
	},
	{
		`{ "foo": [ "bar", "baz" ] }`,
		`[
	   { "op": "add", "path": "/foo/-1", "value": "qux" }
	  ]`,
		`{ "foo": [ "bar", "baz", "qux" ] }`,
		false,
		false,
	},
	{
		`{ "baz": "qux", "foo": "bar" }`,
		`[ { "op": "remove", "path": "/baz" } ]`,
		`{ "foo": "bar" }`,
		false,
		false,
	},
	{
		`{ "foo": [ "bar", "qux", "baz" ] }`,
		`[ { "op": "remove", "path": "/foo/1" } ]`,
		`{ "foo": [ "bar", "baz" ] }`,
		false,
		false,
	},
	{
		`{ "foo": [ "bar", "qux", "baz" ] }`,
		`[ { "op": "remove", "path": "/foo/-1" } ]`,
		`{ "foo": [ "bar", "qux" ] }`,
		false,
		false,
	},
	{
		`{ "foo": [ "bar", "qux", {"a": "abc", "b": "xyz" } ] }`,
		`[ { "op": "remove", "path": "/foo/-1/a" } ]`,
		`{ "foo": [ "bar", "qux", {"b": "xyz" } ] }`,
		false,
		false,
	},
	{
		`{ "baz": "qux", "foo": "bar" }`,
		`[ { "op": "replace", "path": "/baz", "value": "boo" } ]`,
		`{ "baz": "boo", "foo": "bar" }`,
		false,
		false,
	},
	{
		`{
	   "foo": {
	     "bar": "baz",
	     "waldo": "fred"
	   },
	   "qux": {
	     "corge": "grault"
	   }
	 }`,
		`[ { "op": "move", "from": "/foo/waldo", "path": "/qux/thud" } ]`,
		`{
	   "foo": {
	     "bar": "baz"
	   },
	   "qux": {
	     "corge": "grault",
	     "thud": "fred"
	   }
	 }`,
		false,
		false,
	},
	{
		`{ "foo": [ "all", "grass", "cows", "eat" ] }`,
		`[ { "op": "move", "from": "/foo/1", "path": "/foo/3" } ]`,
		`{ "foo": [ "all", "cows", "eat", "grass" ] }`,
		false,
		false,
	},
	{
		`{ "foo": [ "all", "grass", "cows", "eat" ] }`,
		`[ { "op": "move", "from": "/foo/1", "path": "/foo/2" } ]`,
		`{ "foo": [ "all", "cows", "grass", "eat" ] }`,
		false,
		false,
	},
	{
		`{ "foo": "bar" }`,
		`[ { "op": "add", "path": "/child", "value": { "grandchild": { } } } ]`,
		`{ "foo": "bar", "child": { "grandchild": { } } }`,
		false,
		false,
	},
	{
		`{ "foo": ["bar"] }`,
		`[ { "op": "add", "path": "/foo/-", "value": ["abc", "def"] } ]`,
		`{ "foo": ["bar", ["abc", "def"]] }`,
		false,
		false,
	},
	{
		`{ "foo": [{"bar": [{"baz0": "123"}]}]}`,
		`[ { "op": "add", "path": "/foo/0/bar/-", "value": {"baz1": "456"} } ]`,
		`{ "foo": [{"bar": [{"baz0": "123"}, {"baz1": "456"}]}]}`,
		true,
		true,
	},
	{
		`{ "foo": [{"bar": [{"baz0": "123"}]}]}`,
		`[ { "op": "add", "path": "/foo/1/bar/0", "value": {"baz1": "456"} } ]`,
		`{ "foo": [{"bar": [{"baz0": "123"}]}, {"bar": [{"baz1": "456"}]}]}`,
		true,
		true,
	},
	{
		`{ "foo": [{"bar": [{"baz0": "123"}]}]}`,
		`[ { "op": "add", "path": "/foo/1/bar/-1", "value": {"baz1": "456"} } ]`,
		`{ "foo": [{"bar": [{"baz0": "123"}]}, {"bar": [{"baz1": "456"}]}]}`,
		true,
		true,
	},
	{
		`{ "foo": [{"bar": [{"baz0": "123"}]}]}`,
		`[ { "op": "add", "path": "/foo/1/bar/-", "value": {"baz1": "456"} } ]`,
		`{ "foo": [{"bar": [{"baz0": "123"}]}, {"bar": [{"baz1": "456"}]}]}`,
		true,
		true,
	},
	{
		`{ "foo": "bar", "qux": { "baz": 1, "bar": null } }`,
		`[ { "op": "remove", "path": "/qux/bar" } ]`,
		`{ "foo": "bar", "qux": { "baz": 1 } }`,
		false,
		false,
	},
	{
		`{ "foo": "bar" }`,
		`[ { "op": "add", "path": "/baz", "value": null } ]`,
		`{ "baz": null, "foo": "bar" }`,
		false,
		false,
	},
	{
		`{ "foo": ["bar"]}`,
		`[ { "op": "replace", "path": "/foo/0", "value": "baz"}]`,
		`{ "foo": ["baz"]}`,
		false,
		false,
	},
	{
		`{ "foo": ["bar"]}`,
		`[ { "op": "replace", "path": "/foo/-1", "value": "baz"}]`,
		`{ "foo": ["baz"]}`,
		false,
		false,
	},
	{
		`{ "foo": [{"bar": "x"}]}`,
		`[ { "op": "replace", "path": "/foo/-1/bar", "value": "baz"}]`,
		`{ "foo": [{"bar": "baz"}]}`,
		false,
		false,
	},
	{
		`{ "foo": ["bar","baz"]}`,
		`[ { "op": "replace", "path": "/foo/0", "value": "bum"}]`,
		`{ "foo": ["bum","baz"]}`,
		false,
		false,
	},
	{
		`{ "foo": ["bar","qux","baz"]}`,
		`[ { "op": "replace", "path": "/foo/1", "value": "bum"}]`,
		`{ "foo": ["bar", "bum","baz"]}`,
		false,
		false,
	},
	{
		`[ {"foo": ["bar","qux","baz"]}]`,
		`[ { "op": "replace", "path": "/0/foo/0", "value": "bum"}]`,
		`[ {"foo": ["bum","qux","baz"]}]`,
		false,
		false,
	},
	{
		`[ {"foo": ["bar","qux","baz"], "bar": ["qux","baz"]}]`,
		`[ { "op": "copy", "from": "/0/foo/0", "path": "/0/bar/0"}]`,
		`[ {"foo": ["bar","qux","baz"], "bar": ["bar", "qux", "baz"]}]`,
		false,
		false,
	},
	{
		`[ {"foo": ["bar","qux","baz"], "bar": ["qux","baz"]}]`,
		`[ { "op": "copy", "from": "/0/foo/0", "path": "/0/bar"}]`,
		`[ {"foo": ["bar","qux","baz"], "bar": "bar"}]`,
		false,
		false,
	},
	{
		`[ { "foo": {"bar": ["qux","baz"]}, "baz": {"qux": "bum"}}]`,
		`[ { "op": "copy", "from": "/0/foo/bar", "path": "/0/baz/bar"}]`,
		`[ { "baz": {"bar": ["qux","baz"], "qux":"bum"}, "foo": {"bar": ["qux","baz"]}}]`,
		false,
		false,
	},
	{
		`{ "foo": ["bar"]}`,
		`[{"op": "copy", "path": "/foo/0", "from": "/foo"}]`,
		`{ "foo": [["bar"], "bar"]}`,
		false,
		false,
	},
	{
		`{ "foo": null}`,
		`[{"op": "copy", "path": "/bar", "from": "/foo"}]`,
		`{ "foo": null, "bar": null}`,
		false,
		false,
	},
	{
		`{ "foo": ["bar","qux","baz"]}`,
		`[ { "op": "remove", "path": "/foo/-2"}]`,
		`{ "foo": ["bar", "baz"]}`,
		false,
		false,
	},
	{
		`{ "foo": []}`,
		`[ { "op": "add", "path": "/foo/-1", "value": "qux"}]`,
		`{ "foo": ["qux"]}`,
		false,
		false,
	},
	{
		`{ "bar": [{"baz": null}]}`,
		`[ { "op": "replace", "path": "/bar/0/baz", "value": 1 } ]`,
		`{ "bar": [{"baz": 1}]}`,
		false,
		false,
	},
	{
		`{ "bar": [{"baz": 1}]}`,
		`[ { "op": "replace", "path": "/bar/0/baz", "value": null } ]`,
		`{ "bar": [{"baz": null}]}`,
		false,
		false,
	},
	{
		`{ "bar": [null]}`,
		`[ { "op": "replace", "path": "/bar/0", "value": 1 } ]`,
		`{ "bar": [1]}`,
		false,
		false,
	},
	{
		`{ "bar": [1]}`,
		`[ { "op": "replace", "path": "/bar/0", "value": null } ]`,
		`{ "bar": [null]}`,
		false,
		false,
	},
	{
		fmt.Sprintf(`{ "foo": ["A", %q] }`, repeatedA(48)),
		// The wrapping quotes around 'A's are included in the copy
		// size, so each copy operation increases the size by 50 bytes.
		`[ { "op": "copy", "path": "/foo/-", "from": "/foo/1" },
		   { "op": "copy", "path": "/foo/-", "from": "/foo/1" }]`,
		fmt.Sprintf(`{ "foo": ["A", %q, %q, %q] }`, repeatedA(48), repeatedA(48), repeatedA(48)),
		false,
		false,
	},
	{
		`[1, 2, 3]`,
		`[ { "op": "remove", "path": "/0" } ]`,
		`[2, 3]`,
		false,
		false,
	},
	{
		`{ "a": { "b": { "d": 1 } } }`,
		`[ { "op": "remove", "path": "/a/b/c" } ]`,
		`{ "a": { "b": { "d": 1 } } }`,
		true,
		false,
	},
	{
		`{ "a": { "b": { "d": 1 } } }`,
		`[ { "op": "remove", "path": "/x/y/z" } ]`,
		`{ "a": { "b": { "d": 1 } } }`,
		true,
		false,
	},
	{
		`[1, 2, 3]`,
		`[ { "op": "remove", "path": "/10" } ]`,
		`[1, 2, 3]`,
		true,
		false,
	},
	{
		`[1, 2, 3]`,
		`[ { "op": "remove", "path": "/10/x/y/z" } ]`,
		`[1, 2, 3]`,
		true,
		false,
	},
	{
		`[1, 2, 3]`,
		`[ { "op": "remove", "path": "/-10" } ]`,
		`[1, 2, 3]`,
		true,
		false,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a", "value": "hello" } ]`,
		`{"a": "hello" }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b", "value": "hello" } ]`,
		`{"a": {"b": "hello" } }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b/c", "value": "hello" } ]`,
		`{"a": {"b": {"c": "hello" } } }`,
		false,
		true,
	},
	{
		`{"a": {} }`,
		`[ { "op": "add", "path": "/a/b/c", "value": "hello" } ]`,
		`{"a": {"b": {"c": "hello" } } }`,
		false,
		true,
	},
	{
		`{"a": {} }`,
		`[ { "op": "add", "path": "/x/y/z", "value": "hello" } ]`,
		`{"a": {}, "x" : {"y": {"z": "hello" } } }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/0/b", "value": "hello" } ]`,
		`{"a": [{"b": "hello"}] }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b/0", "value": "hello" } ]`,
		`{"a": {"b": ["hello"] } }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b/-1", "value": "hello" } ]`,
		`{"a": {"b": ["hello"] } }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b/-1/c", "value": "hello" } ]`,
		`{"a": {"b": [ { "c": "hello" } ] } }`,
		false,
		true,
	},
	{
		`{"a": {"b": [ { "c": "whatever" } ] } }`,
		`[ { "op": "add", "path": "/a/b/-1/c", "value": "hello" } ]`,
		`{"a": {"b": [ { "c": "hello" } ] } }`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/b/3", "value": "hello" } ]`,
		`{"a": {"b": [null, null, null, "hello"] } }`,
		false,
		true,
	},
	{
		`{"a": []}`,
		`[ { "op": "add", "path": "/a/-1", "value": "hello" } ]`,
		`{"a": ["hello"]}`,
		false,
		true,
	},
	{
		`{}`,
		`[ { "op": "add", "path": "/a/0/0", "value": "hello" } ]`,
		`{"a": [["hello"]]}`,
		false,
		true,
	},
	{
		`{"a": [{}]}`,
		`[ { "op": "add", "path": "/a/-1/b/c", "value": "hello" } ]`,
		`{"a": [{"b": {"c": "hello"}}]}`,
		false,
		true,
	},
	{
		`{"a": [{"b": "whatever"}]}`,
		`[ { "op": "add", "path": "/a/2/b/c", "value": "hello" } ]`,
		`{"a": [{"b": "whatever"}, null, {"b": {"c": "hello"}}]}`,
		false,
		true,
	},
	{
		`{"a": [{"b": "whatever"}]}`,
		`[ { "op": "add", "path": "/a/1/b/c", "value": "hello" } ]`,
		`{"a": [{"b": "whatever"}, {"b": {"c": "hello"}}]}`,
		false,
		true,
	},
	{
		`{
	"id": "00000000-0000-0000-0000-000000000000",
	"parentID": "00000000-0000-0000-0000-000000000000"
}`,
		`[
  {
    "op": "test",
    "path": "",
    "value": {
      "id": "00000000-0000-0000-0000-000000000000",
      "parentID": "00000000-0000-0000-0000-000000000000"
    }
  },
  {
    "op": "replace",
    "path": "",
    "value": {
      "id": "759981e8-ec68-4639-a83e-513225914ecb",
      "originalID": "bar",
      "parentID": "00000000-0000-0000-0000-000000000000"
    }
  }
]`,
		`{
  "id" : "759981e8-ec68-4639-a83e-513225914ecb",
  "originalID" : "bar",
  "parentID" : "00000000-0000-0000-0000-000000000000"
}`,
		false, true,
	},
	{
		`{"baz": []}`,
		`[ { "op": "test", "path": "/foo" } ]`,
		`{"baz": []}`,
		false,
		false,
	},
	{
		`{}`,
		`[{"op": "replace", "path": "", "value": null}]`,
		`null`,
		false,
		false,
	},
	{
		`{"foo": 1}`,
		`[ { "op": "copy", "from": "", "path": "/bar"}]`,
		`{"foo": 1, "bar": {"foo": 1}}`,
		false,
		false,
	},
	{
		`[{"foo": 1}]`,
		`[ { "op": "copy", "from": "", "path": "/1"}]`,
		`[{"foo": 1}, [{"foo": 1}]]`,
		false,
		false,
	},
	{
		`{}`,
		`[{"op":"add","path":"","value":{"foo":"bar"}}]`,
		`{"foo": "bar"}`,
		false,
		false,
	},
	{
		`[]`,
		`[{"op":"add","path":"","value":{"foo":"bar"}}, {"op": "add", "path": "/qux", "value": 1}]`,
		`{"foo": "bar", "qux": 1}`,
		false,
		false,
	},
}

type BadCase struct {
	doc, patch   string
	failOnDecode bool
}

var MutationTestCases = []BadCase{
	{
		`{ "foo": "bar", "qux": { "baz": 1, "bar": null } }`,
		`[ { "op": "remove", "path": "/qux/bar" } ]`,
		false,
	},
	{
		`{ "foo": "bar", "qux": { "baz": 1, "bar": null } }`,
		`[ { "op": "replace", "path": "/qux/baz", "value": null } ]`,
		true,
	},
	// malformed value
	{
		`{ "foo": "bar" }`,
		`[ { "op": "add", "path": "/", "value": "{qux" } ]`,
		true,
	},
}

var BadCases = []BadCase{
	{
		`{ "foo": "bar" }`,
		`[ { "op": "add", "path": "/baz/bat", "value": "qux" } ]`,
		false,
	},
	{
		`{ "a": { "b": { "d": 1 } } }`,
		`[ { "op": "remove", "path": "/a/b/c" } ]`,
		false,
	},
	{
		`{ "a": { "b": { "d": 1 } } }`,
		`[ { "op": "move", "from": "/a/b/c", "path": "/a/b/e" } ]`,
		false,
	},
	{
		`{ "a": { "b": [1] } }`,
		`[ { "op": "remove", "path": "/a/b/1" } ]`,
		false,
	},
	{
		`{ "a": { "b": [1] } }`,
		`[ { "op": "move", "from": "/a/b/1", "path": "/a/b/2" } ]`,
		false,
	},
	{
		`{ "foo": "bar" }`,
		`[ { "op": "add", "pathz": "/baz", "value": "qux" } ]`,
		true,
	},
	{
		`{ "foo": ["bar","baz"]}`,
		`[ { "op": "replace", "path": "/foo/2", "value": "bum"}]`,
		false,
	},
	{
		`{ "foo": ["bar","baz"]}`,
		`[ { "op": "add", "path": "/foo/-4", "value": "bum"}]`,
		false,
	},
	{
		`{ "name":{ "foo": "bat", "qux": "bum"}}`,
		`[ { "op": "replace", "path": "/foo/bar", "value":"baz"}]`,
		false,
	},
	{
		`{ "foo": ["bar"]}`,
		`[ {"op": "add", "path": "/foo/2", "value": "bum"}]`,
		false,
	},
	{
		`{ "foo": []}`,
		`[ {"op": "remove", "path": "/foo/-"}]`,
		false,
	},
	{
		`{ "foo": []}`,
		`[ {"op": "remove", "path": "/foo/-1"}]`,
		false,
	},
	{
		`{ "foo": ["bar"]}`,
		`[ {"op": "remove", "path": "/foo/-2"}]`,
		false,
	},
	{
		`{}`,
		`[ {"op":null,"path":""} ]`,
		true,
	},
	{
		`{}`,
		`[ {"op":"add","path":null} ]`,
		true,
	},
	{
		`{}`,
		`[ { "op": "copy", "from": null }]`,
		true,
	},
	{
		`{ "foo": ["bar"]}`,
		`[{"op": "copy", "path": "/foo/6666666666", "from": "/"}]`,
		false,
	},
	// Can't copy into an index greater than the size of the array
	{
		`{ "foo": ["bar"]}`,
		`[{"op": "copy", "path": "/foo/2", "from": "/foo/0"}]`,
		false,
	},
	// Accumulated copy size cannot exceed AccumulatedCopySizeLimit.
	{
		fmt.Sprintf(`{ "foo": ["A", %q] }`, repeatedA(49)),
		// The wrapping quotes around 'A's are included in the copy
		// size, so each copy operation increases the size by 51 bytes.
		`[ { "op": "copy", "path": "/foo/-", "from": "/foo/1" },
		   { "op": "copy", "path": "/foo/-", "from": "/foo/1" }]`,
		false,
	},
	// Can't move into an index greater than or equal to the size of the array
	{
		`{ "foo": [ "all", "grass", "cows", "eat" ] }`,
		`[ { "op": "move", "from": "/foo/1", "path": "/foo/4" } ]`,
		false,
	},
	{
		`{ "baz": "qux" }`,
		`[ { "op": "replace", "path": "/foo", "value": "bar" } ]`,
		false,
	},
	// Can't copy from non-existent "from" key.
	{
		`{ "foo": "bar"}`,
		`[{"op": "copy", "path": "/qux", "from": "/baz"}]`,
		false,
	},
	{
		`{ "foo": {"bar": []}}`,
		`[{"op": "replace", "path": "/foo/bar", "value": null}, {"op": "add", "path": "/foo/bar/0", "value": "blah"}]`,
		false,
	},
	{
		`{}`,
		`[{"op": "replace", "path": ""}]`,
		true,
	},
	{
		`{ "foo": "bar"}`,
		`[{"op": "move", "path": "/qux", "from": ""}]`,
		false,
	},
}

// This is not thread safe, so we cannot run patch tests in parallel.
func configureGlobals(accumulatedCopySizeLimit int64) func() {
	oldAccumulatedCopySizeLimit := AccumulatedCopySizeLimit
	AccumulatedCopySizeLimit = accumulatedCopySizeLimit
	return func() {
		AccumulatedCopySizeLimit = oldAccumulatedCopySizeLimit
	}
}

func TestAllCases(t *testing.T) {
	defer configureGlobals(int64(100))()

	// Test patch.Apply happy-path cases.
	for i, c := range Cases {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			if !c.allowMissingPathOnRemove && !c.ensurePathExistsOnAdd {
				out, err := applyPatch(c.doc, c.patch)

				if err != nil {
					t.Errorf("Unable to apply patch: %s", err)
				}

				if !compareJSON(out, c.result) {
					t.Errorf("Patch did not apply. Expected:\n%s\n\nActual:\n%s",
						reformatJSON(c.result), reformatJSON(out))
				}
			}
		})
	}

	// Test patch.ApplyWithOptions happy-path cases.
	options := NewApplyOptions()

	for _, c := range Cases {
		options.AllowMissingPathOnRemove = c.allowMissingPathOnRemove
		options.EnsurePathExistsOnAdd = c.ensurePathExistsOnAdd

		out, err := applyPatchWithOptions(c.doc, c.patch, options)

		if err != nil {
			t.Errorf("Unable to apply patch: %s", err)
		}

		if !compareJSON(out, c.result) {
			t.Errorf("Patch did not apply. Expected:\n%s\n\nActual:\n%s",
				reformatJSON(c.result), reformatJSON(out))
		}
	}

	for _, c := range MutationTestCases {
		out, err := applyPatch(c.doc, c.patch)

		if err != nil {
			t.Errorf("Unable to apply patch: %s", err)
		}

		if compareJSON(out, c.doc) {
			t.Errorf("Patch did not apply. Original:\n%s\n\nPatched:\n%s",
				reformatJSON(c.doc), reformatJSON(out))
		}
	}

	for _, c := range BadCases {
		p, err := DecodePatch([]byte(c.patch))
		if err == nil && c.failOnDecode {
			t.Errorf("Patch %q should have failed decode but did not", c.patch)
		}

		if err != nil && !c.failOnDecode {
			t.Errorf("Patch %q should have passed decode but failed with %v", c.patch, err)
		}

		if err == nil && !c.failOnDecode {
			out, err := p.Apply([]byte(c.doc))

			if err == nil {
				t.Log(string(out))
				t.Errorf("Patch %q should have failed to apply but it did not", c.patch)
			}

		}
	}
}

type TestCase struct {
	doc, patch string
	result     bool
	failedPath string
}

var TestCases = []TestCase{
	{
		`{
      "baz": "qux",
      "foo": [ "a", 2, "c" ]
    }`,
		`[
      { "op": "test", "path": "/baz", "value": "qux" },
      { "op": "test", "path": "/foo/1", "value": 2 }
    ]`,
		true,
		"",
	},
	{
		`{ "baz": "qux" }`,
		`[ { "op": "test", "path": "/baz", "value": "bar" } ]`,
		false,
		"/baz",
	},
	{
		`{
      "baz": "qux",
      "foo": ["a", 2, "c"]
    }`,
		`[
      { "op": "test", "path": "/baz", "value": "qux" },
      { "op": "test", "path": "/foo/1", "value": "c" }
    ]`,
		false,
		"/foo/1",
	},
	{
		`{ "baz": "qux" }`,
		`[ { "op": "test", "path": "/foo", "value": 42 } ]`,
		false,
		"/foo",
	},
	{
		`{ "baz": "qux" }`,
		`[ { "op": "test", "path": "/foo", "value": null } ]`,
		true,
		"",
	},
	{
		`{ "foo": null }`,
		`[ { "op": "test", "path": "/foo", "value": null } ]`,
		true,
		"",
	},
	{
		`{ "foo": {} }`,
		`[ { "op": "test", "path": "/foo", "value": null } ]`,
		false,
		"/foo",
	},
	{
		`{ "foo": [] }`,
		`[ { "op": "test", "path": "/foo", "value": null } ]`,
		false,
		"/foo",
	},
	{
		`{ "baz/foo": "qux" }`,
		`[ { "op": "test", "path": "/baz~1foo", "value": "qux"} ]`,
		true,
		"",
	},
	{
		`{ "foo": [] }`,
		`[ { "op": "test", "path": "/foo"} ]`,
		false,
		"/foo",
	},
	{
		`{ "foo": "bar" }`,
		`[ { "op": "test", "path": "/baz", "value": "bar" } ]`,
		false,
		"/baz",
	},
	{
		`{ "foo": "bar" }`,
		`[ { "op": "test", "path": "/baz", "value": null } ]`,
		true,
		"/baz",
	},
}

func TestAllTest(t *testing.T) {
	for _, c := range TestCases {
		_, err := applyPatch(c.doc, c.patch)

		if c.result && err != nil {
			t.Errorf("Testing failed when it should have passed: %s", err)
		} else if !c.result && err == nil {
			t.Errorf("Testing passed when it should have failed: %s", err)
		} else if !c.result {
			expected := fmt.Sprintf("testing value %s failed: test failed", c.failedPath)
			if err.Error() != expected {
				t.Errorf("Testing failed as expected but invalid message: expected [%s], got [%s]", expected, err)
			}
		}
	}
}

func TestAdd(t *testing.T) {
	testCases := []struct {
		name                   string
		key                    string
		val                    lazyNode
		arr                    partialArray
		rejectNegativeIndicies bool
		err                    string
	}{
		{
			name: "should work",
			key:  "0",
			val:  lazyNode{},
			arr:  partialArray{},
		},
		{
			name: "index too large",
			key:  "1",
			val:  lazyNode{},
			arr:  partialArray{},
			err:  "Unable to access invalid index: 1: invalid index referenced",
		},
		{
			name: "negative should work",
			key:  "-1",
			val:  lazyNode{},
			arr:  partialArray{},
		},
		{
			name: "negative too small",
			key:  "-2",
			val:  lazyNode{},
			arr:  partialArray{},
			err:  "Unable to access invalid index: -2: invalid index referenced",
		},
		{
			name:                   "negative but negative disabled",
			key:                    "-1",
			val:                    lazyNode{},
			arr:                    partialArray{},
			rejectNegativeIndicies: true,
			err:                    "Unable to access invalid index: -1: invalid index referenced",
		},
	}

	options := NewApplyOptions()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			arr := &tc.arr
			val := &tc.val
			options.SupportNegativeIndices = !tc.rejectNegativeIndicies
			err := arr.add(key, val, options)
			if err == nil && tc.err != "" {
				t.Errorf("Expected error but got none! %v", tc.err)
			} else if err != nil && tc.err == "" {
				t.Errorf("Did not expect error but go: %v", err)
			} else if err != nil && err.Error() != tc.err {
				t.Errorf("Expected error %v but got error %v", tc.err, err)
			}
		})
	}
}

type EqualityCase struct {
	name  string
	a, b  string
	equal bool
}

var EqualityCases = []EqualityCase{
	{
		"ExtraKeyFalse",
		`{"foo": "bar"}`,
		`{"foo": "bar", "baz": "qux"}`,
		false,
	},
	{
		"StripWhitespaceTrue",
		`{
			"foo": "bar",
			"baz": "qux"
		}`,
		`{"foo": "bar", "baz": "qux"}`,
		true,
	},
	{
		"KeysOutOfOrderTrue",
		`{
			"baz": "qux",
			"foo": "bar"
		}`,
		`{"foo": "bar", "baz": "qux"}`,
		true,
	},
	{
		"ComparingNullFalse",
		`{"foo": null}`,
		`{"foo": "bar"}`,
		false,
	},
	{
		"ComparingNullTrue",
		`{"foo": null}`,
		`{"foo": null}`,
		true,
	},
	{
		"ArrayOutOfOrderFalse",
		`["foo", "bar", "baz"]`,
		`["bar", "baz", "foo"]`,
		false,
	},
	{
		"ArrayTrue",
		`["foo", "bar", "baz"]`,
		`["foo", "bar", "baz"]`,
		true,
	},
	{
		"NonStringTypesTrue",
		`{"int": 6, "bool": true, "float": 7.0, "string": "the_string", "null": null}`,
		`{"int": 6, "bool": true, "float": 7.0, "string": "the_string", "null": null}`,
		true,
	},
	{
		"NestedNullFalse",
		`{"foo": ["an", "array"], "bar": {"an": "object"}}`,
		`{"foo": null, "bar": null}`,
		false,
	},
	{
		"NullCompareStringFalse",
		`"foo"`,
		`null`,
		false,
	},
	{
		"NullCompareIntFalse",
		`6`,
		`null`,
		false,
	},
	{
		"NullCompareFloatFalse",
		`6.01`,
		`null`,
		false,
	},
	{
		"NullCompareBoolFalse",
		`false`,
		`null`,
		false,
	},
	{
		"Unicode",
		`{"name": "λJohn"}`,
		`{"name": "\u03BBJohn"}`,
		true,
	},
}

func TestEquality(t *testing.T) {
	for _, tc := range EqualityCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Equal([]byte(tc.a), []byte(tc.b))
			if got != tc.equal {
				t.Errorf("Expected Equal(%s, %s) to return %t, but got %t", tc.a, tc.b, tc.equal, got)
			}

			got = Equal([]byte(tc.b), []byte(tc.a))
			if got != tc.equal {
				t.Errorf("Expected Equal(%s, %s) to return %t, but got %t", tc.b, tc.a, tc.equal, got)
			}
		})
	}
}

func TestMaintainOrdering(t *testing.T) {
	cases := []struct {
		doc      string
		patch    string
		expected string
	}{
		{
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null}`,
			`[{"op": "add", "path": "/foo", "value": "bar"}]`,
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null,"foo":"bar"}`,
		},
		{
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null}`,
			`[{"op": "remove", "path": "/y"}]`,
			`{"z":"1","a":["baz"],"b":true,"x":null}`,
		},
		{
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null}`,
			`[{"op": "move", "from": "/z", "path": "/a/-"},{"op": "remove", "path": "/y"}]`,
			`{"a":["baz","1"],"b":true,"x":null}`,
		},
		{
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null}`,
			`[
               {"op": "add", "path": "/foo", "value": "bar"},
               {"op": "replace", "path": "/b", "value": {"zz":1,"aa":"foo","yy":true,"bb":null}},
               {"op": "copy", "from": "/foo", "path": "/b/cc"},
               {"op": "move", "from": "/z", "path": "/a/0"},
               {"op": "remove", "path": "/y"}
             ]`,
			`{"a":["1","baz"],"b":{"zz":1,"aa":"foo","yy":true,"bb":null,"cc":"bar"},"x":null,"foo":"bar"}`,
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			res, err := applyPatch(c.doc, c.patch)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if res != c.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", c.expected, res)
			}
		})
	}
}

func TestMaintainOrderingIndented(t *testing.T) {
	cases := []struct {
		doc      string
		patch    string
		expected string
	}{
		{
			`{"z":"1","a":["baz"],"y":3,"b":true,"x":null}`,
			`[
               {"op": "add", "path": "/foo", "value": "bar"},
               {"op": "replace", "path": "/b", "value": {"zz":1,"aa":"foo","yy":true,"bb":null}},
               {"op": "copy", "from": "/foo", "path": "/b/cc"},
               {"op": "move", "from": "/z", "path": "/a/0"},
               {"op": "remove", "path": "/y"}
             ]`,
			`{
  "a": [
    "1",
    "baz"
  ],
  "b": {
    "zz": 1,
    "aa": "foo",
    "yy": true,
    "bb": null,
    "cc": "bar"
  },
  "x": null,
  "foo": "bar"
}`,
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			res, err := applyPatchIndented(c.doc, c.patch)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if res != c.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", c.expected, res)
			}
		})
	}
}

// This is a compile time check that encoding/json's RawMessage can be used in Operation
func init() {
	msg := json.RawMessage([]byte(`1`))

	_ = Operation{
		"foo": &msg,
	}
}
