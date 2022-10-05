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

package channel

// MustClose closes a channel if it isn't already closed
// This avoids the case where closing an already closed channel will panic
func MustClose[T any](ch chan T) {
	if !IsClosed(ch) {
		close(ch)
	}
}

func IsClosed[T any](ch <-chan T) bool {
	select {
	case <-ch:
		return true
	default:
	}
	return false
}
