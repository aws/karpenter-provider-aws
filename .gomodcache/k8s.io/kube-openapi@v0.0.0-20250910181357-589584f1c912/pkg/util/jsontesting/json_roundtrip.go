/*
Copyright 2022 The Kubernetes Authors.

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

package jsontesting

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	kjson "sigs.k8s.io/json"

	"github.com/go-openapi/jsonreference"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func JsonCompare(got, want []byte) error {
	if d := cmp.Diff(got, want, cmp.Transformer("JSONBytes", func(in []byte) (out interface{}) {
		if strictErrors, err := kjson.UnmarshalStrict(in, &out); strictErrors != nil || err != nil {
			return in
		}
		return out
	})); d != "" {
		return fmt.Errorf("JSON mismatch (-got +want):\n%s", d)
	}
	return nil
}

type RoundTripTestCase struct {
	Name   string
	JSON   string
	Object json.Marshaler

	// An error that is expected when `Object` is marshalled to json
	// If `Object` does not exist, then it is inferred from the provided JSON
	ExpectedMarshalError string

	// An error that is expected when the provided JSON is unmarshalled
	// If `JSON` does not exist then this it is inferred from the provided `Object`
	ExpectedUnmarshalError string
}

type MarshalerUnmarshaler interface {
	json.Unmarshaler
	json.Marshaler
}

func (t RoundTripTestCase) RoundTripTest(example MarshalerUnmarshaler) error {
	var jsonBytes []byte
	var err error

	// Tests whether the provided error matches the given pattern, and says
	// whether the test is finished, and the error to return
	expectError := func(e error, name string, expected string) (testFinished bool, err error) {
		if len(expected) > 0 {
			if e == nil || !strings.Contains(e.Error(), expected) {
				return true, fmt.Errorf("expected %v error containing substring: '%s'. but got actual error '%v'", name, expected, e)
			}

			// If an error was expected and achieved, we stop the test
			// since it cannot be continued. But the return nil error since it
			// was expected.
			return true, nil
		} else if e != nil {
			return true, fmt.Errorf("unexpected %v error: %w", name, e)
		}

		return false, nil
	}

	// If user did not provide JSON and instead provided Object, infer JSON
	// from the provided object.
	if len(t.JSON) == 0 {
		jsonBytes, err = json.Marshal(t.Object)
		if testFinished, err := expectError(err, "marshal", t.ExpectedMarshalError); testFinished {
			return err
		}
	} else {
		jsonBytes = []byte(t.JSON)
	}

	err = example.UnmarshalJSON(jsonBytes)
	if testFinished, err := expectError(err, "unmarshal", t.ExpectedUnmarshalError); testFinished {
		return err
	}

	if t.Object != nil && !reflect.DeepEqual(t.Object, example) {
		return fmt.Errorf("test case expected to unmarshal to specific value: %v", cmp.Diff(t.Object, example, cmpopts.IgnoreUnexported(jsonreference.Ref{})))
	}

	reEncoded, err := json.Marshal(example)
	if err != nil {
		return fmt.Errorf("failed to marshal decoded value: %w", err)
	}

	// Check expected marshal error if it has not yet been checked
	// (for case where JSON is provided, and object is not)
	if testFinished, err := expectError(err, "marshal", t.ExpectedMarshalError); testFinished {
		return err
	}
	// Marshal both re-encoded, and original JSON into interface
	// to compare them without ordering issues
	var expected map[string]interface{}
	var actual map[string]interface{}

	if err = json.Unmarshal(jsonBytes, &expected); err != nil {
		return fmt.Errorf("failed to unmarshal test json: %w", err)
	}

	if err = json.Unmarshal(reEncoded, &actual); err != nil {
		return fmt.Errorf("failed to unmarshal actual data: %w", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf("expected equal values: %v", cmp.Diff(expected, actual))
	}

	return nil
}
