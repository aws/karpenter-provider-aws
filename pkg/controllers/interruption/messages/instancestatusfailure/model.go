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

package instancestatusfailure

import (
	"time"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
)

// Message contains the Instance Status from EC2.DescribeInstanceStatus
// This is not vended via EventBridge but is handled in a similar manner
// as other EventBridge messages.
type Message instancestatus.HealthStatus

func (m Message) EC2InstanceIDs() []string {
	return []string{m.InstanceID}
}

func (Message) Kind() messages.Kind {
	return messages.InstanceStatusFailure
}

func (m Message) StartTime() time.Time {
	return m.ImpairedSince
}
