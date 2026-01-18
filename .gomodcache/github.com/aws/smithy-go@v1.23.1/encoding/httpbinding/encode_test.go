package httpbinding

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestEncoder(t *testing.T) {
	actual := &http.Request{
		Header: http.Header{
			"custom-user-header": {"someValue"},
		},
		URL: &url.URL{
			Path:     "/some/{pathKeyOne}/{pathKeyTwo}",
			RawQuery: "someExistingKeys=foobar",
		},
	}

	expected := &http.Request{
		Header: map[string][]string{
			"custom-user-header": {"someValue"},
			"x-amzn-header-foo":  {"someValue"},
			"x-amzn-meta-foo":    {"someValue"},
		},
		URL: &url.URL{
			Path:     "/some/someValue/path",
			RawPath:  "/some/someValue/path",
			RawQuery: "someExistingKeys=foobar&someKey=someValue&someKey=otherValue",
		},
	}

	encoder, err := NewEncoder(actual.URL.Path, actual.URL.RawQuery, actual.Header)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Headers
	encoder.AddHeader("x-amzn-header-foo").String("someValue")
	encoder.Headers("x-amzn-meta-").AddHeader("foo").String("someValue")

	// Query
	encoder.SetQuery("someKey").String("someValue")
	encoder.AddQuery("someKey").String("otherValue")

	// URI
	if err := encoder.SetURI("pathKeyOne").String("someValue"); err != nil {
		t.Errorf("expected no err, but got %v", err)
	}

	// URI
	if err := encoder.SetURI("pathKeyTwo").String("path"); err != nil {
		t.Errorf("expected no err, but got %v", err)
	}

	if actual, err = encoder.Encode(actual); err != nil {
		t.Errorf("expected no err, but got %v", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, but got %v", expected, actual)
	}
}

func TestEncoderHasHeader(t *testing.T) {
	encoder, err := NewEncoder("/", "", http.Header{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if h := "i-dont-exist"; encoder.HasHeader(h) {
		t.Errorf("expect %v not to be set", h)
	}

	encoder.AddHeader("I-do-exist").String("some value")

	if h := "I-do-exist"; !encoder.HasHeader(h) {
		t.Errorf("expect %v to be set", h)
	}

}

func TestEncoderHasQuery(t *testing.T) {
	encoder, err := NewEncoder("/", "", http.Header{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if q := "i-dont-exist"; encoder.HasQuery(q) {
		t.Errorf("expect %v not to be set", q)
	}

	encoder.AddQuery("I-do-exist").String("some value")

	if q := "I-do-exist"; !encoder.HasQuery(q) {
		t.Errorf("expect %v to be set", q)
	}

}

func TestEncodeContentLength(t *testing.T) {
	cases := map[string]struct {
		headerValue string
		expected    int64
		wantErr     bool
	}{
		"valid number": {
			headerValue: "1024",
			expected:    1024,
		},
		"invalid number": {
			headerValue: "1024.5",
			wantErr:     true,
		},
		"not a number": {
			headerValue: "NaN",
			wantErr:     true,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			encoder, err := NewEncoder("/", "", http.Header{})
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			encoder.SetHeader("Content-Length").String(tt.headerValue)

			req := &http.Request{URL: &url.URL{}}
			req, err = encoder.Encode(req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error value wantErr=%v", tt.wantErr)
			} else if tt.wantErr {
				return
			}
			if e, a := tt.expected, req.ContentLength; e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
			if v := req.Header.Get("Content-Length"); len(v) > 0 {
				t.Errorf("expect header not to be set")
			}
		})
	}
}
