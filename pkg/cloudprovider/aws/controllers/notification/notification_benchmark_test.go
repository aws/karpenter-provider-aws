package notification_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"

	awscloudprovider "github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
)

func benchmarkNotificationController(b *testing.B, messageCount int) {
	ctx := logging.WithLogger(context.Background(), zap.NewNop().Sugar())
	providers := newProviders()
	if err := providers.makeInfrastructure(ctx); err != nil {
		b.Fatalf("standing up infrastructure, %v", err)
	}
	if err := providers.provisionMessages(ctx, makeDiverseMessages(messageCount)); err != nil {
		b.Fatalf("provisioning messages, %v", err)
	}
}

type providers struct {
	sqsProvider         *awscloudprovider.SQSProvider
	eventBridgeProvider *awscloudprovider.EventBridgeProvider
}

func newProviders() providers {
	sess := session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			client.DefaultRetryer{NumMaxRetries: client.DefaultRetryerMaxNumRetries},
		),
	))
	sqsProvider := awscloudprovider.NewSQSProvider(ctx, sqs.New(sess))
	eventBridgeProvider := awscloudprovider.NewEventBridgeProvider(eventbridge.New(sess), sqsProvider)
	return providers{
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

func (p *providers) makeInfrastructure(ctx context.Context) error {
	infraProvider := infrastructure.NewProvider(p.sqsProvider, p.eventBridgeProvider)
	return infraProvider.CreateInfrastructure(ctx)
}

func (p *providers) provisionMessages(ctx context.Context, messages ...[]interface{}) error {
	errs := make([]error, len(messages))
	workqueue.ParallelizeUntil(ctx, 20, len(messages), func(i int) {
		_, err := p.sqsProvider.SendMessage(ctx, messages[i])
		errs[i] = err
	})
	return multierr.Combine(errs...)
}

func makeDiverseMessages(count int) []interface{} {
	var messages []interface{}

	messages = append(messages, makeScheduledChangeMessages(count/3))
	messages = append(messages, makeSpotInterruptionMessages(count/3))

	messages = append(messages, makeStateChangeMessages(count-len(messages), []string{
		"stopping", "stopped", "shutting-down", "terminated",
	}))
	return messages
}

func makeScheduledChangeMessages(count int) []interface{} {
	var msgs []interface{}
	for i := 0; i < count; i++ {
		msgs = append(msgs, scheduledChangeMessage(makeInstanceID()))
	}
	return msgs
}

func makeStateChangeMessages(count int, states []string) []interface{} {
	var msgs []interface{}
	for i := 0; i < count; i++ {
		state := states[rand.Intn(len(states))]
		msgs = append(msgs, stateChangeMessage(makeInstanceID(), state))
	}
	return msgs
}

func makeSpotInterruptionMessages(count int) []interface{} {
	var msgs []interface{}
	for i := 0; i < count; i++ {
		msgs = append(msgs, spotInterruptionMessage(makeInstanceID()))
	}
	return msgs
}
