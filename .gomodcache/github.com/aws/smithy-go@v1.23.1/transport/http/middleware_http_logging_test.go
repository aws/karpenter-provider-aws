package http_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type mockLogger struct {
	bytes.Buffer
}

func (m *mockLogger) Logf(_ logging.Classification, format string, v ...interface{}) {
	m.Buffer.WriteString(fmt.Sprintf(format, v...))
	m.Buffer.WriteRune('\n')
}

func TestRequestResponseLogger(t *testing.T) {
	cases := map[string]struct {
		Middleware  smithyhttp.RequestResponseLogger
		Input       *smithyhttp.Request
		InputBody   io.ReadCloser
		Output      *smithyhttp.Response
		ExpectedLog string
	}{
		"no logging": {},
		"request": {
			Middleware: smithyhttp.RequestResponseLogger{
				LogRequest: true,
			},
			Input: &smithyhttp.Request{
				Request: &http.Request{
					URL: &url.URL{
						Scheme: "https",
						Path:   "/foo",
						Host:   "example.amazonaws.com",
					},
					Header: map[string][]string{
						"Foo": {"bar"},
					},
				},
			},
			InputBody: io.NopCloser(bytes.NewReader([]byte(`this is the body`))),
			ExpectedLog: "Request\n" +
				"GET /foo HTTP/1.1\r\n" +
				"Host: example.amazonaws.com\r\n" +
				"User-Agent: Go-http-client/1.1\r\n" +
				"Foo: bar\r\n" +
				"Accept-Encoding: gzip\r\n" +
				"\r\n\n",
		},
		"request with body": {
			Middleware: smithyhttp.RequestResponseLogger{
				LogRequestWithBody: true,
			},
			Input: &smithyhttp.Request{
				Request: &http.Request{
					URL: &url.URL{
						Scheme: "https",
						Path:   "/foo",
						Host:   "example.amazonaws.com",
					},
					Header: map[string][]string{
						"Foo": {"bar"},
					},
					ContentLength: 16,
				},
			},
			InputBody: io.NopCloser(bytes.NewReader([]byte(`this is the body`))),
			ExpectedLog: "Request\n" +
				"GET /foo HTTP/1.1\r\n" +
				"Host: example.amazonaws.com\r\n" +
				"User-Agent: Go-http-client/1.1\r\n" +
				"Content-Length: 16\r\n" +
				"Foo: bar\r\n" +
				"Accept-Encoding: gzip\r\n" +
				"\r\n" +
				"this is the body\n",
		},
		"response": {
			Middleware: smithyhttp.RequestResponseLogger{
				LogResponse: true,
			},
			Output: &smithyhttp.Response{
				Response: &http.Response{
					StatusCode:    200,
					Proto:         "HTTP/1.1",
					ContentLength: 16,
					Header: map[string][]string{
						"Foo": {"Bar"},
					},
					Body: io.NopCloser(bytes.NewReader([]byte(`this is the body`))),
				},
			},
			ExpectedLog: "Response\n" +
				"HTTP/0.0 200 OK\r\n" +
				"Content-Length: 16\r\n" +
				"Foo: Bar\r\n" +
				"\r\n\n",
		},
		"response with body": {
			Middleware: smithyhttp.RequestResponseLogger{
				LogResponseWithBody: true,
			},
			Output: &smithyhttp.Response{
				Response: &http.Response{
					StatusCode:    200,
					Proto:         "HTTP/1.1",
					ContentLength: 16,
					Header: map[string][]string{
						"Foo": {"Bar"},
					},
					Body: io.NopCloser(bytes.NewReader([]byte(`this is the body`))),
				},
			},
			ExpectedLog: "Response\n" +
				"HTTP/0.0 200 OK\r\n" +
				"Content-Length: 16\r\n" +
				"Foo: Bar\r\n" +
				"\r\n" +
				"this is the body\n",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			logger := mockLogger{}
			ctx := middleware.SetLogger(context.Background(), &logger)

			var err error
			if tt.InputBody != nil {
				tt.Input, err = tt.Input.SetStream(tt.InputBody)
				if err != nil {
					t.Errorf("expect no error, got %v", err)
				}
			}

			_, _, err = tt.Middleware.HandleDeserialize(ctx, middleware.DeserializeInput{Request: tt.Input}, middleware.DeserializeHandlerFunc(func(ctx context.Context, input middleware.DeserializeInput) (
				middleware.DeserializeOutput, middleware.Metadata, error,
			) {
				return middleware.DeserializeOutput{RawResponse: tt.Output}, middleware.Metadata{}, nil
			}))
			if err != nil {
				t.Fatal("expect error, got nil")
			}

			actual := string(logger.Bytes())
			if tt.ExpectedLog != actual {
				t.Errorf("%v != %v", tt.ExpectedLog, actual)
			}
		})
	}
}
