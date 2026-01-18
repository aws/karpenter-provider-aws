package http

import (
	"testing"
	"reflect"
	smithy "github.com/aws/smithy-go"
)


func TestSigV4SigningName(t *testing.T) {
	expected := "foo"
	var m smithy.Properties
	SetSigV4SigningName(&m, expected)
	actual, _ := GetSigV4SigningName(&m)

	if expected != actual {
		t.Errorf("Expect SigV4SigningName to be equivalent %s != %s", expected, actual)
	}
}

func TestSigV4SigningRegion(t *testing.T) {
	expected := "foo"
	var m smithy.Properties
	SetSigV4SigningRegion(&m, expected)
	actual, _ := GetSigV4SigningRegion(&m)

	if expected != actual {
		t.Errorf("Expect SigV4SigningRegion to be equivalent %s != %s", expected, actual)
	}
}

func TestSigV4ASigningName(t *testing.T) {
	expected := "foo"
	var m smithy.Properties
	SetSigV4ASigningName(&m, expected)
	actual, _ := GetSigV4ASigningName(&m)

	if expected != actual {
		t.Errorf("Expect SigV4ASigningName to be equivalent %s != %s", expected, actual)
	}
}

func TestSigV4SigningRegions(t *testing.T) {
	expected := []string{"foo", "bar"}
	var m smithy.Properties
	SetSigV4ASigningRegions(&m, expected)
	actual, _ := GetSigV4ASigningRegions(&m)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expect SigV4ASigningRegions to be equivalent %v != %v", expected, actual)
	}
}

func TestUnsignedPayload(t *testing.T) {
	expected := true
	var m smithy.Properties
	SetIsUnsignedPayload(&m, expected)
	actual, _ := GetIsUnsignedPayload(&m)

	if expected != actual {
		t.Errorf("Expect IsUnsignedPayload to be equivalent %v != %v", expected, actual)
	}
}

func TestDisableDoubleEncoding(t *testing.T) {
	expected := true
	var m smithy.Properties
	SetDisableDoubleEncoding(&m, expected)
	actual, _ := GetDisableDoubleEncoding(&m)

	if expected != actual {
		t.Errorf("Expect DisableDoubleEncoding to be equivalent %v != %v", expected, actual)
	}
}