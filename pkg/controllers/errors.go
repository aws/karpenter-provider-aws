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

package controllers

import "errors"

// RetryableError indicates that an error can possibly be retried.
// Cloud providers can wrap selectively wrap certain errors to
// indicate that they might be retryable.
type RetryableError interface {
	IsRetryable() bool
	Error() string
}

// CodedError can be implemented by error types that have a
// short (1-3 word) summary of their error status. This can be
// used by code in places where longer error messages won't do,
// such as Conditions.
type CodedError interface {
	// Very short message explaining the problem
	ErrorCode() string
}

// IsRetryable is a utility function intended to help controllers and
// other clients determine whether a particular error indicates a
// fundamental problem with a resource (one that cannot be resolved
// without operator intervention) or whether it indicates a problem
// that could simply resolve itself.
func IsRetryable(err error) bool {
	var transient RetryableError
	if errors.As(err, &transient) {
		return transient.IsRetryable()
	}
	return false
}

// ErrorCode returns a short version of the error's message, if there
// is one, otherwise nothing. Useful in places where a long message
// isn't acceptable, and you don't want to just randomly hack off a
// longer error message at some fixed byte offset.
func ErrorCode(err error) string {
	var transient CodedError
	if errors.As(err, &transient) {
		return transient.ErrorCode()
	}
	return ""
}
