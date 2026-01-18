package json_test

import (
	"bytes"
	"testing"

	"github.com/aws/smithy-go/encoding/json"
)

func TestEncoder(t *testing.T) {
	encoder := json.NewEncoder()

	object := encoder.Object()

	object.Key("stringKey").String("stringValue")
	object.Key("integerKey").Long(1024)
	object.Key("floatKey").Double(3.14)

	subObj := object.Key("foo").Object()

	subObj.Key("byteSlice").Base64EncodeBytes([]byte("foo bar"))
	subObj.Close()

	object.Close()

	e := []byte(`{"stringKey":"stringValue","integerKey":1024,"floatKey":3.14,"foo":{"byteSlice":"Zm9vIGJhcg=="}}`)
	if a := encoder.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}

	if a := encoder.String(); string(e) != a {
		t.Errorf("expected %s, but got %s", e, a)
	}
}
