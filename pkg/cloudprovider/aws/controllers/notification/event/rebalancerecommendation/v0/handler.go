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

package v0

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

type EC2InstanceRebalanceRecommendation AWSEvent

func (e EC2InstanceRebalanceRecommendation) EventID() string {
	return e.ID
}

func (e EC2InstanceRebalanceRecommendation) EC2InstanceIDs() []string {
	return []string{e.Detail.InstanceID}
}

func (EC2InstanceRebalanceRecommendation) Kind() event.Kind {
	return event.Kinds.RebalanceRecommendation
}

func (e EC2InstanceRebalanceRecommendation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	zap.Inline(AWSEvent(e)).AddTo(enc)
	return nil
}

func (e EC2InstanceRebalanceRecommendation) StartTime() time.Time {
	return e.Time
}
