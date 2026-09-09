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

package errors

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantReason     string
		wantMessage    string
		wantRetryable  bool
	}{
		{
			name:           "nil error",
			err:            nil,
			wantReason:     "",
			wantMessage:    "",
			wantRetryable:  true,
		},
		{
			name:           "non-AWS error",
			err:            errors.New("plain error"),
			wantReason:     "",
			wantMessage:    "",
			wantRetryable:  true,
		},
		{
			name:           "unauthorized operation",
			err:            smithyErrWithCode("UnauthorizedOperation", "Access denied"),
			wantReason:     "Unauthorized",
			wantMessage:    "Access denied",
			wantRetryable:  false,
		},
		{
			name:           "access denied",
			err:            smithyErrWithCode("AccessDenied", "You don't have permission"),
			wantReason:     "Unauthorized",
			wantMessage:    "You don't have permission",
			wantRetryable:  false,
		},
		{
			name:           "request limit exceeded",
			err:            smithyErrWithCode("RequestLimitExceeded", "Too many requests"),
			wantReason:     "RequestLimitExceeded",
			wantMessage:    "Too many requests",
			wantRetryable:  false,
		},
		{
			name:           "spot quota exceeded",
			err:            smithyErrWithCode("MaxSpotInstanceCountExceeded", "Exceeded spot quota"),
			wantReason:     "SpotQuotaExceeded",
			wantMessage:    "A spot instance launch was requested but this would exceed your spot instance quota",
			wantRetryable:  false,
		},
		{
			name:           "fleet quota exceeded",
			err:            smithyErrWithCode("MaxFleetCountExceeded", "Exceeded fleet quota"),
			wantReason:     "FleetQuotaExceeded",
			wantMessage:    "A fleet launch was requested but this would exceed your fleet request quota",
			wantRetryable:  false,
		},
		{
			name:           "VCPU limit exceeded",
			err:            smithyErrWithCode("VcpuLimitExceeded", "Exceeded VCPU limit"),
			wantReason:     "VCPULimitExceeded",
			wantMessage:    "An instance was requested that would exceed your VCPU quota",
			wantRetryable:  false,
		},
		{
			name:           "insufficient IP addresses",
			err:            smithyErrWithCode("InsufficientFreeAddressesInSubnet", "No IPs left"),
			wantReason:     "InsufficientFreeAddressesInSubnet",
			wantMessage:    "There are not enough free IP addresses to launch an instance in this subnet",
			wantRetryable:  false,
		},
		{
			name:           "unknown error",
			err:            smithyErrWithCode("UnknownError", "Unknown message"),
			wantReason:     "",
			wantMessage:    "",
			wantRetryable:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReason, gotMessage, gotRetryable := ClassifyError(tt.err)
			if gotReason != tt.wantReason {
				t.Errorf("ClassifyError() reason = %v, want %v", gotReason, tt.wantReason)
			}
			if gotMessage != tt.wantMessage {
				t.Errorf("ClassifyError() message = %v, want %v", gotMessage, tt.wantMessage)
			}
			if gotRetryable != tt.wantRetryable {
				t.Errorf("ClassifyError() retryable = %v, want %v", gotRetryable, tt.wantRetryable)
			}
		})
	}
}

func smithyErrWithCode(code, message string) smithy.APIError {
	return &smithy.GenericAPIError{
		Code:    code,
		Message: message,
	}
}
