package tests

import (
	"reflect"
	"testing"
)

func TestUnknownFieldsProxy(t *testing.T) {
	baseJson := `{"Field1":"123","Field2":"321"}`

	s := StructWithUnknownsProxy{}

	err := s.UnmarshalJSON([]byte(baseJson))
	if err != nil {
		t.Errorf("UnmarshalJSON didn't expect error: %v", err)
	}

	if s.Field1 != "123" {
		t.Errorf("UnmarshalJSON expected to parse Field1 as \"123\". got: %v", s.Field1)
	}

	data, err := s.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON didn't expect error: %v", err)
	}

	if !reflect.DeepEqual(baseJson, string(data)) {
		t.Errorf("MarshalJSON expected to gen: %v. got: %v", baseJson, string(data))
	}
}

func TestUnknownFieldsProxyWithOmitempty(t *testing.T) {
	baseJson := `{"Field1":"123","Field2":"321"}`

	s := StructWithUnknownsProxyWithOmitempty{}

	err := s.UnmarshalJSON([]byte(baseJson))
	if err != nil {
		t.Errorf("UnmarshalJSON didn't expect error: %v", err)
	}

	if s.Field1 != "123" {
		t.Errorf("UnmarshalJSON expected to parse Field1 as \"123\". got: %v", s.Field1)
	}

	data, err := s.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON didn't expect error: %v", err)
	}

	if !reflect.DeepEqual(baseJson, string(data)) {
		t.Errorf("MarshalJSON expected to gen: %v. got: %v", baseJson, string(data))
	}
}
