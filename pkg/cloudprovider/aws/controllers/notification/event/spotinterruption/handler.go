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
	"time"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

type EC2SpotInstanceInterruptionWarning AWSEvent

func (e EC2SpotInstanceInterruptionWarning) EventID() string {
	return e.ID
}

func (e EC2SpotInstanceInterruptionWarning) EC2InstanceIDs() []string {
	return []string{e.Detail.InstanceID}
}

func (EC2SpotInstanceInterruptionWarning) Kind() event.Kind {
	return event.SpotInterruptionKind
}

func (e EC2SpotInstanceInterruptionWarning) StartTime() time.Time {
	return e.Time
}
