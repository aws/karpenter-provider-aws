package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type testReader struct {
	duration time.Duration
	count    int
}

func (r *testReader) Read(b []byte) (int, error) {
	if r.count > 0 {
		r.count--
		return len(b), nil
	}
	time.Sleep(r.duration)
	return 0, io.EOF
}

func (r *testReader) Close() error {
	return nil
}

func TestTimeoutReadCloser(t *testing.T) {
	reader := timeoutReadCloser{
		reader: &testReader{
			duration: time.Second,
			count:    5,
		},
		duration: time.Millisecond,
	}
	b := make([]byte, 100)
	_, err := reader.Read(b)
	if err != nil {
		t.Log(err)
	}
}

func TestTimeoutReadCloserSameDuration(t *testing.T) {
	reader := timeoutReadCloser{
		reader: &testReader{
			duration: time.Millisecond,
			count:    5,
		},
		duration: time.Millisecond,
	}
	b := make([]byte, 100)
	_, err := reader.Read(b)
	if err != nil {
		t.Log(err)
	}
}

func TestResponseReadTimeoutMiddleware(t *testing.T) {
	reader := testReader{
		duration: time.Second,
		count:    0,
	}

	m := &readTimeout{duration: time.Millisecond}

	out, _, err := m.HandleDeserialize(context.Background(),
		middleware.DeserializeInput{},
		middleware.DeserializeHandlerFunc(
			func(ctx context.Context, input middleware.DeserializeInput) (
				out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
			) {
				response := &smithyhttp.Response{
					Response: &http.Response{Body: &reader},
				}
				return middleware.DeserializeOutput{RawResponse: response}, metadata, err
			}),
	)

	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	response, ok := out.RawResponse.(*smithyhttp.Response)
	if !ok {
		t.Errorf("expect smithy response, got %T", out.RawResponse)
	}

	b := make([]byte, 100)
	_, err = response.Body.Read(b)
	if err != nil {
		readTimeoutError := &ResponseTimeoutError{}
		if !errors.As(err, &readTimeoutError) {
			t.Errorf("expect timeout error, got %v", err)
		}
	} else {
		t.Error("expect timeout error, got none")
	}
}
