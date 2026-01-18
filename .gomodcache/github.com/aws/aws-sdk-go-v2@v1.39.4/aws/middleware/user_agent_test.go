package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var expectedAgent = aws.SDKName + "/" + aws.SDKVersion +
	" ua/2.1" +
	" os/" + getNormalizedOSName() +
	" lang/go#" + strings.Map(rules, languageVersion) + // normalize as the user-agent builder will
	" md/GOOS#" + runtime.GOOS +
	" md/GOARCH#" + runtime.GOARCH

func TestRequestUserAgent_HandleBuild(t *testing.T) {
	cases := map[string]struct {
		Env    map[string]string
		In     middleware.BuildInput
		Next   func(*testing.T, middleware.BuildInput) middleware.BuildHandler
		Expect middleware.BuildInput
		Err    bool
	}{
		"adds product information": {
			In: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{}},
			}},
			Expect: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{
					"User-Agent": {expectedAgent},
					//"X-Amz-User-Agent": {expectedSDKAgent},
				}},
			}},
			Next: func(t *testing.T, expect middleware.BuildInput) middleware.BuildHandler {
				return middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
					if diff := cmpDiff(input, expect); len(diff) > 0 {
						t.Error(diff)
					}
					return o, m, err
				})
			},
		},
		"appends to existing": {
			In: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{
					"User-Agent": {"previously set"},
					//"X-Amz-User-Agent": {"previously set"},
				}},
			}},
			Expect: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{
					"User-Agent": {expectedAgent + " previously set"},
					//"X-Amz-User-Agent": {expectedSDKAgent + " previously set"},
				}},
			}},
			Next: func(t *testing.T, expect middleware.BuildInput) middleware.BuildHandler {
				return middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
					if diff := cmpDiff(input, expect); len(diff) > 0 {
						t.Error(diff)
					}
					return o, m, err
				})
			},
		},
		"adds exec-env if present": {
			Env: map[string]string{
				execEnvVar: "TestCase",
			},
			In: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{}},
			}},
			Expect: middleware.BuildInput{Request: &smithyhttp.Request{
				Request: &http.Request{Header: map[string][]string{
					"User-Agent": {expectedAgent + " exec-env/TestCase"},
					//"X-Amz-User-Agent": {expectedSDKAgent + " exec-env/TestCase"},
				}},
			}},
			Next: func(t *testing.T, expect middleware.BuildInput) middleware.BuildHandler {
				return middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
					if diff := cmpDiff(input, expect); len(diff) > 0 {
						t.Error(diff)
					}
					return o, m, err
				})
			},
		},
		"errors for unknown type": {
			In: middleware.BuildInput{Request: struct{}{}},
			Next: func(t *testing.T, input middleware.BuildInput) middleware.BuildHandler {
				return nil
			},
			Err: true,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			restoreEnv := clearEnv()
			defer restoreEnv()
			for k, v := range tt.Env {
				os.Setenv(k, v)
			}

			b := NewRequestUserAgent()
			_, _, err := b.HandleBuild(context.Background(), tt.In, tt.Next(t, tt.Expect))
			if (err != nil) != tt.Err {
				t.Errorf("error %v, want error %v", err, tt.Err)
				return
			}
		})
	}
}

func clearEnv() func() {
	environ := os.Environ()
	os.Clearenv()
	return func() {
		os.Clearenv()
		for _, v := range environ {
			split := strings.SplitN(v, "=", 2)
			key, value := split[0], split[1]
			os.Setenv(key, value)
		}
	}
}

