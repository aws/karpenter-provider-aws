/*
Copyright 2015 The Kubernetes Authors.

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

package env

import (
	"os"
	"strconv"
	"testing"
)

func TestGetString(t *testing.T) {
	const expected = "foo"

	key := "STRING_SET_VAR"
	os.Setenv(key, expected)
	if e, a := expected, GetString(key, "~"+expected); e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "STRING_UNSET_VAR"
	if e, a := expected, GetString(key, expected); e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}
}

func TestGetInt(t *testing.T) {
	const expected = 1
	const defaultValue = 2

	key := "INT_SET_VAR"
	os.Setenv(key, strconv.Itoa(expected))
	returnVal, _ := GetInt(key, defaultValue)
	if e, a := expected, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "INT_UNSET_VAR"
	returnVal, _ = GetInt(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "INT_SET_VAR"
	os.Setenv(key, "not-an-int")
	returnVal, err := GetInt(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetFloat64(t *testing.T) {
	const expected = 1.0
	const defaultValue = 2.0

	key := "FLOAT_SET_VAR"
	os.Setenv(key, "1.0")
	returnVal, _ := GetFloat64(key, defaultValue)
	if e, a := expected, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "FLOAT_UNSET_VAR"
	returnVal, _ = GetFloat64(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "FLOAT_SET_VAR"
	os.Setenv(key, "not-a-float")
	returnVal, err := GetFloat64(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}
	if err == nil || err.Error() != "strconv.ParseFloat: parsing \"not-a-float\": invalid syntax" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestGetBool(t *testing.T) {
	const expected = true
	const defaultValue = false

	key := "BOOL_SET_VAR"
	os.Setenv(key, "true")
	returnVal, _ := GetBool(key, defaultValue)
	if e, a := expected, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "BOOL_UNSET_VAR"
	returnVal, _ = GetBool(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}

	key = "BOOL_SET_VAR"
	os.Setenv(key, "not-a-bool")
	returnVal, err := GetBool(key, defaultValue)
	if e, a := defaultValue, returnVal; e != a {
		t.Fatalf("expected %#v==%#v", e, a)
	}
	if err == nil || err.Error() != "strconv.ParseBool: parsing \"not-a-bool\": invalid syntax" {
		t.Fatalf("unexpected error: %#v", err)
	}
}
