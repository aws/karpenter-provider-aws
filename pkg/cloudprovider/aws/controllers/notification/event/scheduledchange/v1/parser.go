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

package v1

import (
	"context"
	"encoding/json"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

const (
	source                    = "aws.health"
	detailType                = "AWS Health Event"
	version                   = "1"
	acceptedService           = "EC2"
	acceptedEventTypeCategory = "scheduledChange"
)

type Parser struct{}

func (Parser) Parse(ctx context.Context, str string) event.Interface {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("scheduledChange.v1"))

	evt := AWSHealthEvent{}
	if err := json.Unmarshal([]byte(str), &evt); err != nil {
		logging.FromContext(ctx).
			With("error", err).
			Error("failed to unmarshal AWS health event")
		return nil
	}

	if evt.Source != source || evt.DetailType != detailType || evt.Version != version {
		return nil
	}

	if evt.Detail.Service != acceptedService {
		logging.FromContext(ctx).
			With("eventDetails", evt).
			With("acceptedService", acceptedService).
			Warn("ignoring AWS health event")
		return nil
	}

	if evt.Detail.EventTypeCategory != acceptedEventTypeCategory {
		logging.FromContext(ctx).
			With("eventDetails", evt).
			With("acceptedEventTypeCategory", acceptedEventTypeCategory).
			Warn("ignoring AWS health event")
		return nil
	}

	return evt
}
