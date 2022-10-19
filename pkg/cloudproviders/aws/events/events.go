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

package events

import "k8s.io/apimachinery/pkg/runtime"

type Event struct {
	InvolvedObject runtime.Object
	Type           string
	Reason         string
	Message        string
}

type EventTemplate[T runtime.Object] struct {
	Type            string
	Reason          string
	MessageTemplate func(T) string
}

func (et *EventTemplate[T]) For(obj T) Event {
	return Event{
		InvolvedObject: obj,
		Type:           et.Type,
		Reason:         et.Reason,
		Message:        et.MessageTemplate(obj),
	}
}

type EventTemplateWithArgs[T runtime.Object, Args any] struct {
	Type            string
	Reason          string
	MessageTemplate func(T, Args) string
}

func (et *EventTemplateWithArgs[T, Args]) For(obj T, args Args) Event {
	return Event{
		InvolvedObject: obj,
		Type:           et.Type,
		Reason:         et.Reason,
		Message:        et.MessageTemplate(obj, args),
	}
}
