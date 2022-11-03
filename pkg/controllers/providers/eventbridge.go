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

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/eventbridge/eventbridgeiface"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/workqueue"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	awserrors "github.com/aws/karpenter/pkg/errors"
)

const (
	ScheduledChangedRule = "ScheduledChangeRule"
	SpotTerminationRule  = "SpotTerminationRule"
	RebalanceRule        = "RebalanceRule"
	StateChangeRule      = "StateChangeRule"
)

var DefaultRules = map[string]Rule{
	ScheduledChangedRule: {
		Name: fmt.Sprintf("Karpenter-%s-%s", ScheduledChangedRule, rand.String(64-len(fmt.Sprintf("Karpenter-%s-", ScheduledChangedRule)))),
		Pattern: Pattern{
			Source:     []string{"aws.health"},
			DetailType: []string{"AWS Health Event"},
		},
	},
	SpotTerminationRule: {
		Name: fmt.Sprintf("Karpenter-%s-%s", SpotTerminationRule, rand.String(64-len(fmt.Sprintf("Karpenter-%s-", SpotTerminationRule)))),
		Pattern: Pattern{
			Source:     []string{"aws.ec2"},
			DetailType: []string{"EC2 Spot Instance Interruption Warning"},
		},
	},
	RebalanceRule: {
		Name: fmt.Sprintf("Karpenter-%s-%s", RebalanceRule, rand.String(64-len(fmt.Sprintf("Karpenter-%s-", RebalanceRule)))),
		Pattern: Pattern{
			Source:     []string{"aws.ec2"},
			DetailType: []string{"EC2 Instance Rebalance Recommendation"},
		},
	},
	StateChangeRule: {
		Name: fmt.Sprintf("Karpenter-%s-%s", StateChangeRule, rand.String(64-len(fmt.Sprintf("Karpenter-%s-", StateChangeRule)))),
		Pattern: Pattern{
			Source:     []string{"aws.ec2"},
			DetailType: []string{"EC2 Instance State-change Notification"},
		},
	},
}

type Rule struct {
	Name    string
	Pattern Pattern
	Target  Target
}

const QueueTargetID = "KarpenterEventQueue"

func (er Rule) addQueueTarget(queueARN string) Rule {
	er.Target = Target{
		ID:  QueueTargetID,
		ARN: queueARN,
	}
	return er
}

type Target struct {
	ID  string
	ARN string
}

type Pattern struct {
	Source     []string `json:"source,omitempty"`
	DetailType []string `json:"detail-type,omitempty"`
}

func (ep Pattern) Serialize() []byte {
	return lo.Must(json.Marshal(ep))
}

type EventBridge struct {
	client      eventbridgeiface.EventBridgeAPI
	sqsProvider *SQS
}

func NewEventBridge(eb eventbridgeiface.EventBridgeAPI, sqsProvider *SQS) *EventBridge {
	return &EventBridge{
		client:      eb,
		sqsProvider: sqsProvider,
	}
}

func (eb *EventBridge) CreateRules(ctx context.Context) error {
	queueARN, err := eb.sqsProvider.queueARN.TryGet(ctx)
	if err != nil {
		return fmt.Errorf("resolving queue arn, %w", err)
	}
	existingRules, err := eb.DiscoverRules(ctx)
	if err != nil {
		return fmt.Errorf("discovering existing rules, %w", err)
	}
	rules := lo.MapToSlice(eb.mergeRules(existingRules), func(_ string, r Rule) Rule {
		return r.addQueueTarget(queueARN)
	})
	errs := make([]error, len(rules))
	workqueue.ParallelizeUntil(ctx, len(rules), len(rules), func(i int) {
		_, err := eb.client.PutRuleWithContext(ctx, &eventbridge.PutRuleInput{
			Name:         aws.String(rules[i].Name),
			EventPattern: aws.String(string(rules[i].Pattern.Serialize())),
			Tags:         eb.getTags(ctx),
		})
		if err != nil {
			errs[i] = multierr.Append(errs[i], err)
		}
		_, err = eb.client.PutTargetsWithContext(ctx, &eventbridge.PutTargetsInput{
			Rule: aws.String(rules[i].Name),
			Targets: []*eventbridge.Target{
				{
					Id:  aws.String(rules[i].Target.ID),
					Arn: aws.String(rules[i].Target.ARN),
				},
			},
		})
		if err != nil {
			errs[i] = multierr.Append(errs[i], err)
		}
	})
	return multierr.Combine(errs...)
}

