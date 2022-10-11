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

package event

import (
	"fmt"
)

type Parser interface {
	Parse(string) (Interface, error)

	Version() string
	Source() string
	DetailType() string
}

type Interface interface {
	EC2InstanceIDs() []string
	Kind() Kind
}

type Kind byte

const (
	_ = iota
	RebalanceRecommendationKind
	ScheduledChangeKind
	SpotInterruptionKind
	StateChangeKind
	NoOpKind
)

// manually written or generated using https://pkg.go.dev/golang.org/x/tools/cmd/stringer
func (k Kind) String() string {
	switch k {
	case RebalanceRecommendationKind:
		return "RebalanceRecommendation"
	case ScheduledChangeKind:
		return "ScheduledChange"
	case SpotInterruptionKind:
		return "SpotInterruption"
	case StateChangeKind:
		return "StateChange"
	case NoOpKind:
		return "NoOp"
	default:
		return fmt.Sprintf("Unsupported Kind %d", k)
	}
}
