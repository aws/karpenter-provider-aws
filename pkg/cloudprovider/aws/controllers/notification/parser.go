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

package notification

import (
	"encoding/json"
	"fmt"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/noop"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/rebalancerecommendation"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/scheduledchange"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/spotinterruption"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/statechange"
)

type parserKey struct {
	Version    string
	Source     string
	DetailType string
}

func newParserKey(metadata event.AWSMetadata) parserKey {
	return parserKey{
		Version:    metadata.Version,
		Source:     metadata.Source,
		DetailType: metadata.DetailType,
	}
}

func newParserKeyFromParser(p event.Parser) parserKey {
	return parserKey{
		Version:    p.Version(),
		Source:     p.Source(),
		DetailType: p.DetailType(),
	}
}

var (
	DefaultParsers = []event.Parser{
		statechange.Parser{},
		spotinterruption.Parser{},
		scheduledchange.Parser{},
		rebalancerecommendation.Parser{},
	}
)

type EventParser struct {
	parserMap map[parserKey]event.Parser
}

func NewEventParser(parsers ...event.Parser) *EventParser {
	return &EventParser{
		parserMap: lo.SliceToMap(parsers, func(p event.Parser) (parserKey, event.Parser) {
			return newParserKeyFromParser(p), p
		}),
	}
}

func (p EventParser) Parse(msg string) (event.Interface, error) {
	if msg == "" {
		return noop.Event{}, nil
	}
	md := event.AWSMetadata{}
	if err := json.Unmarshal([]byte(msg), &md); err != nil {
		return noop.Event{}, fmt.Errorf("unmarshalling the message as AWSMetadata, %w", err)
	}
	if parser, ok := p.parserMap[newParserKey(md)]; ok {
		evt, err := parser.Parse(msg)
		if err != nil {
			return noop.Event{}, fmt.Errorf("parsing event message, %w", err)
		}
		if evt == nil {
			return noop.Event{}, nil
		}
		return evt, nil
	}
	return noop.Event{AWSMetadata: md}, nil
}
