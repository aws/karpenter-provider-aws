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

package fake

// fakeError implements controllers.RetryableError & controllers.CodedError
type fakeRetryableError struct {
	error
	retryable bool
}

func (e *fakeRetryableError) IsRetryable() bool {
	return e.retryable
}
func (e *fakeRetryableError) ErrorCode() string {
	return e.Error()
}

func RetryableError(err error) *fakeRetryableError {
	return &fakeRetryableError{error: err, retryable: true}
}
