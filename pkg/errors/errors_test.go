/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors_test

import (
	"fmt"
	"net/http"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/karpenter-provider-aws/pkg/errors"
)

// withStatus wraps apiErr the way the AWS SDK does, so the HTTP status code of the response is
// reachable from the returned error.
func withStatus(statusCode int, apiErr error) error {
	return &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: "RunInstances",
		Err: &awshttp.ResponseError{
			ResponseError: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{Response: &http.Response{StatusCode: statusCode}},
				Err:      apiErr,
			},
		},
	}
}

func TestIsServerError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "non-AWS error", err: fmt.Errorf("connection reset by peer"), want: false},
		{name: "server fault", err: &smithy.GenericAPIError{Code: "InternalError", Fault: smithy.FaultServer}, want: true},
		// EC2's deserializers leave Fault unset, so these are only distinguishable by status code.
		{name: "faultless 503", err: withStatus(503, &smithy.GenericAPIError{Code: "Unavailable"}), want: true},
		{name: "faultless 500", err: withStatus(500, &smithy.GenericAPIError{Code: "InternalError"}), want: true},
		{name: "faultless 400", err: withStatus(400, &smithy.GenericAPIError{Code: "InvalidBlockDeviceMapping"}), want: false},
		// 501 is a 5xx the SDK does not consider retryable.
		{name: "faultless 501", err: withStatus(501, &smithy.GenericAPIError{Code: "NotImplemented"}), want: false},
		{name: "unwrapped API error", err: &smithy.GenericAPIError{Code: "InvalidBlockDeviceMapping"}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := errors.IsServerError(tc.err); got != tc.want {
				t.Errorf("IsServerError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsNonTerminalError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "non-AWS error", err: fmt.Errorf("connection reset by peer"), want: false},
		{name: "throttle code from the SDK set", err: &smithy.GenericAPIError{Code: "EC2ThrottledException"}, want: true},
		{name: "retryable code from the SDK set", err: &smithy.GenericAPIError{Code: "RequestTimeout"}, want: true},
		{name: "credential blip", err: &smithy.GenericAPIError{Code: "AuthFailure"}, want: true},
		{name: "garbage collected launch template", err: &smithy.GenericAPIError{Code: "InvalidLaunchTemplateId.NotFound"}, want: true},
		{name: "request rejected on its contents", err: &smithy.GenericAPIError{Code: "InvalidBlockDeviceMapping"}, want: false},
		{name: "unauthorized", err: &smithy.GenericAPIError{Code: "UnauthorizedOperation"}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := errors.IsNonTerminalError(tc.err); got != tc.want {
				t.Errorf("IsNonTerminalError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestToAPIErrorMessage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		want   string
		wantOK bool
	}{
		{name: "nil", err: nil, want: "", wantOK: false},
		{name: "non-AWS error", err: fmt.Errorf("connection reset by peer"), want: "", wantOK: false},
		{
			name:   "code and message",
			err:    &smithy.GenericAPIError{Code: "InvalidBlockDeviceMapping", Message: "Volume of size 2GB is smaller than snapshot"},
			want:   "InvalidBlockDeviceMapping: Volume of size 2GB is smaller than snapshot",
			wantOK: true,
		},
		{name: "code only", err: &smithy.GenericAPIError{Code: "InvalidBlockDeviceMapping"}, want: "InvalidBlockDeviceMapping", wantOK: true},
		{
			name:   "wrapped by the SDK transport types",
			err:    withStatus(400, &smithy.GenericAPIError{Code: "InvalidParameterValue", Message: "Invalid value"}),
			want:   "InvalidParameterValue: Invalid value",
			wantOK: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := errors.ToAPIErrorMessage(tc.err)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ToAPIErrorMessage() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
