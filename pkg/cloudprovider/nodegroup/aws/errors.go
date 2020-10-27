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

package aws

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
)

type AWSTransientError struct {
	err error
}

func NewTransientError(err error) error {
	if err != nil {
		return AWSTransientError{
			err: err,
		}
	}
	return nil
}

func (e AWSTransientError) Unwrap() error {
	return e.err
}

func (e AWSTransientError) Error() string {
	return e.err.Error()
}

func (e AWSTransientError) IsRetryable() bool {
	return request.IsErrorRetryable(e.err)
}

func (e AWSTransientError) Message() string {
	var err awserr.Error
	if errors.As(e.err, &err) {
		return err.Code()
	}
	return ""
}
