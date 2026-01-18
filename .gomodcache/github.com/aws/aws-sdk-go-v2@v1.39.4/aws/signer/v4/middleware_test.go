package v4

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

func TestComputePayloadHashMiddleware(t *testing.T) {
	cases := []struct {
		content      io.Reader
		expectedHash string
		expectedErr  interface{}
	}{
		0: {
			content: func() io.Reader {
				br := bytes.NewReader([]byte("some content"))
				return br
			}(),
			expectedHash: "290f493c44f5d63d06b374d0a5abd292fae38b92cab2fae5efefe1b0e9347f56",
		},
		1: {
			content: func() io.Reader {
				return &nonSeeker{}
			}(),
			expectedErr: &HashComputationError{},
		},
		2: {
			content: func() io.Reader {
				return &semiSeekable{}
			}(),
			expectedErr: &HashComputationError{},
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			c := &ComputePayloadSHA256{}

			next := middleware.FinalizeHandlerFunc(func(ctx context.Context, in middleware.FinalizeInput) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
				value := GetPayloadHash(ctx)
				if len(value) == 0 {
					t.Fatalf("expected payload hash value to be on context")
				}
				if e, a := tt.expectedHash, value; e != a {
					t.Errorf("expected %v, got %v", e, a)
				}

				return out, metadata, err
			})

			stream, err := smithyhttp.NewStackRequest().(*smithyhttp.Request).SetStream(tt.content)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			_, _, err = c.HandleFinalize(context.Background(), middleware.FinalizeInput{Request: stream}, next)
			if err != nil && tt.expectedErr == nil {
				t.Errorf("expected no error, got %v", err)
			} else if err != nil && tt.expectedErr != nil {
				e, a := tt.expectedErr, err
				if !errors.As(a, &e) {
					t.Errorf("expected error type %T, got %T", e, a)
				}
			} else if err == nil && tt.expectedErr != nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

type httpSignerFunc func(ctx context.Context, credentials aws.Credentials, r *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*SignerOptions)) error

func (f httpSignerFunc) SignHTTP(ctx context.Context, credentials aws.Credentials, r *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*SignerOptions)) error {
	return f(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}

func TestSignHTTPRequestMiddleware(t *testing.T) {
	cases := map[string]struct {
		creds       aws.CredentialsProvider
		hash        string
		logSigning  bool
		expectedErr interface{}
	}{
		"success": {
			creds: unit.StubCredentialsProvider{},
			hash:  "0123456789abcdef",
		},
		"error": {
			creds:       unit.StubCredentialsProvider{},
			hash:        "",
			expectedErr: &SigningError{},
		},
		"anonymous creds": {
			creds: aws.AnonymousCredentials{},
		},
		"nil creds": {
			creds: nil,
		},
		"with log signing": {
			creds:      unit.StubCredentialsProvider{},
			hash:       "0123456789abcdef",
			logSigning: true,
		},
	}

	const (
		signingName   = "serviceId"
		signingRegion = "regionName"
	)

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			c := &SignHTTPRequestMiddleware{
				credentialsProvider: tt.creds,
				signer: httpSignerFunc(
					func(ctx context.Context,
						credentials aws.Credentials, r *http.Request, payloadHash string,
						service string, region string, signingTime time.Time,
						optFns ...func(*SignerOptions),
					) error {
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

						expectCreds, _ := unit.StubCredentialsProvider{}.Retrieve(context.Background())
						if e, a := expectCreds, credentials; e != a {
							t.Errorf("expected %v, got %v", e, a)
						}
						if e, a := tt.hash, payloadHash; e != a {
							t.Errorf("expected %v, got %v", e, a)
						}
						if e, a := signingName, service; e != a {
							t.Errorf("expected %v, got %v", e, a)
						}
						if e, a := signingRegion, region; e != a {
							t.Errorf("expected %v, got %v", e, a)
						}
						return nil
					}),
				logSigning: tt.logSigning,
			}

			next := middleware.FinalizeHandlerFunc(func(ctx context.Context, in middleware.FinalizeInput) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
				return out, metadata, err
			})

			ctx := awsmiddleware.SetSigningRegion(
				awsmiddleware.SetSigningName(context.Background(), signingName),
				signingRegion)

			var loggerBuf bytes.Buffer
			logger := logging.NewStandardLogger(&loggerBuf)
			ctx = middleware.SetLogger(ctx, logger)

			if len(tt.hash) != 0 {
				ctx = SetPayloadHash(ctx, tt.hash)
			}

			_, _, err := c.HandleFinalize(ctx, middleware.FinalizeInput{
				Request: &smithyhttp.Request{Request: &http.Request{}},
			}, next)
			if err != nil && tt.expectedErr == nil {
				t.Errorf("expected no error, got %v", err)
			} else if err != nil && tt.expectedErr != nil {
				e, a := tt.expectedErr, err
				if !errors.As(a, &e) {
					t.Errorf("expected error type %T, got %T", e, a)
				}
			} else if err == nil && tt.expectedErr != nil {
				t.Errorf("expected error, got nil")
			}

			if tt.logSigning {
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

func TestSwapComputePayloadSHA256ForUnsignedPayloadMiddleware(t *testing.T) {
	cases := map[string]struct {
		InitStep  func(*middleware.Stack) error
		Mutator   func(*middleware.Stack) error
		ExpectErr string
		ExpectIDs []string
	}{
		"swap in place": {
			InitStep: func(s *middleware.Stack) (err error) {
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("before", nil), middleware.After)
				if err != nil {
					return err
				}
				err = AddComputePayloadSHA256Middleware(s)
				if err != nil {
					return err
				}
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("after", nil), middleware.After)
				if err != nil {
					return err
				}
				return nil
			},
			Mutator: SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
			ExpectIDs: []string{
				"ResolveEndpointV2",
				computePayloadHashMiddlewareID, // should snap to after resolve endpoint
				"before",
				"after",
			},
		},

		"already unsigned payload exists": {
			InitStep: func(s *middleware.Stack) (err error) {
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("before", nil), middleware.After)
				if err != nil {
					return err
				}
				err = AddUnsignedPayloadMiddleware(s)
				if err != nil {
					return err
				}
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("after", nil), middleware.After)
				if err != nil {
					return err
				}
				return nil
			},
			Mutator: SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
			ExpectIDs: []string{
				"ResolveEndpointV2",
				computePayloadHashMiddlewareID,
				"before",
				"after",
			},
		},

		"no compute payload": {
			InitStep: func(s *middleware.Stack) (err error) {
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("before", nil), middleware.After)
				if err != nil {
					return err
				}
				err = s.Finalize.Add(middleware.FinalizeMiddlewareFunc("after", nil), middleware.After)
				if err != nil {
					return err
				}
				return nil
			},
			Mutator:   SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
			ExpectErr: "not found, " + computePayloadHashMiddlewareID,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stack := middleware.NewStack(t.Name(), smithyhttp.NewStackRequest)
			stack.Finalize.Add(&nopResolveEndpoint{}, middleware.After)
			if err := c.InitStep(stack); err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			err := c.Mutator(stack)
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

			if diff := cmpDiff(c.ExpectIDs, stack.Finalize.List()); len(diff) != 0 {
				t.Errorf("expect match\n%v", diff)
			}
		})
	}
}

