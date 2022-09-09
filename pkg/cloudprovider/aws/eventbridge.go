package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/samber/lo"
	"go.uber.org/multierr"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/injection"
)

type EventBridgeClient interface {
	PutRuleWithContext(context.Context, *eventbridge.PutRuleInput, ...request.Option) (*eventbridge.PutRuleOutput, error)
	PutTargetsWithContext(context.Context, *eventbridge.PutTargetsInput, ...request.Option) (*eventbridge.PutTargetsOutput, error)
}

type EventBridgeProvider struct {
	EventBridgeClient
	queueName string
	metadata  *Metadata
}

type EventRule struct {
	Name    string
	Pattern *EventPattern
	Target  *EventTarget
}

type EventTarget struct {
	ID  string
	ARN string
}

type EventPattern struct {
	Source     []string `json:"source,omitempty"`
	DetailType []string `json:"detailType,omitempty"`
}

func (ep *EventPattern) Serialize() []byte {
	return lo.Must(json.Marshal(ep))
}

func NewEventBridgeProvider(eb EventBridgeClient, metadata *Metadata, queueName string) *EventBridgeProvider {
	return &EventBridgeProvider{
		EventBridgeClient: eb,
		metadata:          metadata,
		queueName:         queueName,
	}
}

func (eb *EventBridgeProvider) CreateEC2NotificationRules(ctx context.Context) error {
	var err error
	for _, rule := range eb.getEC2NotificationEventRules(ctx) {
		_, e := eb.PutRuleWithContext(ctx, &eventbridge.PutRuleInput{
			Name:         aws.String(rule.Name),
			EventPattern: aws.String(string(rule.Pattern.Serialize())),
			Tags: []*eventbridge.Tag{
				{
					Key:   aws.String(v1alpha5.DiscoveryLabelKey),
					Value: aws.String(injection.GetOptions(ctx).ClusterName),
				},
			},
		})
		err = multierr.Append(err, e)
		_, e = eb.PutTargetsWithContext(ctx, &eventbridge.PutTargetsInput{
			Rule: aws.String(rule.Name),
			Targets: []*eventbridge.Target{
				{
					Id:  aws.String(rule.Target.ID),
					Arn: aws.String(rule.Target.ARN),
				},
			},
		})
		err = multierr.Append(err, e)
	}
	return err
}

func (eb *EventBridgeProvider) getEC2NotificationEventRules(ctx context.Context) []EventRule {
	return []EventRule{
		{
			Name: fmt.Sprintf("%s-ScheduledChangeRule", injection.GetOptions(ctx).ClusterName),
			Pattern: &EventPattern{
				Source:     []string{"aws.health"},
				DetailType: []string{"AWS Health Event"},
			},
			Target: &EventTarget{
				ID:  "1",
				ARN: eb.getQueueARN(),
			},
		},
		{
			Name: fmt.Sprintf("%s-SpotTerminationRule", injection.GetOptions(ctx).ClusterName),
			Pattern: &EventPattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Spot Instance Interruption Warning"},
			},
			Target: &EventTarget{
				ID:  "1",
				ARN: eb.getQueueARN(),
			},
		},
		{
			Name: fmt.Sprintf("%s-RebalanceRule", injection.GetOptions(ctx).ClusterName),
			Pattern: &EventPattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Instance Rebalance Recommendation"},
			},
			Target: &EventTarget{
				ID:  "1",
				ARN: eb.getQueueARN(),
			},
		},
		{
			Name: fmt.Sprintf("%s-InstanceStateChangeRule", injection.GetOptions(ctx).ClusterName),
			Pattern: &EventPattern{
				Source:     []string{"aws.ec2"},
				DetailType: []string{"EC2 Instance State-change Notification"},
			},
			Target: &EventTarget{
				ID:  "1",
				ARN: eb.getQueueARN(),
			},
		},
	}
}

func (eb *EventBridgeProvider) getQueueARN() string {
	return fmt.Sprintf("arn:aws:sqs:%s:%s:%s", eb.metadata.region, eb.metadata.accountID, eb.queueName)
}
