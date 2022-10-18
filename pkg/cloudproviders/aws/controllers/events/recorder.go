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

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type EventType byte

const (
	_ EventType = iota
	NormalType
	WarningType
)

func (e EventType) String() string {
	switch e {
	case NormalType:
		return "Normal"
	case WarningType:
		return "Warning"
	default:
		return fmt.Sprintf("Unsupported EventType %d", e)
	}
}

type Event interface {
	InvolvedObject() runtime.Object
	Type() EventType
	Reason() string
	Message() string
}

type Recorder interface {
	Create(evt Event)
}

type recorder struct {
	rec record.EventRecorder
}

func NewRecorder(r record.EventRecorder) Recorder {
	return &recorder{
		rec: r,
	}
}

func (r *recorder) Create(evt Event) {
	r.rec.Eventf(evt.InvolvedObject(), evt.Type().String(), evt.Reason(), evt.Message())
}
