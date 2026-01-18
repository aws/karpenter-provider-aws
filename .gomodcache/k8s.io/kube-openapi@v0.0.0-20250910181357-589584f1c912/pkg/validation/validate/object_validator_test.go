// Copyright 2017 go-swagger maintainers
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

package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	kubeopenapierrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
)

func itemsFixture() map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": "dummy",
	}
}

func expectAllValid(t *testing.T, ov ValueValidator, dataValid, dataInvalid map[string]interface{}) {
	res := ov.Validate(dataValid)
	assert.Equal(t, 0, len(res.Errors))

	res = ov.Validate(dataInvalid)
	assert.Equal(t, 0, len(res.Errors))
}

func expectOnlyInvalid(t *testing.T, ov ValueValidator, dataValid, dataInvalid map[string]interface{}) {
	res := ov.Validate(dataValid)
	assert.Equal(t, 0, len(res.Errors))

	res = ov.Validate(dataInvalid)
	assert.NotEqual(t, 0, len(res.Errors))
}

func TestItemsMustBeTypeArray(t *testing.T) {
	ov := new(objectValidator)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"type":  "object",
		"items": "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)
}

func TestItemsMustHaveType(t *testing.T) {
	ov := new(objectValidator)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"items": "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)
}

func TestTypeArrayMustHaveItems(t *testing.T) {
	ov := new(objectValidator)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"type": "array",
		"key":  "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)
}

// Test edge cases in object_validator which are difficult
// to simulate with specs
// (this one is a trivial, just to check all methods are filled)
func TestObjectValidator_EdgeCases(t *testing.T) {
	s := objectValidator{}
	s.SetPath("path")
	assert.Equal(t, "path", s.Path)
}

func TestMinPropertiesMaxPropertiesDontShortCircuit(t *testing.T) {
	s := objectValidator{
		In:            "body",
		Path:          "some.path[5]",
		KnownFormats:  strfmt.Default,
		MinProperties: ptr(int64(20)),
		MaxProperties: ptr(int64(0)),
		Properties: map[string]spec.Schema{
			"intField": {
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"integer"},
				},
			},
			"requiredField": {
				SchemaProps: spec.SchemaProps{
					Type: spec.StringOrArray{"string"},
				},
			},
		},
		Required: []string{"requiredField"},
		AdditionalProperties: &spec.SchemaOrBool{
			Allows: true,
			Schema: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Type:    spec.StringOrArray{"string"},
					Pattern: "^20[0-9][0-9]",
				},
			},
		},
		Options: SchemaValidatorOptions{
			NewValidatorForIndex: func(index int, schema *spec.Schema, rootSchema interface{}, root string, formats strfmt.Registry, opts ...Option) ValueValidator {
				return NewSchemaValidator(schema, rootSchema, root, formats, opts...)
			},
			NewValidatorForField: func(field string, schema *spec.Schema, rootSchema interface{}, root string, formats strfmt.Registry, opts ...Option) ValueValidator {
				return NewSchemaValidator(schema, rootSchema, root, formats, opts...)
			},
		},
	}

	obj := map[string]interface{}{
		"field": "hello, world",
	}
	res := s.Validate(obj)

	assert.ElementsMatch(t, []*kubeopenapierrors.Validation{
		kubeopenapierrors.TooFewProperties(s.Path, s.In, *s.MinProperties, int64(len(obj))),
		kubeopenapierrors.TooManyProperties(s.Path, s.In, *s.MaxProperties, int64(len(obj))),
		kubeopenapierrors.FailedPattern(s.Path+"."+"field", s.In, s.AdditionalProperties.Schema.Pattern, "hello, world"),
		kubeopenapierrors.Required(s.Path+"."+"requiredField", s.In),
	}, res.Errors)

	obj = map[string]interface{}{
		"field":    "hello, world",
		"field2":   "hello, other world",
		"field3":   "hello, third world",
		"intField": "a string",
	}
	res = s.Validate(obj)

	assert.ElementsMatch(t, []*kubeopenapierrors.Validation{
		kubeopenapierrors.TooFewProperties(s.Path, s.In, *s.MinProperties, int64(len(obj))),
		kubeopenapierrors.TooManyProperties(s.Path, s.In, *s.MaxProperties, int64(len(obj))),
		kubeopenapierrors.FailedPattern(s.Path+"."+"field", s.In, s.AdditionalProperties.Schema.Pattern, "hello, world"),
		kubeopenapierrors.FailedPattern(s.Path+"."+"field2", s.In, s.AdditionalProperties.Schema.Pattern, "hello, other world"),
		kubeopenapierrors.FailedPattern(s.Path+"."+"field3", s.In, s.AdditionalProperties.Schema.Pattern, "hello, third world"),
		kubeopenapierrors.InvalidType(s.Path+"."+"intField", s.In, "integer", "string"),
		kubeopenapierrors.Required(s.Path+"."+"requiredField", s.In),
	}, res.Errors)
}

func ptr[T any](v T) *T {
	return &v
}
