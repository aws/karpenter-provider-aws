package protocol

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithytesting "github.com/aws/smithy-go/testing"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// TestAddCaptureRequestMiddleware tests AddCaptureRequestMiddleware
func TestAddCaptureRequestMiddleware(t *testing.T) {
	cases := map[string]struct {
		Request       *http.Request
		ExpectRequest *http.Request
		ExpectQuery   []smithytesting.QueryItem
		Stream        io.Reader
	}{
		"normal request": {
			Request: &http.Request{
				Method: "PUT",
				Header: map[string][]string{
					"Foo":      {"bar", "too"},
					"Checksum": {"SHA256"},
				},
				URL: &url.URL{
					Path:     "test/path",
					RawQuery: "language=us&region=us-west+east",
				},
				ContentLength: 100,
			},
			ExpectRequest: &http.Request{
				Method: "PUT",
				Header: map[string][]string{
					"Foo":            {"bar", "too"},
					"Checksum":       {"SHA256"},
					"Content-Length": {"100"},
				},
				URL: &url.URL{
					Path:    "test/path",
					RawPath: "test/path",
				},
				Body: io.NopCloser(strings.NewReader("hello world.")),
			},
			ExpectQuery: []smithytesting.QueryItem{
				{
					Key:   "language",
					Value: "us",
				},
				{
					Key:   "region",
					Value: "us-west%20east",
				},
			},
			Stream: strings.NewReader("hello world."),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := &smithyhttp.Request{
				Request: c.Request,
			}
			if c.Stream != nil {
				req, err = req.SetStream(c.Stream)
				if err != nil {
					t.Fatalf("Got error while retrieving case stream: %v", err)
				}
			}
			capturedRequest := &http.Request{}
			m := captureRequestMiddleware{
				req: capturedRequest,
			}
			_, _, err = m.HandleBuild(context.Background(),
				middleware.BuildInput{Request: req},
				middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (
					out middleware.BuildOutput, metadata middleware.Metadata, err error) {
					return out, metadata, nil
				}),
			)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if e, a := c.ExpectRequest.Method, capturedRequest.Method; e != a {
				t.Errorf("expect request method %v found, got %v", e, a)
			}
			if e, a := c.ExpectRequest.URL.Path, capturedRequest.URL.RawPath; e != a {
				t.Errorf("expect %v path, got %v", e, a)
			}
			if c.ExpectRequest.Body != nil {
				expect, err := io.ReadAll(c.ExpectRequest.Body)
				if capturedRequest.Body == nil {
					t.Errorf("Expect request stream %v captured, get nil", string(expect))
				}
				actual, err := io.ReadAll(capturedRequest.Body)
				if err != nil {
					t.Errorf("unable to read captured request body, %v", err)
				}
				if e, a := string(expect), string(actual); e != a {
					t.Errorf("expect request body to be %s, got %s", e, a)
				}
			}
			queryItems := smithytesting.ParseRawQuery(capturedRequest.URL.RawQuery)
			smithytesting.AssertHasQuery(t, c.ExpectQuery, queryItems)
			smithytesting.AssertHasHeader(t, c.ExpectRequest.Header, capturedRequest.Header)
		})
	}
}
