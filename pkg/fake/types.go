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
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type MockedFunction[I any, O any] struct {
	Output          AtomicPtr[O] // Output to return on call to this function
	MultiOut        AtomicPtrSlice[O]
	OutputPages     AtomicPtrSlice[O]
	CalledWithInput AtomicPtrSlice[I] // Slice used to keep track of passed input to this function
	Error           AtomicError       // Error to return a certain number of times defined by custom error options

	pageMapping     sync.Map     // token uuid -> page number: Internal construct to keep track of the page that we are on
	successfulCalls atomic.Int32 // Internal construct to keep track of the number of times this function has successfully been called
	failedCalls     atomic.Int32 // Internal construct to keep track of the number of times this function has failed (with error)
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (m *MockedFunction[I, O]) Reset() {
	m.Output.Reset()
	m.MultiOut.Reset()
	m.OutputPages.Reset()
	m.CalledWithInput.Reset()
	m.Error.Reset()

	m.successfulCalls.Store(0)
	m.failedCalls.Store(0)
	m.pageMapping.Clear()
}

func (m *MockedFunction[I, O]) Invoke(input *I, defaultTransformer func(*I) (*O, error)) (*O, error) {
	err := m.Error.Get()
	if err != nil {
		m.failedCalls.Add(1)
		return nil, err
	}

	m.CalledWithInput.Add(input)

	if !m.Output.IsNil() {
		m.successfulCalls.Add(1)
		return m.Output.Clone(), nil
	}

	if m.MultiOut.Len() > 0 {
		m.successfulCalls.Add(1)
		return m.MultiOut.Pop(), nil
	}
	// This output pages multi-threaded handling isn't perfect
	// It will fail if pages are asynchronously requested from the same NextToken
	if m.OutputPages.Len() > 0 {
		token := uuid.New().String() // generate a token so that each paginated request set gets its own mapping
		if !reflect.ValueOf(input).Elem().FieldByName("NextToken").Elem().CanSet() {
			m.pageMapping.Store(token, 0)
		} else {
			token = reflect.ValueOf(input).Elem().FieldByName("NextToken").Elem().String()
		}
		pageNum := lo.Must(m.pageMapping.Load(token)).(int)
		page := m.OutputPages.At(pageNum)
		if pageNum < m.OutputPages.Len()-1 {
			reflect.ValueOf(page).Elem().FieldByName("NextToken").Set(reflect.ValueOf(lo.ToPtr(token)))
		}
		m.pageMapping.Store(token, pageNum+1)
		return page, nil
	}
	out, err := defaultTransformer(input)
	if err != nil {
		m.failedCalls.Add(1)
	} else {
		m.successfulCalls.Add(1)
	}
	return out, err
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
