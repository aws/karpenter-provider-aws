package tests

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRequiredField(t *testing.T) {
	cases := []struct{ json, errorMessage string }{
		{`{"first_name":"Foo", "last_name": "Bar"}`, ""},
		{`{"last_name":"Bar"}`, "key 'first_name' is required"},
		{"{}", "key 'first_name' is required"},
	}

	for _, tc := range cases {
		var v RequiredOptionalStruct
		err := v.UnmarshalJSON([]byte(tc.json))
		if tc.errorMessage == "" {
			if err != nil {
				t.Errorf("%s. UnmarshalJSON didn`t expect error: %v", tc.json, err)
			}
		} else {
			if fmt.Sprintf("%v", err) != tc.errorMessage {
				t.Errorf("%s. UnmarshalJSON expected error: %v. got: %v", tc.json, tc.errorMessage, err)
			}
		}
	}
}

func TestRequiredOptionalMap(t *testing.T) {
	baseJson := `{"req_map":{}, "oe_map":{}, "noe_map":{}, "oe_slice":[]}`
	wantDecoding := RequiredOptionalMap{MapIntString{}, nil, MapIntString{}}

	var v RequiredOptionalMap
	if err := v.UnmarshalJSON([]byte(baseJson)); err != nil {
		t.Errorf("%s. UnmarshalJSON didn't expect error: %v", baseJson, err)
	}
	if !reflect.DeepEqual(v, wantDecoding) {
		t.Errorf("%s. UnmarshalJSON expected to gen: %v. got: %v", baseJson, wantDecoding, v)
	}

	baseStruct := RequiredOptionalMap{MapIntString{}, MapIntString{}, MapIntString{}}
	wantJson := `{"req_map":{},"noe_map":{}}`
	data, err := baseStruct.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON didn't expect error: %v on %v", err, data)
	} else if string(data) != wantJson {
		t.Errorf("%v. MarshalJSON wanted: %s got %s", baseStruct, wantJson, string(data))
	}
}
