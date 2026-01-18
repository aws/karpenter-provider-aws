package requestcompression

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/transport/http"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestRequestCompression(t *testing.T) {
	cases := map[string]struct {
		DisableRequestCompression   bool
		RequestMinCompressSizeBytes int64
		ContentLength               int64
		Header                      map[string][]string
		Stream                      io.Reader
		ExpectedStream              []byte
		ExpectedHeader              map[string][]string
	}{
		"GZip request stream": {
			Stream:         strings.NewReader("Hi, world!"),
			ExpectedStream: []byte("Hi, world!"),
			ExpectedHeader: map[string][]string{
				"Content-Encoding": {"gzip"},
			},
		},
		"GZip request stream with existing encoding header": {
			Stream:         strings.NewReader("Hi, world!"),
			ExpectedStream: []byte("Hi, world!"),
			Header: map[string][]string{
				"Content-Encoding": {"custom"},
			},
			ExpectedHeader: map[string][]string{
				"Content-Encoding": {"custom, gzip"},
			},
		},
		"GZip request stream smaller than min compress request size": {
			RequestMinCompressSizeBytes: 100,
			Stream:                      strings.NewReader("Hi, world!"),
			ExpectedStream:              []byte("Hi, world!"),
			ExpectedHeader:              map[string][]string{},
		},
		"Disable GZip request stream": {
			DisableRequestCompression: true,
			Stream:                    strings.NewReader("Hi, world!"),
			ExpectedStream:            []byte("Hi, world!"),
			ExpectedHeader:            map[string][]string{},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := http.NewStackRequest().(*http.Request)
			req.ContentLength = c.ContentLength
			req, _ = req.SetStream(c.Stream)
			if c.Header != nil {
				req.Header = c.Header
			}
			var updatedRequest *http.Request

			m := requestCompression{
				disableRequestCompression:   c.DisableRequestCompression,
				requestMinCompressSizeBytes: c.RequestMinCompressSizeBytes,
				compressAlgorithms:          []string{GZIP},
			}
			_, _, err = m.HandleSerialize(context.Background(),
				middleware.SerializeInput{Request: req},
				middleware.SerializeHandlerFunc(func(ctx context.Context, input middleware.SerializeInput) (
					out middleware.SerializeOutput, metadata middleware.Metadata, err error) {
					updatedRequest = input.Request.(*http.Request)
					return out, metadata, nil
				}),
			)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if stream := updatedRequest.GetStream(); stream != nil {
				if err := testUnzipContent(stream, c.ExpectedStream, c.DisableRequestCompression, c.RequestMinCompressSizeBytes); err != nil {
					t.Errorf("error while checking request stream: %q", err)
				}
			}

			if e, a := c.ExpectedHeader, map[string][]string(updatedRequest.Header); !reflect.DeepEqual(e, a) {
				t.Errorf("expect request header to be %q, got %q", e, a)
			}
		})
	}
}

func testUnzipContent(content io.Reader, expect []byte, disableRequestCompression bool, requestMinCompressionSizeBytes int64) error {
	if disableRequestCompression || int64(len(expect)) < requestMinCompressionSizeBytes {
		b, err := io.ReadAll(content)
		if err != nil {
			return fmt.Errorf("error while reading request")
		}
		if e, a := expect, b; !bytes.Equal(e, a) {
			return fmt.Errorf("expect content to be %s, got %s", e, a)
		}
	} else {
		r, err := gzip.NewReader(content)
		if err != nil {
			return fmt.Errorf("error while reading request")
		}

		var actualBytes bytes.Buffer
		_, err = actualBytes.ReadFrom(r)
		if err != nil {
			return fmt.Errorf("error while unzipping request payload")
		}

		if e, a := expect, actualBytes.Bytes(); !bytes.Equal(e, a) {
			return fmt.Errorf("expect unzipped content to be %s, got %s", e, a)
		}
	}

	return nil
}
