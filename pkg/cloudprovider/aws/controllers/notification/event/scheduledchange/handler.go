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

package scheduledchange

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

type AWSHealthEvent AWSEvent

func (e AWSHealthEvent) EventID() string {
	return e.ID
}

func (e AWSHealthEvent) EC2InstanceIDs() []string {
	ids := make([]string, len(e.Detail.AffectedEntities))
	for i, entity := range e.Detail.AffectedEntities {
		ids[i] = entity.EntityValue
	}
	return ids
}

func (AWSHealthEvent) Kind() event.Kind {
	return event.Kinds.ScheduledChange
}

func (e AWSHealthEvent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	zap.Inline(AWSEvent(e)).AddTo(enc)
	return nil
}

func (e AWSHealthEvent) StartTime() time.Time {
	return e.Time
}
