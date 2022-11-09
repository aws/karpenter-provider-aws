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

import (
	"sync/atomic"
)

type MockedFunction[I any, O any] struct {
	Output          AtomicPtr[O]      // Output to return on call to this function
	CalledWithInput AtomicPtrSlice[I] // Slice used to keep track of passed input to this function
	Error           AtomicError       // Error to return a certain number of times defined by custom error options

	defaultOutput   AtomicPtr[O] // Default output stores the default output if Output isn't set
	successfulCalls atomic.Int32 // Internal construct to keep track of the number of times this function has successfully been called
	failedCalls     atomic.Int32 // Internal construct to keep track of the number of times this function has failed (with error)
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (m *MockedFunction[I, O]) Reset() {
	m.Output.Reset()
	m.CalledWithInput.Reset()
	m.Error.Reset()

	m.defaultOutput.Reset()
	m.successfulCalls.Store(0)
	m.failedCalls.Store(0)
}

func (m *MockedFunction[I, O]) WithDefault(output *O) *MockedFunction[I, O] {
	m.defaultOutput.Set(output)
	return m
}

func (m *MockedFunction[I, O]) Invoke(input *I) (*O, error) {
	err := m.Error.Get()
	if err != nil {
		m.failedCalls.Add(1)
		return nil, err
	}
	m.CalledWithInput.Add(input)
	m.successfulCalls.Add(1)

	if !m.Output.IsNil() {
		return m.Output.Clone(), nil
	}
	if !m.defaultOutput.IsNil() {
		return m.defaultOutput.Clone(), nil
	}
	return new(O), nil
}

func (m *MockedFunction[I, O]) Calls() int {
	return m.SuccessfulCalls() + m.FailedCalls()
}

func (m *MockedFunction[I, O]) SuccessfulCalls() int {
	return int(m.successfulCalls.Load())
}

func (m *MockedFunction[I, O]) FailedCalls() int {
	return int(m.failedCalls.Load())
}
