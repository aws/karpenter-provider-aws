/*
Copyright 2018 The Kubernetes Authors.

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

package merge_test

import (
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/merge"
	"sigs.k8s.io/structured-merge-diff/v6/value"
)

var (
	// Short names for readable test cases.
	_NS  = fieldpath.NewSet
	_P   = fieldpath.MakePathOrDie
	_KBF = fieldpath.KeyByFields
	_V   = value.NewValueInterface
)

func TestNewFromSets(t *testing.T) {
	got := merge.ConflictsFromManagers(fieldpath.ManagedFields{
		"Bob": fieldpath.NewVersionedSet(
			_NS(
				_P("key"),
				_P("list", _KBF("key", "a", "id", 2), "id"),
			),
			"v1",
			false,
		),
		"Alice": fieldpath.NewVersionedSet(
			_NS(
				_P("value"),
				_P("list", _KBF("key", "a", "id", 2), "key"),
			),
			"v1",
			false,
		),
	})
	wanted := `conflicts with "Alice":
- .value
- .list[id=2,key="a"].key
conflicts with "Bob":
- .key
- .list[id=2,key="a"].id`
	if got.Error() != wanted {
		t.Errorf("Got %v, wanted %v", got.Error(), wanted)
	}
}

func TestToSet(t *testing.T) {
	conflicts := merge.ConflictsFromManagers(fieldpath.ManagedFields{
		"Bob": fieldpath.NewVersionedSet(
			_NS(
				_P("key"),
				_P("list", _KBF("key", "a", "id", 2), "id"),
			),
			"v1",
			false,
		),
		"Alice": fieldpath.NewVersionedSet(
			_NS(
				_P("value"),
				_P("list", _KBF("key", "a", "id", 2), "key"),
			),
			"v1",
			false,
		),
	})
	expected := fieldpath.NewSet(
		_P("key"),
		_P("value"),
		_P("list", _KBF("key", "a", "id", 2), "id"),
		_P("list", _KBF("key", "a", "id", 2), "key"),
	)
	actual := conflicts.ToSet()
	if !expected.Equals(actual) {
		t.Fatalf("expected\n%v\n, but got\n%v\n", expected, actual)
	}
}

func TestConflictsFromManagers(t *testing.T) {
	got := merge.ConflictsFromManagers(fieldpath.ManagedFields{
		"Bob": fieldpath.NewVersionedSet(
			_NS(
				_P("obj", "template", "obj", "list", _KBF("name", "a"), "id"),
				_P("obj", "template", "obj", "list", _KBF("name", "a"), "key"),
			),
			"v1",
			false,
		),
	})
	wanted := `conflicts with "Bob":
- .obj.template.obj.list[name="a"].id
- .obj.template.obj.list[name="a"].key`
	if got.Error() != wanted {
		t.Errorf("Got %v, wanted %v", got.Error(), wanted)
	}
}
