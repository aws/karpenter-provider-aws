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

package spotinterruption

import (
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
)

// Message contains the properties defined in AWS EventBridge schema
// aws.ec2@EC2SpotInstanceInterruptionWarning v0.
type Message struct {
	messages.Metadata

	Detail Detail `json:"detail"`
}

type Detail struct {
	InstanceID     string `json:"instance-id"`
	InstanceAction string `json:"instance-action"`
}

func (m Message) EC2InstanceIDs() []string {
	return []string{m.Detail.InstanceID}
}

func (Message) Kind() messages.Kind {
	return messages.SpotInterruptionKind
}