func TestUseDynamicPayloadSigningMiddleware(t *testing.T) {
	cases := map[string]struct {
		content      io.Reader
		url          string
		expectedHash string
		expectedErr  interface{}
	}{
		"TLS disabled": {
			content: func() io.Reader {
				br := bytes.NewReader([]byte("some content"))
				return br
			}(),
			url:          "http://localhost.com/",
			expectedHash: "290f493c44f5d63d06b374d0a5abd292fae38b92cab2fae5efefe1b0e9347f56",
		},
		"TLS enabled": {
			content: func() io.Reader {
				br := bytes.NewReader([]byte("some content"))
				return br
			}(),
			url:          "https://localhost.com/",
			expectedHash: "UNSIGNED-PAYLOAD",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			c := &dynamicPayloadSigningMiddleware{}

			next := middleware.FinalizeHandlerFunc(func(ctx context.Context, in middleware.FinalizeInput) (out middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
				value := GetPayloadHash(ctx)
				if len(value) == 0 {
					t.Fatalf("expected payload hash value to be on context")
				}
				if e, a := tt.expectedHash, value; e != a {
					t.Errorf("expected %v, got %v", e, a)
				}

				return out, metadata, err
			})

			req := smithyhttp.NewStackRequest().(*smithyhttp.Request)
			req.URL, _ = url.Parse(tt.url)
			stream, err := req.SetStream(tt.content)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			_, _, err = c.HandleFinalize(context.Background(), middleware.FinalizeInput{Request: stream}, next)
			if err != nil && tt.expectedErr == nil {
				t.Errorf("expected no error, got %v", err)
			} else if err != nil && tt.expectedErr != nil {
				e, a := tt.expectedErr, err
				if !errors.As(a, &e) {
					t.Errorf("expected error type %T, got %T", e, a)
				}
			} else if err == nil && tt.expectedErr != nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestGetSignedRequestSignature(t *testing.T) {
	testCases := map[string]struct {
		authHeader     string
		expectedSig    string
		expectedErrMsg string
	}{
		"Valid signature": {
			authHeader:  "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expectedSig: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
		},
		"Whitespace after Signature": {
			authHeader:  "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024 ",
			expectedSig: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
		},
		"Whitespaces before Signature": {
			authHeader:  "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date,     Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024 ",
			expectedSig: "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
		},
		"Empty signature": {
			authHeader:     "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=",
			expectedErrMsg: "invalid request signature authorization header",
		},
		"Missing signature": {
			authHeader:     "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date",
			expectedErrMsg: "request not signed",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			r, err := http.NewRequest("GET", "/", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			r.Header.Set("Authorization", tc.authHeader)

			sig, err := GetSignedRequestSignature(r)

			if tc.expectedErrMsg != "" {
				if err == nil {
					t.Errorf("Expected error with message '%s', but got no error", tc.expectedErrMsg)
				} else if err.Error() != tc.expectedErrMsg {
					t.Errorf("Expected error message '%s', but got '%s'", tc.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if hex.EncodeToString(sig) != tc.expectedSig {
					t.Errorf("Expected signature '%s', but got '%s'", tc.expectedSig, hex.EncodeToString(sig))
				}
			}
		})
	}
}

type nonSeeker struct{}

func (nonSeeker) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type semiSeekable struct {
	hasSeeked bool
}

func (s *semiSeekable) Seek(offset int64, whence int) (int64, error) {
	if !s.hasSeeked {
		s.hasSeeked = true
		return 0, nil
	}
	return 0, fmt.Errorf("io seek error")
}

func (*semiSeekable) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type nopResolveEndpoint struct{}

func (*nopResolveEndpoint) ID() string { return "ResolveEndpointV2" }

func (*nopResolveEndpoint) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
) {
	return out, metadata, err
}

var (
	_ middleware.FinalizeMiddleware = &UnsignedPayload{}
	_ middleware.FinalizeMiddleware = &ComputePayloadSHA256{}
	_ middleware.FinalizeMiddleware = &ContentSHA256Header{}
	_ middleware.FinalizeMiddleware = &SignHTTPRequestMiddleware{}
)
