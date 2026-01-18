package bearer

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"

	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestSignHTTPSMessage(t *testing.T) {
	cases := map[string]struct {
		message       Message
		token         Token
		expectMessage Message
		expectErr     string
	}{
		// Cases
		"not smithyhttp.Request": {
			message:   struct{}{},
			expectErr: "expect smithy-go HTTP Request",
		},
		"not https": {
			message: func() Message {
				r := smithyhttp.NewStackRequest().(*smithyhttp.Request)
				r.URL, _ = url.Parse("http://example.aws")
				return r
			}(),
			expectErr: "requires HTTPS",
		},
		"success": {
			message: func() Message {
				r := smithyhttp.NewStackRequest().(*smithyhttp.Request)
				r.URL, _ = url.Parse("https://example.aws")
				return r
			}(),
			token: Token{Value: "abc123"},
			expectMessage: func() Message {
				r := smithyhttp.NewStackRequest().(*smithyhttp.Request)
				r.URL, _ = url.Parse("https://example.aws")
				r.Header.Set("Authorization", "Bearer abc123")
				return r
			}(),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			signer := SignHTTPSMessage{}
			message, err := signer.SignWithBearerToken(ctx, c.token, c.message)
			if c.expectErr != "" {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.expectErr, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect %v in error %v", e, a)
				}
				return
			} else if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			expect := c.expectMessage.(*smithyhttp.Request)

			actual, ok := message.(*smithyhttp.Request)
			if !ok {
				t.Fatalf("*smithyhttp.Request != %T", actual)
			}
			if !reflect.DeepEqual(expect.Header, actual.Header) {
				t.Errorf("%v != %v", expect.Header, actual.Header)
			}
		})
	}
}
