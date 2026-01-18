package auth

import (
	"testing"

	smithy "github.com/aws/smithy-go"
)

func TestV4(t *testing.T) {

	propsV4 := smithy.Properties{}

	propsV4.Set("authSchemes", interface{}([]interface{}{
		map[string]interface{}{
			"disableDoubleEncoding": true,
			"name":                  "sigv4",
			"signingName":           "s3",
			"signingRegion":         "us-west-2",
		},
	}))

	result, err := GetAuthenticationSchemes(&propsV4)
	if err != nil {
		t.Fatalf("Did not expect error, got %v", err)
	}

	_, ok := result[0].(AuthenticationScheme)
	if !ok {
		t.Fatalf("Did not get expected AuthenticationScheme. %v", result[0])
	}

	v4Scheme, ok := result[0].(*AuthenticationSchemeV4)
	if !ok {
		t.Fatalf("Did not get expected AuthenticationSchemeV4. %v", result[0])
	}

	if v4Scheme.Name != "sigv4" {
		t.Fatalf("Did not get expected AuthenticationSchemeV4 signer version name")
	}

}

func TestV4A(t *testing.T) {

	propsV4A := smithy.Properties{}

	propsV4A.Set("authSchemes", []interface{}{
		map[string]interface{}{
			"disableDoubleEncoding": true,
			"name":                  "sigv4a",
			"signingName":           "s3",
			"signingRegionSet":      []string{"*"},
		},
	})

	result, err := GetAuthenticationSchemes(&propsV4A)
	if err != nil {
		t.Fatalf("Did not expect error, got %v", err)
	}

	_, ok := result[0].(AuthenticationScheme)
	if !ok {
		t.Fatalf("Did not get expected AuthenticationScheme. %v", result[0])
	}

	v4AScheme, ok := result[0].(*AuthenticationSchemeV4A)
	if !ok {
		t.Fatalf("Did not get expected AuthenticationSchemeV4A. %v", result[0])
	}

	if v4AScheme.Name != "sigv4a" {
		t.Fatalf("Did not get expected AuthenticationSchemeV4A signer version name")
	}

}

func TestV4S3Express(t *testing.T) {
	props := smithy.Properties{}
	props.Set("authSchemes", []interface{}{
		map[string]interface{}{
			"name":                  SigV4S3Express,
			"signingName":           "s3",
			"signingRegion":         "us-east-1",
			"disableDoubleEncoding": true,
		},
	})

	result, err := GetAuthenticationSchemes(&props)
	if err != nil {
		t.Fatalf("Did not expect error, got %v", err)
	}

	scheme, ok := result[0].(*AuthenticationSchemeV4)
	if !ok {
		t.Fatalf("Did not get expected AuthenticationSchemeV4. %v", result[0])
	}

	if scheme.Name != SigV4S3Express {
		t.Fatalf("expected %s, got %s", SigV4S3Express, scheme.Name)
	}
}
