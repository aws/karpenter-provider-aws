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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/eventbridge/eventbridgeiface"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"

	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	awserrors "github.com/aws/karpenter/pkg/cloudproviders/aws/errors"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/utils"
	"github.com/aws/karpenter/pkg/operator/injection"
)

const QueueTargetID = "KarpenterEventQueue"

type rule struct {
	Name    string
	Pattern *pattern
	Target  *target
}

type target struct {
	ID  string
	ARN string
}

type pattern struct {
	Source     []string `json:"source,omitempty"`
	DetailType []string `json:"detail-type,omitempty"`
}

func (ep *pattern) Serialize() []byte {
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

func (eb *EventBridge) CreateEC2NotificationRules(ctx context.Context) error {
	queueARN, err := eb.sqsProvider.queueARN.TryGet(ctx)
	if err != nil {
		return fmt.Errorf("resolving queue arn, %w", err)
	}
	rules := lo.Map(eb.getEC2NotificationEventRules(ctx), func(r rule, _ int) rule { return r.AddQueueTarget(queueARN) })
	errs := make([]error, len(rules))
	workqueue.ParallelizeUntil(ctx, len(rules), len(rules), func(i int) {
		_, err := eb.client.PutRuleWithContext(ctx, &eventbridge.PutRuleInput{
			Name:         aws.String(rules[i].Name),
			EventPattern: aws.String(string(rules[i].Pattern.Serialize())),
			Tags: []*eventbridge.Tag{
				{
					Key:   aws.String(awsv1alpha1.DiscoveryTagKey),
					Value: aws.String(injection.GetOptions(ctx).ClusterName),
				},
			},
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

func (eb *EventBridge) DeleteEC2NotificationRules(ctx context.Context) error {
	rules := eb.getEC2NotificationEventRules(ctx)
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

func (eb *EventBridge) getEC2NotificationEventRules(ctx context.Context) []rule {
	return []rule{
		{
			Name: fmt.Sprintf("Karpenter-ScheduledChangeRule-%s", utils.GetClusterNameHash(ctx, 20)),
			Pattern: &pattern{
				Source:     []string{"aws.health"},
				DetailType: []string{"AWS Health Event"},
			},
		},
		{
			Name: fmt.Sprintf("Karpenter-SpotTerminationRule-%s", utils.GetClusterNameHash(ctx, 20)),
			Pattern: &pattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Spot Instance Interruption Warning"},
			},
		},
		{
			Name: fmt.Sprintf("Karpenter-RebalanceRule-%s", utils.GetClusterNameHash(ctx, 20)),
			Pattern: &pattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Instance Rebalance Recommendation"},
			},
		},
		{
			Name: fmt.Sprintf("Karpenter-InstanceStateChangeRule-%s", utils.GetClusterNameHash(ctx, 20)),
			Pattern: &pattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Instance State-change Notification"},
			},
		},
	}
}

func (er rule) AddQueueTarget(queueARN string) rule {
	er.Target = &target{
		ID:  QueueTargetID,
		ARN: queueARN,
	}
	return er
}
