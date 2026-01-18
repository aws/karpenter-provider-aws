package auth

import (
	"testing"
	"reflect"
	smithy "github.com/aws/smithy-go"
)

func TestAuthOptions(t *testing.T) {
	var ip smithy.Properties
	ip.Set("foo", "bar")

	var sp smithy.Properties
	sp.Set("foo", "bar")

	expected := []*Option{
		&Option{
			SchemeID: "fakeSchemeID",
			IdentityProperties: ip,
			SignerProperties: sp,
		},	
	}

	var m smithy.Properties
	SetAuthOptions(&m, expected)
	actual, _ := GetAuthOptions(&m)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expect AuthOptions to be equivalent %v != %v", expected, actual)
	}
}