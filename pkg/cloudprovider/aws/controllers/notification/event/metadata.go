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
	"time"

	"go.uber.org/zap/zapcore"
)

type AWSMetadata struct {
	Account    string    `json:"account"`
	DetailType string    `json:"detail-type"`
	ID         string    `json:"id"`
	Region     string    `json:"region"`
	Resources  []string  `json:"resources"`
	Source     string    `json:"source"`
	Time       time.Time `json:"time"`
	Version    string    `json:"version"`
}

func (e AWSMetadata) MarshalLogObject(enc zapcore.ObjectEncoder) (err error) {
	enc.AddString("source", e.Source)
	enc.AddString("detail-type", e.DetailType)
	enc.AddString("id", e.ID)
	enc.AddTime("time", e.Time)
	enc.AddString("region", e.Region)
	_ = enc.AddArray("resources", zapcore.ArrayMarshalerFunc(func(enc zapcore.ArrayEncoder) error {
		for _, resource := range e.Resources {
			enc.AppendString(resource)
		}
		return nil
	}))
	enc.AddString("version", e.Version)
	enc.AddString("account", e.Account)
	return err
}
