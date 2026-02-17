/*
Copyright The Kubernetes Authors.

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

package ringbuffer

type RingBuffer[T any] struct {
	values []T
	head   int
}

func New[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		values: make([]T, 0, capacity),
	}
}

func (b *RingBuffer[T]) Insert(value T) {
	// If buffer is not full, append the new value
	if len(b.values) < cap(b.values) {
		b.values = append(b.values, value)
		return
	}
	// If buffer is full, replace the oldest entry
	b.values[b.head] = value
	b.head = (b.head + 1) % cap(b.values)
}

func (b *RingBuffer[T]) Len() int {
	return len(b.values)
}

func (b *RingBuffer[T]) Reset() {
	b.values = b.values[:0]
	b.head = 0
}

func (b *RingBuffer[T]) Items() []T {
	return b.values
}
