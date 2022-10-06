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

package atomic

import (
	"container/list"
	"sync"

	"github.com/aws/karpenter/pkg/utils/channel"
)

// List exposes a slice of a type in a race-free manner.
type List[T any] struct {
	mu   sync.RWMutex
	list *list.List

	waiter chan struct{}
}

func NewList[T any]() *List[T] {
	return &List[T]{
		mu:     sync.RWMutex{},
		list:   list.New(),
		waiter: make(chan struct{}),
	}
}

func (l *List[T]) Back() {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.list.Back()
}

func (l *List[T]) Front() {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.list.Front()
}

func (l *List[T]) Init() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.Init()
	l.waiter = make(chan struct{})
}

func (l *List[T]) InsertAfter(v T, mark *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.InsertAfter(v, mark)
	channel.MustClose(l.waiter)
}

func (l *List[T]) InsertBefore(v T, mark *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.InsertBefore(v, mark)
	channel.MustClose(l.waiter)
}

func (l *List[T]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.list.Len()
}

func (l *List[T]) MoveAfter(e, mark *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.MoveAfter(e, mark)
}

func (l *List[T]) MoveBefore(e, mark *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.MoveBefore(e, mark)
}

func (l *List[T]) MoveToBack(e *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.MoveToBack(e)
}

func (l *List[T]) MoveToFront(e *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.MoveToFront(e)
}

// PushBack is modified to not return the element for thread-safety
func (l *List[T]) PushBack(v T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.PushBack(v)
	channel.MustClose(l.waiter)
}

// PushFront is modified to not return the element for thread-safety
func (l *List[T]) PushFront(v T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.PushFront(v)
	channel.MustClose(l.waiter)
}

func (l *List[T]) Remove(e *list.Element) T {
	l.mu.Lock()
	defer l.mu.Unlock()

	// If this is the last element that we are about to remove, we should set the waiter
	if l.list.Len() == 1 {
		l.waiter = make(chan struct{})
	}
	return l.list.Remove(e).(T)
}

// PopFront performs the Remove action on the Front element
func (l *List[T]) PopFront() T {
	l.mu.Lock()
	defer l.mu.Unlock()

	// If this is the last element that we are about to remove, we should set the waiter
	if l.list.Len() == 1 {
		l.waiter = make(chan struct{})
	}
	if l.list.Len() > 0 {
		return l.list.Remove(l.list.Front()).(T)
	}
	return *new(T)
}

// PopBack performs the Remove action on the Back element
func (l *List[T]) PopBack() T {
	l.mu.Lock()
	defer l.mu.Unlock()

	// If this is the last element that we are about to remove, we should set the waiter
	if l.list.Len() == 1 {
		l.waiter = make(chan struct{})
	}
	if l.list.Len() > 0 {
		return l.list.Remove(l.list.Back()).(T)
	}
	return *new(T)
}

// PopAll removes and returns all elements in the list
func (l *List[T]) PopAll() []T {
	l.mu.Lock()
	defer l.mu.Unlock()
	var ret []T

	for l.list.Len() > 0 {
		ret = append(ret, l.list.Remove(l.list.Front()).(T))
	}
	l.waiter = make(chan struct{})
	return ret
}

// WaitForElems closes an event channel when there are elements in the channel and creates
// a new empty channel to wait on when there are no elements
func (l *List[T]) WaitForElems() <-chan struct{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.waiter
}