func (eb *EventBridge) DiscoverRules(ctx context.Context) (map[string]Rule, error) {
	m := map[string]Rule{}
	output, err := eb.client.ListRulesWithContext(ctx, &eventbridge.ListRulesInput{
		NamePrefix: aws.String("Karpenter-"),
	})
	if err != nil {
		return nil, fmt.Errorf("listing rules, %w", err)
	}
	for _, rule := range output.Rules {
		out, err := eb.client.ListTagsForResourceWithContext(ctx, &eventbridge.ListTagsForResourceInput{
			ResourceARN: rule.Arn,
		})
		// If we get access denied, that means the tag-based policy didn't allow us to get the tags from the rule
		// which means it isn't a rule that we created for this cluster anyways
		if err != nil && !awserrors.IsAccessDenied(err) {
			return nil, fmt.Errorf("describing rules, %w", err)
		}
		for _, tag := range out.Tags {
			if aws.StringValue(tag.Key) == v1alpha5.DiscoveryTagKey &&
				aws.StringValue(tag.Value) == injection.GetOptions(ctx).ClusterName {

				// If we succeed to parse the rule name, we should store it by its rule type
				t, err := parseRuleName(aws.StringValue(rule.Name))
				if err == nil {
					m[t] = Rule{
						Name: aws.StringValue(rule.Name),
					}
				}
			}
		}
	}
	return m, nil
}

func (eb *EventBridge) DeleteRules(ctx context.Context) error {
	out, err := eb.DiscoverRules(ctx)
	if err != nil {
		return fmt.Errorf("discovering existing rules, %w", err)
	}
	rules := lo.Values(out)
	errs := make([]error, len(rules))
	workqueue.ParallelizeUntil(ctx, len(rules), len(rules), func(i int) {
		targetInput := &eventbridge.RemoveTargetsInput{
			Ids:  []*string{aws.String(QueueTargetID)},
			Rule: aws.String(rules[i].Name),
		}
		_, err := eb.client.RemoveTargetsWithContext(ctx, targetInput)
		if err != nil && !awserrors.IsNotFound(err) {
			errs[i] = err
			return
		}
		ruleInput := &eventbridge.DeleteRuleInput{
			Name: aws.String(rules[i].Name),
		}
		_, err = eb.client.DeleteRuleWithContext(ctx, ruleInput)
		if err != nil && !awserrors.IsNotFound(err) {
			errs[i] = err
		}
	})
	return multierr.Combine(errs...)
}

// mergeRules merges the existing rules with the default rules based on the rule type
func (eb *EventBridge) mergeRules(existing map[string]Rule) map[string]Rule {
	rules := lo.Assign(DefaultRules)
	for k, rule := range rules {
		if existingRule, ok := existing[k]; ok {
			rule.Name = existingRule.Name
			rules[k] = rule
		}
	}
	return rules
}

func (eb *EventBridge) getTags(ctx context.Context) []*eventbridge.Tag {
	return append(
		[]*eventbridge.Tag{
			{
				Key:   aws.String(v1alpha5.DiscoveryTagKey),
				Value: aws.String(injection.GetOptions(ctx).ClusterName),
			},
			{
				Key:   aws.String(v1alpha5.ManagedByTagKey),
				Value: aws.String(injection.GetOptions(ctx).ClusterName),
			},
		},
		lo.MapToSlice(awssettings.FromContext(ctx).Tags, func(k, v string) *eventbridge.Tag {
			return &eventbridge.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			}
		})...,
	)
}

// parseRuleName parses out the rule type based on the expected naming convention for rules
// provisioned by Karpenter
func parseRuleName(raw string) (string, error) {
	r := regexp.MustCompile(`Karpenter-(?P<RuleType>.*)-.*`)
	matches := r.FindStringSubmatch(raw)
	if matches == nil {
		return "", fmt.Errorf("parsing rule name, %s", raw)
	}
	for i, name := range r.SubexpNames() {
		if name == "RuleType" {
			return matches[i], nil
		}
	}
	return "", fmt.Errorf("parsing rule name, %s", raw)
}
