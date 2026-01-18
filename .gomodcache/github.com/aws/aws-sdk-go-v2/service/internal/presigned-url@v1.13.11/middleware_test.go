package presignedurl

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestPresignMiddleware(t *testing.T) {
	cases := map[string]struct {
		Input *mockURLPresignInput

		ExpectInput *mockURLPresignInput
		ExpectErr   string
	}{
		"no source": {
			Input:       &mockURLPresignInput{},
			ExpectInput: &mockURLPresignInput{},
		},
		"with presigned URL": {
			Input: &mockURLPresignInput{
				SourceRegion: "source-region",
				PresignedURL: "https://example.amazonaws.com/someURL",
			},
			ExpectInput: &mockURLPresignInput{
				SourceRegion: "source-region",
				PresignedURL: "https://example.amazonaws.com/someURL",
			},
		},
		"with source": {
			Input: &mockURLPresignInput{
				SourceRegion: "source-region",
			},
			ExpectInput: &mockURLPresignInput{
				SourceRegion: "source-region",
				PresignedURL: "https://example.source-region.amazonaws.com/?DestinationRegion=mock-region",
			},
		},
		"matching source destination region": {
			Input: &mockURLPresignInput{
				SourceRegion: "mock-region",
			},
			ExpectInput: &mockURLPresignInput{
				SourceRegion: "mock-region",
				PresignedURL: "https://example.mock-region.amazonaws.com/?DestinationRegion=mock-region",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stack := middleware.NewStack(name, smithyhttp.NewStackRequest)

			stack.Initialize.Add(&awsmiddleware.RegisterServiceMetadata{
				Region: "mock-region",
			}, middleware.After)

			stack.Initialize.Add(&presign{options: getURLPresignMiddlewareOptions()}, middleware.After)

			stack.Initialize.Add(middleware.InitializeMiddlewareFunc(name+"_verifyParams",
				func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (
					out middleware.InitializeOutput, metadata middleware.Metadata, err error,
				) {
					input := in.Parameters.(*mockURLPresignInput)
					if diff := cmpDiff(c.ExpectInput, input); len(diff) != 0 {
						t.Errorf("expect input to be updated\n%s", diff)
					}

					return next.HandleInitialize(ctx, in)
				},
			), middleware.After)

			handler := middleware.DecorateHandler(smithyhttp.NewClientHandler(smithyhttp.NopClient{}), stack)
			_, _, err := handler.Handle(context.Background(), c.Input)
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

func getURLPresignMiddlewareOptions() Options {
	return Options{
		Accessor: ParameterAccessor{
			GetPresignedURL: func(c interface{}) (string, bool, error) {
				presignURL := c.(*mockURLPresignInput).PresignedURL
				if len(presignURL) != 0 {
					return presignURL, true, nil
				}
				return "", false, nil
			},
			GetSourceRegion: func(c interface{}) (string, bool, error) {
				srcRegion := c.(*mockURLPresignInput).SourceRegion
				if len(srcRegion) != 0 {
					return srcRegion, true, nil
				}
				return "", false, nil
			},
			CopyInput: func(c interface{}) (interface{}, error) {
				input := *(c.(*mockURLPresignInput))
				return &input, nil
			},
			SetDestinationRegion: func(c interface{}, v string) error {
				c.(*mockURLPresignInput).DestinationRegion = v
				return nil
			},
			SetPresignedURL: func(c interface{}, v string) error {
				c.(*mockURLPresignInput).PresignedURL = v
				return nil
			},
		},
		Presigner: &mockURLPresigner{},
	}
}

type mockURLPresignInput struct {
	SourceRegion      string
	DestinationRegion string
	PresignedURL      string
}

type mockURLPresigner struct{}

func (*mockURLPresigner) PresignURL(ctx context.Context, srcRegion string, params interface{}) (
	req *v4.PresignedHTTPRequest, err error,
) {
	in := params.(*mockURLPresignInput)

	return &v4.PresignedHTTPRequest{
		URL:          "https://example." + srcRegion + ".amazonaws.com/?DestinationRegion=" + in.DestinationRegion,
		Method:       "GET",
		SignedHeader: http.Header{},
	}, nil
}

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
