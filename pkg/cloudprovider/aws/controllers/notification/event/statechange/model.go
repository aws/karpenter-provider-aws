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

package statechange

import (
	"time"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

// Event contains the properties defined in AWS EventBridge schema
// aws.ec2@EC2InstanceStateChangeNotification v1.
type Event struct {
	event.AWSMetadata

	Detail Detail `json:"detail"`
}

type Detail struct {
	InstanceID string `json:"instance-id"`
	State      string `json:"state"`
}

func (e Event) EventID() string {
	return e.ID
}

func (e Event) EC2InstanceIDs() []string {
	return []string{e.Detail.InstanceID}
}

func (e Event) State() string {
	return e.Detail.State
}

func (Event) Kind() event.Kind {
	return event.StateChangeKind
}

func (e Event) StartTime() time.Time {
	return e.Time
}
