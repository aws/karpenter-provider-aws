package aws

import (
	"strconv"
	"testing"
)

type mockOptions struct {
	Bool                   bool
	Str                    string
	DualStackEndpointState DualStackEndpointState
	FIPSEndpointState      FIPSEndpointState
}

func (m mockOptions) GetDisableHTTPS() bool {
	return m.Bool
}

func (m mockOptions) GetUseDualStackEndpoint() DualStackEndpointState {
	return m.DualStackEndpointState
}

func (m mockOptions) GetUseFIPSEndpoint() FIPSEndpointState {
	return m.FIPSEndpointState
}

func (m mockOptions) GetResolvedRegion() string {
	return m.Str
}

func TestGetDisableHTTPS(t *testing.T) {
	cases := []struct {
		Options     []interface{}
		ExpectFound bool
		ExpectValue bool
	}{
		{
			Options: []interface{}{struct{}{}},
		},
		{
			Options: []interface{}{mockOptions{
				Bool: false,
			}},
			ExpectFound: true,
			ExpectValue: false,
		},
		{
			Options: []interface{}{mockOptions{
				Bool: true,
			}},
			ExpectFound: true,
			ExpectValue: true,
		},
		{
			Options:     []interface{}{struct{}{}, mockOptions{Bool: true}, mockOptions{Bool: false}},
			ExpectFound: true,
			ExpectValue: true,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value, found := GetDisableHTTPS(tt.Options...)
			if found != tt.ExpectFound {
				t.Fatalf("expect value to not be found")
			}
			if value != tt.ExpectValue {
				t.Errorf("expect %v, got %v", tt.ExpectValue, value)
			}
		})
	}
}

func TestGetResolvedRegion(t *testing.T) {
	cases := []struct {
		Options     []interface{}
		ExpectFound bool
		ExpectValue string
	}{
		{
			Options: []interface{}{struct{}{}},
		},
		{
			Options:     []interface{}{mockOptions{Str: ""}},
			ExpectFound: true,
			ExpectValue: "",
		},
		{
			Options:     []interface{}{mockOptions{Str: "foo"}},
			ExpectFound: true,
			ExpectValue: "foo",
		},
		{
			Options:     []interface{}{struct{}{}, mockOptions{Str: "bar"}, mockOptions{Str: "baz"}},
			ExpectFound: true,
			ExpectValue: "bar",
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value, found := GetResolvedRegion(tt.Options...)
			if found != tt.ExpectFound {
				t.Fatalf("expect value to not be found")
			}
			if value != tt.ExpectValue {
				t.Errorf("expect %v, got %v", tt.ExpectValue, value)
			}
		})
	}
}

func TestGetUseDualStackEndpoint(t *testing.T) {
	cases := []struct {
		Options     []interface{}
		ExpectFound bool
		ExpectValue DualStackEndpointState
	}{
		{
			Options: []interface{}{struct{}{}},
		},
		{
			Options:     []interface{}{mockOptions{DualStackEndpointState: DualStackEndpointStateUnset}},
			ExpectFound: true,
			ExpectValue: DualStackEndpointStateUnset,
		},
		{
			Options:     []interface{}{mockOptions{DualStackEndpointState: DualStackEndpointStateEnabled}},
			ExpectFound: true,
			ExpectValue: DualStackEndpointStateEnabled,
		},
		{
			Options:     []interface{}{struct{}{}, mockOptions{DualStackEndpointState: DualStackEndpointStateEnabled}, mockOptions{DualStackEndpointState: DualStackEndpointStateDisabled}},
			ExpectFound: true,
			ExpectValue: DualStackEndpointStateEnabled,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value, found := GetUseDualStackEndpoint(tt.Options...)
			if found != tt.ExpectFound {
				t.Fatalf("expect value to not be found")
			}
			if value != tt.ExpectValue {
				t.Errorf("expect %v, got %v", tt.ExpectValue, value)
			}
		})
	}
}

func TestGetUseFIPSEndpoint(t *testing.T) {
	cases := []struct {
		Options     []interface{}
		ExpectFound bool
		ExpectValue FIPSEndpointState
	}{
		{
			Options: []interface{}{struct{}{}},
		},
		{
			Options:     []interface{}{mockOptions{FIPSEndpointState: FIPSEndpointStateUnset}},
			ExpectFound: true,
			ExpectValue: FIPSEndpointStateUnset,
		},
		{
			Options:     []interface{}{mockOptions{FIPSEndpointState: FIPSEndpointStateEnabled}},
			ExpectFound: true,
			ExpectValue: FIPSEndpointStateEnabled,
		},
		{
			Options:     []interface{}{struct{}{}, mockOptions{FIPSEndpointState: FIPSEndpointStateEnabled}, mockOptions{FIPSEndpointState: FIPSEndpointStateDisabled}},
			ExpectFound: true,
			ExpectValue: FIPSEndpointStateEnabled,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			value, found := GetUseFIPSEndpoint(tt.Options...)
			if found != tt.ExpectFound {
				t.Fatalf("expect value to not be found")
			}
			if value != tt.ExpectValue {
				t.Errorf("expect %v, got %v", tt.ExpectValue, value)
			}
		})
	}
}

var _ EndpointResolverWithOptions = EndpointResolverWithOptionsFunc(nil)

func TestEndpointResolverWithOptionsFunc_ResolveEndpoint(t *testing.T) {
	var er EndpointResolverWithOptions = EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (Endpoint, error) {
		if e, a := "foo", service; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
		if e, a := "bar", region; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
		if e, a := 2, len(options); e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
		return Endpoint{
			URL: "https://foo.amazonaws.com",
		}, nil
	})

	e, err := er.ResolveEndpoint("foo", "bar", 1, 2)
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if e, a := "https://foo.amazonaws.com", e.URL; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}
