package v4

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4Internal "github.com/aws/aws-sdk-go-v2/aws/signer/internal/v4"
)

var testCredentials = aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "SESSION"}

func buildRequest(serviceName, region, body string) (*http.Request, string) {
	reader := strings.NewReader(body)
	return buildRequestWithBodyReader(serviceName, region, reader)
}

func buildRequestWithBodyReader(serviceName, region string, body io.Reader) (*http.Request, string) {
	var bodyLen int

	type lenner interface {
		Len() int
	}
	if lr, ok := body.(lenner); ok {
		bodyLen = lr.Len()
	}

	endpoint := "https://" + serviceName + "." + region + ".amazonaws.com"
	req, _ := http.NewRequest("POST", endpoint, body)
	req.URL.Opaque = "//example.org/bucket/key-._~,!@#$%^&*()"
	req.Header.Set("X-Amz-Target", "prefix.Operation")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")

	if bodyLen > 0 {
		req.ContentLength = int64(bodyLen)
	}

	req.Header.Set("X-Amz-Meta-Other-Header", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-Amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")

	h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	return req, payloadHash
}

func TestPresignRequest(t *testing.T) {
	req, body := buildRequest("dynamodb", "us-east-1", "{}")

	query := req.URL.Query()
	query.Set("X-Amz-Expires", "300")
	req.URL.RawQuery = query.Encode()

	signer := NewSigner()
	signed, headers, err := signer.PresignHTTP(context.Background(), testCredentials, req, body, "dynamodb", "us-east-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedDate := "19700101T000000Z"
	expectedHeaders := "content-length;content-type;host;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore"
	expectedSig := "122f0b9e091e4ba84286097e2b3404a1f1f4c4aad479adda95b7dff0ccbe5581"
	expectedCred := "AKID/19700101/us-east-1/dynamodb/aws4_request"
	expectedTarget := "prefix.Operation"

	q, err := url.ParseQuery(signed[strings.Index(signed, "?"):])
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	if e, a := expectedSig, q.Get("X-Amz-Signature"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedCred, q.Get("X-Amz-Credential"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedHeaders, q.Get("X-Amz-SignedHeaders"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if a := q.Get("X-Amz-Meta-Other-Header"); len(a) != 0 {
		t.Errorf("expect %v to be empty", a)
	}
	if e, a := expectedTarget, q.Get("X-Amz-Target"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	for _, h := range strings.Split(expectedHeaders, ";") {
		v := headers.Get(h)
		if len(v) == 0 {
			t.Errorf("expect %v, to be present in header map", h)
		}
	}
}

func TestPresignBodyWithArrayRequest(t *testing.T) {
	req, body := buildRequest("dynamodb", "us-east-1", "{}")
	req.URL.RawQuery = "Foo=z&Foo=o&Foo=m&Foo=a"

	query := req.URL.Query()
	query.Set("X-Amz-Expires", "300")
	req.URL.RawQuery = query.Encode()

	signer := NewSigner()
	signed, headers, err := signer.PresignHTTP(context.Background(), testCredentials, req, body, "dynamodb", "us-east-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	q, err := url.ParseQuery(signed[strings.Index(signed, "?"):])
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}

	expectedDate := "19700101T000000Z"
	expectedHeaders := "content-length;content-type;host;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore"
	expectedSig := "e3ac55addee8711b76c6d608d762cff285fe8b627a057f8b5ec9268cf82c08b1"
	expectedCred := "AKID/19700101/us-east-1/dynamodb/aws4_request"
	expectedTarget := "prefix.Operation"

	if e, a := expectedSig, q.Get("X-Amz-Signature"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedCred, q.Get("X-Amz-Credential"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedHeaders, q.Get("X-Amz-SignedHeaders"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if a := q.Get("X-Amz-Meta-Other-Header"); len(a) != 0 {
		t.Errorf("expect %v to be empty, was not", a)
	}
	if e, a := expectedTarget, q.Get("X-Amz-Target"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	for _, h := range strings.Split(expectedHeaders, ";") {
		v := headers.Get(h)
		if len(v) == 0 {
			t.Errorf("expect %v, to be present in header map", h)
		}
	}
}

func TestSignRequest(t *testing.T) {
	req, body := buildRequest("dynamodb", "us-east-1", "{}")
	// test ignored headers
	req.Header.Set("User-Agent", "foo")
	req.Header.Set("X-Amzn-Trace-Id", "bar")
	req.Header.Set("Expect", "baz")
	req.Header.Set("Transfer-Encoding", "qux")
	signer := NewSigner()
	err := signer.SignHTTP(context.Background(), testCredentials, req, body, "dynamodb", "us-east-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	expectedDate := "19700101T000000Z"
	expectedSig := "AWS4-HMAC-SHA256 Credential=AKID/19700101/us-east-1/dynamodb/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore;x-amz-security-token;x-amz-target, Signature=a518299330494908a70222cec6899f6f32f297f8595f6df1776d998936652ad9"

	q := req.Header
	if e, a := expectedSig, q.Get("Authorization"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestBuildCanonicalRequest(t *testing.T) {
	req, _ := buildRequest("dynamodb", "us-east-1", "{}")
	req.URL.RawQuery = "Foo=z&Foo=o&Foo=m&Foo=a"

	ctx := &httpSigner{
		ServiceName:  "dynamodb",
		Region:       "us-east-1",
		Request:      req,
		Time:         v4Internal.NewSigningTime(time.Now()),
		KeyDerivator: v4Internal.NewSigningKeyDeriver(),
	}

	build, err := ctx.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "https://example.org/bucket/key-._~,!@#$%^&*()?Foo=a&Foo=m&Foo=o&Foo=z"
	if e, a := expected, build.Request.URL.String(); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestSigner_SignHTTP_NoReplaceRequestBody(t *testing.T) {
	req, bodyHash := buildRequest("dynamodb", "us-east-1", "{}")
	req.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))

	s := NewSigner()

	origBody := req.Body

	err := s.SignHTTP(context.Background(), testCredentials, req, bodyHash, "dynamodb", "us-east-1", time.Now())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	if req.Body != origBody {
		t.Errorf("expect request body to not be chagned")
	}
}

func TestRequestHost(t *testing.T) {
	req, _ := buildRequest("dynamodb", "us-east-1", "{}")
	req.URL.RawQuery = "Foo=z&Foo=o&Foo=m&Foo=a"
	req.Host = "myhost"

	query := req.URL.Query()
	query.Set("X-Amz-Expires", "5")
	req.URL.RawQuery = query.Encode()

	ctx := &httpSigner{
		ServiceName:  "dynamodb",
		Region:       "us-east-1",
		Request:      req,
		Time:         v4Internal.NewSigningTime(time.Now()),
		KeyDerivator: v4Internal.NewSigningKeyDeriver(),
	}

	build, err := ctx.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(build.CanonicalString, "host:"+req.Host) {
		t.Errorf("canonical host header invalid")
	}
}

func TestSign_buildCanonicalHeadersContentLengthPresent(t *testing.T) {
	body := `{"description": "this is a test"}`
	req, _ := buildRequest("dynamodb", "us-east-1", body)
	req.URL.RawQuery = "Foo=z&Foo=o&Foo=m&Foo=a"
	req.Host = "myhost"

	contentLength := fmt.Sprintf("%d", len([]byte(body)))
	req.Header.Add("Content-Length", contentLength)

	query := req.URL.Query()
	query.Set("X-Amz-Expires", "5")
	req.URL.RawQuery = query.Encode()

	ctx := &httpSigner{
		ServiceName:  "dynamodb",
		Region:       "us-east-1",
		Request:      req,
		Time:         v4Internal.NewSigningTime(time.Now()),
		KeyDerivator: v4Internal.NewSigningKeyDeriver(),
	}

	build, err := ctx.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(build.CanonicalString, "content-length:"+contentLength+"\n") {
		t.Errorf("canonical header content-length invalid")
	}
}

func TestSign_buildCanonicalHeaders(t *testing.T) {
	serviceName := "mockAPI"
	region := "mock-region"
	endpoint := "https://" + serviceName + "." + region + ".amazonaws.com"

	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		t.Fatalf("failed to create request, %v", err)
	}

	req.Header.Set("FooInnerSpace", "   inner      space    ")
	req.Header.Set("FooLeadingSpace", "    leading-space")
	req.Header.Add("FooMultipleSpace", "no-space")
	req.Header.Add("FooMultipleSpace", "\ttab-space")
	req.Header.Add("FooMultipleSpace", "trailing-space    ")
	req.Header.Set("FooNoSpace", "no-space")
	req.Header.Set("FooTabSpace", "\ttab-space\t")
	req.Header.Set("FooTrailingSpace", "trailing-space    ")
	req.Header.Set("FooWrappedSpace", "   wrapped-space    ")

	ctx := &httpSigner{
		ServiceName:  serviceName,
		Region:       region,
		Request:      req,
		Time:         v4Internal.NewSigningTime(time.Date(2021, 10, 20, 12, 42, 0, 0, time.UTC)),
		KeyDerivator: v4Internal.NewSigningKeyDeriver(),
	}

	build, err := ctx.Build()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectCanonicalString := strings.Join([]string{
		`POST`,
		`/`,
		``,
		`fooinnerspace:inner space`,
		`fooleadingspace:leading-space`,
		`foomultiplespace:no-space,tab-space,trailing-space`,
		`foonospace:no-space`,
		`footabspace:tab-space`,
		`footrailingspace:trailing-space`,
		`foowrappedspace:wrapped-space`,
		`host:mockAPI.mock-region.amazonaws.com`,
		`x-amz-date:20211020T124200Z`,
		``,
		`fooinnerspace;fooleadingspace;foomultiplespace;foonospace;footabspace;footrailingspace;foowrappedspace;host;x-amz-date`,
		``,
	}, "\n")
	if diff := cmpDiff(expectCanonicalString, build.CanonicalString); diff != "" {
		t.Errorf("expect match, got\n%s", diff)
	}
}

func BenchmarkPresignRequest(b *testing.B) {
	signer := NewSigner()
	req, bodyHash := buildRequest("dynamodb", "us-east-1", "{}")

	query := req.URL.Query()
	query.Set("X-Amz-Expires", "5")
	req.URL.RawQuery = query.Encode()

	for i := 0; i < b.N; i++ {
		signer.PresignHTTP(context.Background(), testCredentials, req, bodyHash, "dynamodb", "us-east-1", time.Now())
	}
}

func BenchmarkSignRequest(b *testing.B) {
	signer := NewSigner()
	req, bodyHash := buildRequest("dynamodb", "us-east-1", "{}")
	for i := 0; i < b.N; i++ {
		signer.SignHTTP(context.Background(), testCredentials, req, bodyHash, "dynamodb", "us-east-1", time.Now())
	}
}

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
