package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/smithy-go/middleware"
)

func TestContentLengthMiddleware(t *testing.T) {
	cases := map[string]struct {
		Stream          io.Reader
		ExpectNilStream bool
		ExpectLen       int64
		ExpectErr       string
	}{
		// Cases
		"bytes.Reader": {
			Stream:          bytes.NewReader(make([]byte, 10)),
			ExpectLen:       10,
			ExpectNilStream: false,
		},
		"bytes.Buffer": {
			Stream:          bytes.NewBuffer(make([]byte, 10)),
			ExpectLen:       10,
			ExpectNilStream: false,
		},
		"strings.Reader": {
			Stream:          strings.NewReader("hello"),
			ExpectLen:       5,
			ExpectNilStream: false,
		},
		"empty stream": {
			Stream:          strings.NewReader(""),
			ExpectLen:       0,
			ExpectNilStream: false,
		},
		"empty stream bytes": {
			Stream:          bytes.NewReader([]byte{}),
			ExpectLen:       0,
			ExpectNilStream: false,
		},
		"nil stream": {
			ExpectLen:       0,
			ExpectNilStream: true,
		},
		"un-seekable and no length": {
			Stream:          &basicReader{buf: make([]byte, 10)},
			ExpectLen:       -1,
			ExpectNilStream: false,
		},
		"with error": {
			Stream:          &errorSecondSeekableReader{err: fmt.Errorf("seek failed")},
			ExpectErr:       "seek failed",
			ExpectLen:       -1,
			ExpectNilStream: false,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := NewStackRequest().(*Request)
			req, err = req.SetStream(c.Stream)
			if err != nil {
				t.Fatalf("expect to set stream, %v", err)
			}

			var updatedRequest *Request
			var m ComputeContentLength
			_, _, err = m.HandleBuild(context.Background(),
				middleware.BuildInput{Request: req},
				middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (
					out middleware.BuildOutput, metadata middleware.Metadata, err error) {
					updatedRequest = input.Request.(*Request)
					return out, metadata, nil
				}),
			)
			if len(c.ExpectErr) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.ExpectErr, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %q, got %v", e, a)
				}
				return
			} else if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if e, a := c.ExpectLen, updatedRequest.ContentLength; e != a {
				t.Errorf("expect %v content-length, got %v", e, a)
			}

			if e, a := c.ExpectNilStream, updatedRequest.stream == nil; e != a {
				t.Errorf("expect %v nil stream, got %v", e, a)
			}
		})
	}
}

func TestContentLengthMiddleware_HeaderSet(t *testing.T) {
	req := NewStackRequest().(*Request)
	req.Header.Set("Content-Length", "1234")

	var err error
	req, err = req.SetStream(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("expect to set stream, %v", err)
	}

	var m ComputeContentLength
	_, _, err = m.HandleBuild(context.Background(),
		middleware.BuildInput{Request: req},
		nopBuildHandler,
	)
	if err != nil {
		t.Fatalf("expect middleware to run, %v", err)
	}

	if e, a := "1234", req.Header.Get("Content-Length"); e != a {
		t.Errorf("expect Content-Length not to change, got %v", a)
	}
}

var nopBuildHandler = middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	return out, metadata, nil
})

type basicReader struct {
	buf []byte
}

func (r *basicReader) Read(p []byte) (int, error) {
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

type errorSecondSeekableReader struct {
	err   error
	count int
}

func (r *errorSecondSeekableReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}
func (r *errorSecondSeekableReader) Seek(offset int64, whence int) (int64, error) {
	r.count++
	if r.count == 2 {
		return 0, r.err
	}
	return 0, nil
}

func TestValidateContentLengthHeader(t *testing.T) {
	cases := map[string]struct {
		contentLength int64
		expectError   string
	}{
		"success": {
			contentLength: 10,
		},
		"length set to 0": {
			contentLength: 0,
		},
		"content-length unset": {
			contentLength: -1,
			expectError:   "content length for payload is required and must be at least 0",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := NewStackRequest().(*Request)
			req.ContentLength = c.contentLength

			var m validateContentLength
			_, _, err = m.HandleBuild(context.Background(),
				middleware.BuildInput{Request: req},
				nopBuildHandler,
			)

			if len(c.expectError) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.expectError, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %q, got %v", e, a)
				}
			} else if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
		})
	}
}
