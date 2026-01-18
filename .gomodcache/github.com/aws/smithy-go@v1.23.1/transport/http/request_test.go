package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRequestRewindable(t *testing.T) {
	cases := map[string]struct {
		Stream    io.Reader
		ExpectErr string
	}{
		"rewindable": {
			Stream: bytes.NewReader([]byte{}),
		},
		"empty not rewindable": {
			Stream: bytes.NewBuffer([]byte{}),
			// ExpectErr: "stream is not seekable",
		},
		"not empty not rewindable": {
			Stream:    bytes.NewBuffer([]byte("abc123")),
			ExpectErr: "stream is not seekable",
		},
		"nil stream": {},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			req := NewStackRequest().(*Request)

			req, err := req.SetStream(c.Stream)
			if err != nil {
				t.Fatalf("expect no error setting stream, %v", err)
			}

			err = req.RewindStream()
			if len(c.ExpectErr) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.ExpectErr, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %v, got %v", e, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
		})
	}
}

func TestRequestBuild_contentLength(t *testing.T) {
	cases := []struct {
		Request  *Request
		Expected int64
	}{
		{
			Request: &Request{
				Request: &http.Request{
					ContentLength: 100,
				},
			},
			Expected: 100,
		},
		{
			Request: &Request{
				Request: &http.Request{
					ContentLength: -1,
				},
			},
			Expected: 0,
		},
		{
			Request: &Request{
				Request: &http.Request{
					ContentLength: 100,
				},
				stream: bytes.NewReader(make([]byte, 100)),
			},
			Expected: 100,
		},
		{
			Request: &Request{
				Request: &http.Request{
					ContentLength: 100,
				},
				stream: http.NoBody,
			},
			Expected: 100,
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			build := tt.Request.Build(context.Background())

			if build.ContentLength != tt.Expected {
				t.Errorf("expect %v, got %v", tt.Expected, build.ContentLength)
			}
		})
	}
}

func TestRequestSetStream(t *testing.T) {
	cases := map[string]struct {
		reader                 io.Reader
		expectSeekable         bool
		expectStreamStartPos   int64
		expectContentLength    int64
		expectNilStream        bool
		expectNilBody          bool
		expectReqContentLength int64
	}{
		"nil stream": {
			expectNilStream: true,
			expectNilBody:   true,
		},
		"empty unseekable stream": {
			reader:          bytes.NewBuffer([]byte{}),
			expectNilStream: true,
			expectNilBody:   true,
		},
		"empty seekable stream": {
			reader:              bytes.NewReader([]byte{}),
			expectContentLength: 0,
			expectSeekable:      true,
			expectNilStream:     false,
			expectNilBody:       true,
		},
		"unseekable no len stream": {
			reader:                 io.NopCloser(bytes.NewBuffer([]byte("abc123"))),
			expectContentLength:    -1,
			expectNilStream:        false,
			expectNilBody:          false,
			expectReqContentLength: -1,
		},
		"unseekable stream": {
			reader:                 bytes.NewBuffer([]byte("abc123")),
			expectContentLength:    6,
			expectNilStream:        false,
			expectNilBody:          false,
			expectReqContentLength: 6,
		},
		"seekable stream": {
			reader:                 bytes.NewReader([]byte("abc123")),
			expectContentLength:    6,
			expectNilStream:        false,
			expectSeekable:         true,
			expectNilBody:          false,
			expectReqContentLength: 6,
		},
		"offset seekable stream": {
			reader: func() io.Reader {
				r := bytes.NewReader([]byte("abc123"))
				_, _ = r.Seek(1, os.SEEK_SET)
				return r
			}(),
			expectStreamStartPos:   1,
			expectContentLength:    5,
			expectSeekable:         true,
			expectNilStream:        false,
			expectNilBody:          false,
			expectReqContentLength: 5,
		},
		"NoBody stream": {
			reader:          http.NoBody,
			expectNilStream: true,
			expectNilBody:   true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := NewStackRequest().(*Request)
			req, err = req.SetStream(c.reader)
			if err != nil {
				t.Fatalf("expect not error, got %v", err)
			}

			if e, a := c.expectSeekable, req.IsStreamSeekable(); e != a {
				t.Errorf("expect %v seekable, got %v", e, a)
			}
			if e, a := c.expectStreamStartPos, req.streamStartPos; e != a {
				t.Errorf("expect %v seek start position, got %v", e, a)
			}
			if e, a := c.expectNilStream, req.stream == nil; e != a {
				t.Errorf("expect %v nil stream, got %v", e, a)
			}

			if l, ok, err := req.StreamLength(); err != nil {
				t.Fatalf("expect no stream length error, got %v", err)
			} else if ok {
				req.ContentLength = l
			}

			if e, a := c.expectContentLength, req.ContentLength; e != a {
				t.Errorf("expect %v content-length, got %v", e, a)
			}
			if e, a := c.expectStreamStartPos, req.streamStartPos; e != a {
				t.Errorf("expect %v streamStartPos, got %v", e, a)
			}

			r := req.Build(context.Background())
			if e, a := c.expectNilBody, r.Body == nil; e != a {
				t.Errorf("expect %v request nil body, got %v", e, a)
			}
			if e, a := c.expectContentLength, req.ContentLength; e != a {
				t.Errorf("expect %v request content-length, got %v", e, a)
			}
		})
	}
}
