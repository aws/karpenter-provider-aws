package v4

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/internal/awstesting/unit"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type httpPresignerFunc func(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*SignerOptions),
) (url string, signedHeader http.Header, err error)

func (f httpPresignerFunc) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*SignerOptions),
) (
	url string, signedHeader http.Header, err error,
) {
	return f(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}

func TestPresignHTTPRequestMiddleware(t *testing.T) {
	cases := map[string]struct {
		Request      *http.Request
		Creds        aws.CredentialsProvider
		PayloadHash  string
		LogSigning   bool
		ExpectResult *PresignedHTTPRequest
		ExpectErr    string
	}{
		"success": {
			Request: &http.Request{
				URL: func() *url.URL {
					u, _ := url.Parse("https://example.aws/path?query=foo")
					return u
				}(),
				Header: http.Header{},
			},
			Creds:       unit.StubCredentialsProvider{},
			PayloadHash: "0123456789abcdef",
			ExpectResult: &PresignedHTTPRequest{
				URL:          "https://example.aws/path?query=foo",
				SignedHeader: http.Header{},
			},
		},
		"error": {
			Request: func() *http.Request {
				return &http.Request{}
			}(),
			Creds:       unit.StubCredentialsProvider{},
			PayloadHash: "",
			ExpectErr:   "failed to sign request",
		},
		"anonymous creds": {
			Request: &http.Request{
				URL: func() *url.URL {
					u, _ := url.Parse("https://example.aws/path?query=foo")
					return u
				}(),
				Header: http.Header{},
			},
			Creds:       unit.StubCredentialsProvider{},
			PayloadHash: "",
			ExpectErr:   "failed to sign request",
			ExpectResult: &PresignedHTTPRequest{
				URL:          "https://example.aws/path?query=foo",
				SignedHeader: http.Header{},
			},
		},
		"nil creds": {
			Request: &http.Request{
				URL: func() *url.URL {
					u, _ := url.Parse("https://example.aws/path?query=foo")
					return u
				}(),
				Header: http.Header{},
			},
			Creds: nil,
			ExpectResult: &PresignedHTTPRequest{
				URL:          "https://example.aws/path?query=foo",
				SignedHeader: http.Header{},
			},
		},
		"with log signing": {
			Request: &http.Request{
				URL: func() *url.URL {
					u, _ := url.Parse("https://example.aws/path?query=foo")
					return u
				}(),
				Header: http.Header{},
			},
			Creds:       unit.StubCredentialsProvider{},
			PayloadHash: "0123456789abcdef",
			ExpectResult: &PresignedHTTPRequest{
				URL:          "https://example.aws/path?query=foo",
				SignedHeader: http.Header{},
			},

			LogSigning: true,
		},
	}

	const (
		signingName   = "serviceId"
		signingRegion = "regionName"
	)

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := &PresignHTTPRequestMiddleware{
				credentialsProvider: c.Creds,

				presigner: httpPresignerFunc(func(
					ctx context.Context, credentials aws.Credentials, r *http.Request,
					payloadHash string, service string, region string, signingTime time.Time,
					optFns ...func(*SignerOptions),
				) (url string, signedHeader http.Header, err error) {
					var options SignerOptions
					for _, fn := range optFns {
						fn(&options)
					}
					if options.Logger == nil {
						t.Errorf("expect logger, got none")
					}
					if options.LogSigning {
						options.Logger.Logf(logging.Debug, t.Name())
					}

					if !haveCredentialProvider(c.Creds) {
						t.Errorf("expect presigner not to be called for not credentials provider")
					}

					expectCreds, _ := unit.StubCredentialsProvider{}.Retrieve(context.Background())
					if e, a := expectCreds, credentials; e != a {
						t.Errorf("expected %v, got %v", e, a)
					}
					if e, a := c.PayloadHash, payloadHash; e != a {
						t.Errorf("expected %v, got %v", e, a)
					}
					if e, a := signingName, service; e != a {
						t.Errorf("expected %v, got %v", e, a)
					}
					if e, a := signingRegion, region; e != a {
						t.Errorf("expected %v, got %v", e, a)
					}

					return c.ExpectResult.URL, c.ExpectResult.SignedHeader, nil
				}),
				logSigning: c.LogSigning,
			}

			next := middleware.FinalizeHandlerFunc(
				func(ctx context.Context, in middleware.FinalizeInput) (
					out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
				) {
					t.Errorf("expect next handler not to be called")
					return out, metadata, err
				})

			ctx := awsmiddleware.SetSigningRegion(
				awsmiddleware.SetSigningName(context.Background(), signingName),
				signingRegion)

			var loggerBuf bytes.Buffer
			logger := logging.NewStandardLogger(&loggerBuf)
			ctx = middleware.SetLogger(ctx, logger)

			if len(c.PayloadHash) != 0 {
				ctx = SetPayloadHash(ctx, c.PayloadHash)
			}

			result, _, err := m.HandleFinalize(ctx, middleware.FinalizeInput{
				Request: &smithyhttp.Request{
					Request: c.Request,
				},
			}, next)
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

			if diff := cmpDiff(c.ExpectResult, result.Result); len(diff) != 0 {
				t.Errorf("expect result match\n%v", diff)
			}

			if c.LogSigning {
				if e, a := t.Name(), loggerBuf.String(); !strings.Contains(a, e) {
					t.Errorf("expect %v logged in %v", e, a)
				}
			} else {
				if loggerBuf.Len() != 0 {
					t.Errorf("expect no log, got %v", loggerBuf.String())
				}
			}
		})
	}
}

var (
	_ middleware.FinalizeMiddleware = &PresignHTTPRequestMiddleware{}
)
