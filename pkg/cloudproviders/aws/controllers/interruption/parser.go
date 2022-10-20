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

package interruption

import (
	"encoding/json"
	"fmt"

	"github.com/samber/lo"

	event2 "github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/noop"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/rebalancerecommendation"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/spotinterruption"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/statechange"
)

type parserKey struct {
	Version    string
	Source     string
	DetailType string
}

func newParserKey(metadata event2.Metadata) parserKey {
	return parserKey{
		Version:    metadata.Version,
		Source:     metadata.Source,
		DetailType: metadata.DetailType,
	}
}

func newParserKeyFromParser(p event2.Parser) parserKey {
	return parserKey{
		Version:    p.Version(),
		Source:     p.Source(),
		DetailType: p.DetailType(),
	}
}

var (
	DefaultParsers = []event2.Parser{
		statechange.Parser{},
		spotinterruption.Parser{},
		scheduledchange.Parser{},
		rebalancerecommendation.Parser{},
	}
)

type EventParser struct {
	parserMap map[parserKey]event2.Parser
}

func NewEventParser(parsers ...event2.Parser) *EventParser {
	return &EventParser{
		parserMap: lo.SliceToMap(parsers, func(p event2.Parser) (parserKey, event2.Parser) {
			return newParserKeyFromParser(p), p
		}),
	}
}

func (p EventParser) Parse(msg string) (event2.Interface, error) {
	if msg == "" {
		return noop.Event{}, nil
	}
	md := event2.Metadata{}
	if err := json.Unmarshal([]byte(msg), &md); err != nil {
		return noop.Event{}, fmt.Errorf("unmarshalling the message as Metadata, %w", err)
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
	return noop.Event{Metadata: md}, nil
}
