package http

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	smithyio "github.com/aws/smithy-go/io"
	"github.com/aws/smithy-go/middleware"
)

func TestChecksumMiddleware(t *testing.T) {
	cases := map[string]struct {
		payload               io.Reader
		expectedPayloadLength int64
		expectedMD5Checksum   string
		expectError           string
	}{
		"empty body": {
			payload: smithyio.ReadSeekNopCloser{
				ReadSeeker: bytes.NewReader([]byte(``)),
			},
			expectedPayloadLength: 0,
			expectedMD5Checksum:   "1B2M2Y8AsgTpgAmY7PhCfg==",
		},
		"standard req body": {
			payload: smithyio.ReadSeekNopCloser{
				ReadSeeker: bytes.NewReader([]byte(`abc`)),
			},
			expectedPayloadLength: 3,
			expectedMD5Checksum:   "kAFQmDzST7DWlj99KOF/cg==",
		},
		"nil body": {},
		"unseekable payload": {
			payload:     bytes.NewBuffer([]byte(`xyz`)),
			expectError: "unseekable stream is not supported",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := NewStackRequest().(*Request)

			req, err = req.SetStream(c.payload)
			if err != nil {
				t.Fatalf("error setting request stream")
			}
			m := contentMD5Checksum{}
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
				return
			} else if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if e, a := c.expectedMD5Checksum, req.Header.Get(contentMD5Header); e != a {
				t.Errorf("expect md5 checksum : %v, got %v", e, a)
			}

			size, ok, err := req.StreamLength()
			if err != nil {
				t.Fatalf("error fetching request stream length")
			}
			if !ok {
				t.Fatalf("request stream is not seekable")
			}
			if e, a := c.expectedPayloadLength, size; e != a {
				t.Fatalf("expected request stream content length to be %v, got length %v", e, a)
			}
		})
	}
}
