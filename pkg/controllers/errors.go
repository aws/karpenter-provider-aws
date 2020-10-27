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

// TransientError indicates that an error can possibly be retried.
// Cloud providers can wrap selectively wrap certain errors to
// indicate that they might be retryable.
// TODO: TERRIBLE NAME!!!!!!!
type TransientError interface {
	IsRetryable() bool
	Error() string

	// Very short message explaining the problem
	ConditionMessage() string
}

// IsRetryable is a utility function intended to help controllers and
// other clients determine whether a particular error indicates a
// fundamental problem with a resource (one that cannot be resolved
// without operator intervention) or whether it indicates a problem
// that could simply resolve itself.
func IsRetryable(err error) bool {
	var transient TransientError
	if errors.As(err, &transient) {
		return transient.IsRetryable()
	}
	return false
}

func ConditionMessage(err error) string {
	var transient TransientError
	if errors.As(err, &transient) {
		return transient.ConditionMessage()
	}
	return ""
}