func TestAddUserAgentKey(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		Key    string
		Expect string
	}{
		"Simple key": {
			Key:    "baz",
			Expect: expectedAgent + " baz",
		},
		"Key containing slash": {
			Key:    "baz/ba",
			Expect: expectedAgent + " baz-ba",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewRequestUserAgent()
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			err := stack.Build.Add(b, middleware.After)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			err = AddUserAgentKey(c.Key)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			_, _, err = b.HandleBuild(context.Background(), bi, middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return o, m, err
			}))
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddUserAgentKeyValue(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		Key    string
		Value  string
		Expect string
	}{
		"Simple key value pair": {
			Key:    "foo",
			Value:  "a+b-C$4'5.6",
			Expect: expectedAgent + " foo/a+b-C$4'5.6",
		},
		"Value containing invalid rune": {
			Key:    "foo",
			Value:  "1(2)3",
			Expect: expectedAgent + " foo/1-2-3",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewRequestUserAgent()
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			err := stack.Build.Add(b, middleware.After)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			err = AddUserAgentKeyValue(c.Key, c.Value)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			_, _, err = b.HandleBuild(context.Background(), bi, middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return o, m, err
			}))
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddUserAgentFeature(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		Features []UserAgentFeature
		Expect   string
	}{
		"none": {
			Features: []UserAgentFeature{},
			Expect:   expectedAgent,
		},
		"one": {
			Features: []UserAgentFeature{
				UserAgentFeatureWaiter,
			},
			Expect: expectedAgent + " " + "m/B",
		},
		"two": {
			Features: []UserAgentFeature{
				UserAgentFeatureRetryModeAdaptive, // ensure stable order, and idempotent
				UserAgentFeatureRetryModeAdaptive,
				UserAgentFeatureWaiter,
			},
			Expect: expectedAgent + " " + "m/B,F",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewRequestUserAgent()
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			err := stack.Build.Add(b, middleware.After)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			for _, f := range c.Features {
				b.AddUserAgentFeature(f)
			}

			in := middleware.BuildInput{
				Request: &smithyhttp.Request{
					Request: &http.Request{
						Header: map[string][]string{},
					},
				},
			}
			_, _, err = b.HandleBuild(context.Background(), in, middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return o, m, err
			}))
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := in.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddSDKAgentKey(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		KeyType SDKAgentKeyType
		Key     string
		Expect  string
	}{
		"Additional metadata key type": {
			KeyType: AdditionalMetadata,
			Key:     "baz",
			Expect:  expectedAgent + " md/baz",
		},
		"Config metadata key type": {
			KeyType: ConfigMetadata,
			Key:     "foo",
			Expect:  expectedAgent + " cfg/foo",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewRequestUserAgent()
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			err := stack.Build.Add(b, middleware.After)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			err = AddSDKAgentKey(c.KeyType, c.Key)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			_, _, err = b.HandleBuild(context.Background(), bi, middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return o, m, err
			}))
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddSDKAgentKeyValue(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		KeyType SDKAgentKeyType
		Key     string
		Value   string
		Expect  string
	}{
		"Value containing valid chars": {
			KeyType: AdditionalMetadata,
			Key:     "baz",
			Value:   "a+b-C$4'5.6",
			Expect:  expectedAgent + " md/baz#a+b-C$4'5.6",
		},
		"Value containing invalid chars": {
			KeyType: ConfigMetadata,
			Key:     "foo",
			Value:   "1(2)3",
			Expect:  expectedAgent + " cfg/foo#1-2-3",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			b := NewRequestUserAgent()
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			err := stack.Build.Add(b, middleware.After)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			err = AddSDKAgentKeyValue(c.KeyType, c.Key, c.Value)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			_, _, err = b.HandleBuild(context.Background(), bi, middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return o, m, err
			}))
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: expected %q != actual %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddUserAgentKey_AddToStack(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		Key    string
		Expect string
	}{
		"Simple key": {
			Key:    "baz",
			Expect: expectedAgent + " baz",
		},
		"Key containing slash": {
			Key:    "baz/ba",
			Expect: expectedAgent + " baz-ba",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			stack.Build.Add(middleware.BuildMiddlewareFunc("testInit", func(ctx context.Context, input middleware.BuildInput, handler middleware.BuildHandler) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return handler.HandleBuild(ctx, bi)
			}), middleware.After)
			err := AddUserAgentKey(c.Key)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			_, _, err = middleware.DecorateHandler(middleware.HandlerFunc(func(ctx context.Context, input interface{}) (output interface{}, metadata middleware.Metadata, err error) {
				return output, metadata, err
			}), stack).Handle(context.Background(), nil)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func TestAddUserAgentKeyValue_AddToStack(t *testing.T) {
	restoreEnv := clearEnv()
	defer restoreEnv()

	cases := map[string]struct {
		Key    string
		Value  string
		Expect string
	}{
		"Simple key value pair": {
			Key:    "foo",
			Value:  "a+b-C$4'5.6",
			Expect: expectedAgent + " foo/a+b-C$4'5.6",
		},
		"Value containing invalid rune": {
			Key:    "foo",
			Value:  "1(2)3",
			Expect: expectedAgent + " foo/1-2-3",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stack := middleware.NewStack("testStack", smithyhttp.NewStackRequest)
			bi := middleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: map[string][]string{}}}}
			stack.Build.Add(middleware.BuildMiddlewareFunc("testInit", func(ctx context.Context, input middleware.BuildInput, handler middleware.BuildHandler) (o middleware.BuildOutput, m middleware.Metadata, err error) {
				return handler.HandleBuild(ctx, bi)
			}), middleware.After)
			err := AddUserAgentKeyValue(c.Key, c.Value)(stack)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			_, _, err = middleware.DecorateHandler(middleware.HandlerFunc(func(ctx context.Context, input interface{}) (output interface{}, metadata middleware.Metadata, err error) {
				return output, metadata, err
			}), stack).Handle(context.Background(), nil)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
			ua, ok := bi.Request.(*smithyhttp.Request).Header["User-Agent"]
			if !ok {
				t.Fatalf("expect User-Agent to be present")
			}
			if ua[0] != c.Expect {
				t.Errorf("User-Agent: %q != %q", c.Expect, ua[0])
			}
		})
	}
}

func cmpDiff(e, a interface{}) string {
	if !reflect.DeepEqual(e, a) {
		return fmt.Sprintf("%v != %v", e, a)
	}
	return ""
}
