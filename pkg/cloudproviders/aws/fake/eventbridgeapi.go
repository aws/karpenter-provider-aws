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

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/eventbridge/eventbridgeiface"
)

// EventBridgeBehavior must be reset between tests otherwise tests will
// pollute each other.
type EventBridgeBehavior struct {
	PutRuleBehavior       MockedFunction[eventbridge.PutRuleInput, eventbridge.PutRuleOutput]
	PutTargetsBehavior    MockedFunction[eventbridge.PutTargetsInput, eventbridge.PutTargetsOutput]
	DeleteRuleBehavior    MockedFunction[eventbridge.DeleteRuleInput, eventbridge.DeleteRuleOutput]
	RemoveTargetsBehavior MockedFunction[eventbridge.RemoveTargetsInput, eventbridge.RemoveTargetsOutput]
}

type EventBridgeAPI struct {
	eventbridgeiface.EventBridgeAPI
	EventBridgeBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (eb *EventBridgeAPI) Reset() {
	eb.PutRuleBehavior.Reset()
	eb.PutTargetsBehavior.Reset()
	eb.DeleteRuleBehavior.Reset()
	eb.RemoveTargetsBehavior.Reset()
}

// TODO: Create a dummy rule ARN for the default that is returned from this function
func (eb *EventBridgeAPI) PutRuleWithContext(_ context.Context, input *eventbridge.PutRuleInput, _ ...request.Option) (*eventbridge.PutRuleOutput, error) {
	return eb.PutRuleBehavior.Invoke(input)
}

// TODO: Create a default response that returns failed entries
func (eb *EventBridgeAPI) PutTargetsWithContext(_ context.Context, input *eventbridge.PutTargetsInput, _ ...request.Option) (*eventbridge.PutTargetsOutput, error) {
	return eb.PutTargetsBehavior.Invoke(input)
}

func (eb *EventBridgeAPI) DeleteRuleWithContext(_ context.Context, input *eventbridge.DeleteRuleInput, _ ...request.Option) (*eventbridge.DeleteRuleOutput, error) {
	return eb.DeleteRuleBehavior.Invoke(input)
}

func (eb *EventBridgeAPI) RemoveTargetsWithContext(_ context.Context, input *eventbridge.RemoveTargetsInput, _ ...request.Option) (*eventbridge.RemoveTargetsOutput, error) {
	return eb.RemoveTargetsBehavior.Invoke(input)
}
