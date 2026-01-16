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

package messages

import (
	"time"
)

type Parser interface {
	Parse(string) (Message, error)

	Version() string
	Source() string
	DetailType() string
}

type Message interface {
	EC2InstanceIDs() []string
	Kind() Kind
	StartTime() time.Time
}

type Kind string

const (
	RebalanceRecommendationKind Kind = "rebalance_recommendation"
	ScheduledChangeKind         Kind = "scheduled_change"
	SpotInterruptionKind        Kind = "spot_interrupted"
	InstanceStoppedKind         Kind = "instance_stopped"
	InstanceTerminatedKind      Kind = "instance_terminated"
	InstanceStatusFailure       Kind = "instance_status_failure"
	NoOpKind                    Kind = "no_op"
)

type Metadata struct {
	Account    string    `json:"account"`
	DetailType string    `json:"detail-type"`
	ID         string    `json:"id"`
	Region     string    `json:"region"`
	Resources  []string  `json:"resources"`
	Source     string    `json:"source"`
	Time       time.Time `json:"time"`
	Version    string    `json:"version"`
}

func (m Metadata) StartTime() time.Time {
	return m.Time
}
