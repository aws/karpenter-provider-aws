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
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
)

// Message contains the properties defined in AWS EventBridge schema
// aws.ec2@EC2InstanceStateChangeNotification v1.
type Message struct {
	messages.Metadata

	Detail Detail `json:"detail"`
}

type Detail struct {
	InstanceID string `json:"instance-id"`
	State      string `json:"state"`
}

func (m Message) EC2InstanceIDs() []string {
	return []string{m.Detail.InstanceID}
}

func (m Message) Kind() messages.Kind {
	if lo.Contains([]string{"stopping", "stopped"}, m.Detail.State) {
		return messages.InstanceStoppedKind
	}
	return messages.InstanceTerminatedKind
}
