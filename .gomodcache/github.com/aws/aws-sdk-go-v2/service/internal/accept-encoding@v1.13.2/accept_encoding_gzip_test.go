package acceptencoding

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestAddAcceptEncodingGzip(t *testing.T) {
	cases := map[string]struct {
		Enable bool
	}{
		"disabled": {
			Enable: false,
		},
		"enabled": {
			Enable: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stack := middleware.NewStack("test", smithyhttp.NewStackRequest)

			stack.Deserialize.Add(&stubOpDeserializer{}, middleware.After)

			AddAcceptEncodingGzip(stack, AddAcceptEncodingGzipOptions{
				Enable: c.Enable,
			})

			id := "OperationDeserializer"
			if m, ok := stack.Deserialize.Get(id); !ok || m == nil {
				t.Fatalf("expect %s not to be removed", id)
			}

			if c.Enable {
				id = (*EnableGzip)(nil).ID()
				if m, ok := stack.Finalize.Get(id); !ok || m == nil {
					t.Fatalf("expect %s to be present.", id)
				}

				id = (*DecompressGzip)(nil).ID()
				if m, ok := stack.Deserialize.Get(id); !ok || m == nil {
					t.Fatalf("expect %s to be present.", id)
				}
				return
			}
			id = (*EnableGzip)(nil).ID()
			if m, ok := stack.Finalize.Get(id); ok || m != nil {
				t.Fatalf("expect %s not to be present.", id)
			}

			id = (*DecompressGzip)(nil).ID()
			if m, ok := stack.Deserialize.Get(id); ok || m != nil {
				t.Fatalf("expect %s not to be present.", id)
			}
		})
	}
}

func TestAcceptEncodingGzipMiddleware(t *testing.T) {
	m := &EnableGzip{}

	_, _, err := m.HandleFinalize(context.Background(),
		middleware.FinalizeInput{
			Request: smithyhttp.NewStackRequest(),
		},
		middleware.FinalizeHandlerFunc(
			func(ctx context.Context, input middleware.FinalizeInput) (
				output middleware.FinalizeOutput, metadata middleware.Metadata, err error,
			) {
				req, ok := input.Request.(*smithyhttp.Request)
				if !ok || req == nil {
					t.Fatalf("expect smithy request, got %T", input.Request)
				}

				actual := req.Header.Get(acceptEncodingHeaderKey)
				if e, a := "gzip", actual; e != a {
					t.Errorf("expect %v accept-encoding, got %v", e, a)
				}

				return output, metadata, err
			}),
	)
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
}

func TestDecompressGzipMiddleware(t *testing.T) {
	cases := map[string]struct {
		Response            *smithyhttp.Response
		ExpectBody          []byte
		ExpectContentLength int64
	}{
		"not compressed": {
			Response: &smithyhttp.Response{
				Response: &http.Response{
					StatusCode:    200,
					Header:        http.Header{},
					ContentLength: 2,
					Body: &wasClosedReadCloser{
						Reader: bytes.NewBuffer([]byte(`{}`)),
					},
				},
			},
			ExpectBody:          []byte(`{}`),
			ExpectContentLength: 2,
		},
		"compressed": {
			Response: &smithyhttp.Response{
				Response: &http.Response{
					StatusCode: 200,
					Header: http.Header{
						contentEncodingHeaderKey: []string{"gzip"},
					},
					ContentLength: 10,
					Body: func() io.ReadCloser {
						var buf bytes.Buffer
						w := gzip.NewWriter(&buf)
						w.Write([]byte(`{}`))
						w.Close()

						return &wasClosedReadCloser{Reader: &buf}
					}(),
				},
			},
			ExpectBody:          []byte(`{}`),
			ExpectContentLength: -1, // Length empty because was decompressed
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := &DecompressGzip{}

			var origRespBody io.Reader
			output, _, err := m.HandleDeserialize(context.Background(),
				middleware.DeserializeInput{},
				middleware.DeserializeHandlerFunc(
					func(ctx context.Context, input middleware.DeserializeInput) (
						output middleware.DeserializeOutput, metadata middleware.Metadata, err error,
					) {
						output.RawResponse = c.Response
						origRespBody = c.Response.Body
						return output, metadata, err
					}),
			)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			resp, ok := output.RawResponse.(*smithyhttp.Response)
			if !ok || resp == nil {
				t.Fatalf("expect smithy request, got %T", output.RawResponse)
			}

			if e, a := c.ExpectContentLength, resp.ContentLength; e != a {
				t.Errorf("expect %v content-length, got %v", e, a)
			}

			actual, err := ioutil.ReadAll(resp.Body)
			if e, a := c.ExpectBody, actual; !bytes.Equal(e, a) {
				t.Errorf("expect body equal\nexpect:\n%s\nactual:\n%s",
					hex.Dump(e), hex.Dump(a))
			}

			if err := resp.Body.Close(); err != nil {
				t.Fatalf("expect no close error, got %v", err)
			}

			if c, ok := origRespBody.(interface{ WasClosed() bool }); ok {
				if !c.WasClosed() {
					t.Errorf("expect original reader closed, but was not")
				}
			}
		})
	}
}

type stubOpDeserializer struct{}

func (*stubOpDeserializer) ID() string { return "OperationDeserializer" }
func (*stubOpDeserializer) HandleDeserialize(
	ctx context.Context, input middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	output middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	return next.HandleDeserialize(ctx, input)
}

type wasClosedReadCloser struct {
	io.Reader
	closed bool
}

func (c *wasClosedReadCloser) WasClosed() bool {
	return c.closed
}

func (c *wasClosedReadCloser) Close() error {
	c.closed = true
	if v, ok := c.Reader.(io.Closer); ok {
		return v.Close()
	}
	return nil
}
