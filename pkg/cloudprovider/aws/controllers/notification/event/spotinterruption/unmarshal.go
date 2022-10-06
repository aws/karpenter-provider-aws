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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

// AWSEvent contains the properties defined in AWS EventBridge schema
// aws.ec2@EC2SpotInstanceInterruptionWarning v1.
type AWSEvent struct {
	event.AWSMetadata

	Detail EC2SpotInstanceInterruptionWarningDetail `json:"detail"`
}

func (e AWSEvent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	zap.Inline(e.AWSMetadata).AddTo(enc)
	return enc.AddObject("detail", e.Detail)
}

type EC2SpotInstanceInterruptionWarningDetail struct {
	InstanceID     string `json:"instance-id"`
	InstanceAction string `json:"instance-action"`
}

func (e EC2SpotInstanceInterruptionWarningDetail) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("instance-id", e.InstanceID)
	enc.AddString("instance-action", e.InstanceAction)
	return nil
}
