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

package cloudprovider

import "errors"

// Queue abstracts all provider specific behavior for Queues
type Queue interface {
	// Name returns the name of the queue
	Name() string
	// Length returns the length of the queue
	Length() (int64, error)
	// OldestMessageAge returns the age of the oldest message
	OldestMessageAge() (int64, error)
}

// NodeGroup abstracts all provider specific behavior for NodeGroups.
// It is meant to be used by controllers.
type NodeGroup interface {
	SetReplicas(count int32) error
	GetReplicas() (int32, error)
}

// TransientError indicates that an error can possibly be retried.
// Cloud providers can wrap selectively wrap certain errors to
// indicate that they might be retryable.
type TransientError interface {
	IsRetryable() bool
	Error() string
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
