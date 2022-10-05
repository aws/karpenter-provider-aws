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

package apis

import (
	"github.com/cenkalti/backoff/v4"
	"k8s.io/apimachinery/pkg/types"
)

type EventType string

const (
	CreateEvent EventType = "Create"
	DeleteEvent EventType = "Delete"
)

func NoOp() error { return nil }

type Event struct {
	Source         string
	Type           EventType
	InvolvedObject types.NamespacedName
	Completed      bool
	OnComplete     func() error
	Backoff        *backoff.ExponentialBackOff
}

func (e Event) WithBackoff(b *backoff.ExponentialBackOff) Event {
	e.Backoff = b
	return e
}

type ConvertibleToEvent interface {
	ToEvent() Event
}
