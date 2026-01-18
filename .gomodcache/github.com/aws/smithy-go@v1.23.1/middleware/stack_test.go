package middleware

import (
	"reflect"
	"strings"
	"testing"
)

func TestStackList(t *testing.T) {
	s := NewStack("fooStack", func() interface{} { return struct{}{} })

	s.Initialize.Add(mockInitializeMiddleware("first"), After)
	s.Serialize.Add(mockSerializeMiddleware("second"), After)
	s.Build.Add(mockBuildMiddleware("third"), After)
	s.Finalize.Add(mockFinalizeMiddleware("fourth"), After)
	s.Deserialize.Add(mockDeserializeMiddleware("fifth"), After)

	actual := s.List()

	expect := []string{
		"fooStack",
		(*InitializeStep)(nil).ID(),
		"first",
		(*SerializeStep)(nil).ID(),
		"second",
		(*BuildStep)(nil).ID(),
		"third",
		(*FinalizeStep)(nil).ID(),
		"fourth",
		(*DeserializeStep)(nil).ID(),
		"fifth",
	}

	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("expect and actual stack list differ: %v != %v", expect, actual)
	}
}

func TestStackString(t *testing.T) {
	s := NewStack("fooStack", func() interface{} { return struct{}{} })

	s.Initialize.Add(mockInitializeMiddleware("first"), After)
	s.Serialize.Add(mockSerializeMiddleware("second"), After)
	s.Build.Add(mockBuildMiddleware("third"), After)
	s.Finalize.Add(mockFinalizeMiddleware("fourth"), After)
	s.Deserialize.Add(mockDeserializeMiddleware("fifth"), After)

	actual := s.String()

	expect := strings.Join([]string{
		"fooStack",
		"\t" + (*InitializeStep)(nil).ID(),
		"\t\t" + "first",
		"\t" + (*SerializeStep)(nil).ID(),
		"\t\t" + "second",
		"\t" + (*BuildStep)(nil).ID(),
		"\t\t" + "third",
		"\t" + (*FinalizeStep)(nil).ID(),
		"\t\t" + "fourth",
		"\t" + (*DeserializeStep)(nil).ID(),
		"\t\t" + "fifth",
		"",
	}, "\n")

	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("expect and actual stack list differ: %v != %v", expect, actual)
	}
}
