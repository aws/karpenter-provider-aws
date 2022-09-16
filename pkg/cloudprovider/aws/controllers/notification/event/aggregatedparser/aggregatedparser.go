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

package aggregatedparser

import (
	"context"
	"encoding/json"

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	rebalancerecommendationv0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/rebalancerecommendation/v0"
	scheduledchangev0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/scheduledchange/v0"
	spotinterruptionv0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/spotinterruption/v0"
	statechangev0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/statechange/v0"
)

var (
	DefaultParsers = []event.Parser{
		statechangev0.Parser{},
		spotinterruptionv0.Parser{},
		scheduledchangev0.Parser{},
		rebalancerecommendationv0.Parser{},
	}
)

type AggregatedParser []event.Parser

func NewAggregatedParser(parsers ...event.Parser) AggregatedParser {
	return parsers
}

func (p AggregatedParser) Parse(ctx context.Context, str string) event.Interface {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("event.parser"))

	if str == "" {
		return event.NoOp{}
	}

	// We will go through all the parsers to see if we can parse
	// If we aren't able to parse the message, we will just assume that it is a no-op
	for _, parser := range p {
		if a := parser.Parse(ctx, str); a != nil {
			return a
		}
	}

	md := event.AWSMetadata{}
	if err := json.Unmarshal([]byte(str), &md); err != nil {
		logging.FromContext(ctx).
			With("error", err).
			Error("failed to unmarshal message metadata")
		return event.NoOp{}
	}
	return event.NoOp(md)
}
