package endpoints

import (
	"reflect"
	"testing"
)

func TestEndpointResolve(t *testing.T) {
	defs := Endpoint{
		Hostname:          "service.{region}.amazonaws.com",
		SignatureVersions: []string{"v4"},
	}

	e := Endpoint{
		Protocols:         []string{"http", "https"},
		SignatureVersions: []string{"v4"},
		CredentialScope: CredentialScope{
			Region:  "us-west-2",
			Service: "service",
		},
	}

	resolved, err := e.resolve("aws", "us-west-2", defs, Options{})
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if e, a := "https://service.us-west-2.amazonaws.com", resolved.URL; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := "aws", resolved.PartitionID; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := "service", resolved.SigningName; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := "us-west-2", resolved.SigningRegion; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := "v4", resolved.SigningMethod; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestEndpointMergeIn(t *testing.T) {
	expected := Endpoint{
		Hostname:          "other hostname",
		Protocols:         []string{"http"},
		SignatureVersions: []string{"v4"},
		CredentialScope: CredentialScope{
			Region:  "region",
			Service: "service",
		},
	}

	actual := Endpoint{}
	actual.mergeIn(Endpoint{
		Hostname:          "other hostname",
		Protocols:         []string{"http"},
		SignatureVersions: []string{"v4"},
		CredentialScope: CredentialScope{
			Region:  "region",
			Service: "service",
		},
	})

	if e, a := expected, actual; !reflect.DeepEqual(e, a) {
		t.Errorf("expect %v, got %v", e, a)
	}
}
