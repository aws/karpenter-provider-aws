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
	"errors"
	"fmt"
	"testing"

	"github.com/aws/smithy-go"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
)

type apiErr struct {
	code    string
	message string
}

func (e *apiErr) Error() string                 { return fmt.Sprintf("%s: %s", e.code, e.message) }
func (e *apiErr) ErrorCode() string             { return e.code }
func (e *apiErr) ErrorMessage() string          { return e.message }
func (e *apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantReason     string
		wantMessage    string
		wantRetryable  bool
	}{
		{
			name:          "nil error is retryable and returns empty classification",
			err:           nil,
			wantReason:    "",
			wantMessage:   "",
			wantRetryable: true,
		},
		{
			name:          "non-AWS error falls through as retryable",
			err:           errors.New("boom"),
			wantReason:    "",
			wantMessage:   "",
			wantRetryable: true,
		},
		{
			name:          "UnauthorizedOperation is terminal with Unauthorized reason",
			err:           &apiErr{code: awserrors.UnauthorizedOperationErrorCode, message: "user is not authorized"},
			wantReason:    "Unauthorized",
			wantMessage:   "user is not authorized",
			wantRetryable: false,
		},
		{
			name:          "AccessDenied is terminal with Unauthorized reason",
			err:           &apiErr{code: awserrors.AccessDeniedErrorCode, message: "explicit deny"},
			wantReason:    "Unauthorized",
			wantMessage:   "explicit deny",
			wantRetryable: false,
		},
		{
			name:          "AccessDeniedException (IAM shape) is terminal with Unauthorized reason",
			err:           &apiErr{code: awserrors.AccessDeniedExceptionErrorCode, message: "iam explicit deny"},
			wantReason:    "Unauthorized",
			wantMessage:   "iam explicit deny",
			wantRetryable: false,
		},
		{
			name:          "AuthFailure is terminal with Unauthorized reason",
			err:           &apiErr{code: awserrors.AuthFailureErrorCode, message: "auth failure"},
			wantReason:    "Unauthorized",
			wantMessage:   "auth failure",
			wantRetryable: false,
		},
		{
			name:          "LimitExceeded is terminal with LimitExceeded reason",
			err:           &apiErr{code: awserrors.LimitExceededErrorCode, message: "instance profile limit reached"},
			wantReason:    "LimitExceeded",
			wantMessage:   "instance profile limit reached",
			wantRetryable: false,
		},
		{
			name:          "RequestLimitExceeded (rate limiting) is retryable",
			err:           &apiErr{code: awserrors.RateLimitingErrorCode, message: "slow down"},
			wantReason:    "",
			wantMessage:   "",
			wantRetryable: true,
		},
		{
			name:          "InvalidParameterValue is retryable (transient/config error, not blanket-terminal)",
			err:           &apiErr{code: awserrors.RunInstancesInvalidParameterValueCode, message: "invalid"},
			wantReason:    "",
			wantMessage:   "",
			wantRetryable: true,
		},
		{
			name:          "wrapped terminal error is still classified as terminal",
			err:           fmt.Errorf("listing subnets: %w", &apiErr{code: awserrors.AccessDeniedErrorCode, message: "wrapped"}),
			wantReason:    "Unauthorized",
			wantMessage:   "wrapped",
			wantRetryable: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason, message, retryable := awserrors.ClassifyError(tc.err)
			if reason != tc.wantReason {
				t.Errorf("reason: got %q, want %q", reason, tc.wantReason)
			}
			if message != tc.wantMessage {
				t.Errorf("message: got %q, want %q", message, tc.wantMessage)
			}
			if retryable != tc.wantRetryable {
				t.Errorf("retryable: got %v, want %v", retryable, tc.wantRetryable)
			}
		})
	}
}
