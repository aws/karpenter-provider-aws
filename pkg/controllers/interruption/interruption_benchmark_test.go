//go:build test_performance

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

//nolint:gosec
package interruption_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	servicesqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/go-logr/zapr"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	clock "k8s.io/utils/clock/testing"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/events"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

var r = rand.New(rand.NewSource(time.Now().Unix()))

func BenchmarkNotification15000(b *testing.B) {
	benchmarkNotificationController(b, 15000)
}

func BenchmarkNotification5000(b *testing.B) {
	benchmarkNotificationController(b, 5000)
}

func BenchmarkNotification1000(b *testing.B) {
	benchmarkNotificationController(b, 1000)
}

func BenchmarkNotification100(b *testing.B) {
	benchmarkNotificationController(b, 100)
}

//nolint:gocyclo
func benchmarkNotificationController(b *testing.B, messageCount int) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("message-count", messageCount))
	fakeClock = &clock.FakeClock{}
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		ClusterName:       lo.ToPtr("karpenter-notification-benchmarking"),
		IsolatedVPC:       lo.ToPtr(true),
		InterruptionQueue: lo.ToPtr("test-cluster"),
	}))
	env = coretest.NewEnvironment()
	// Stop the coretest environment after the coretest completes
	defer func() {
		if err := retry.Do(func() error {
			return env.Stop()
		}); err != nil {
			b.Fatalf("stopping coretest environment, %v", err)
		}
	}()

	providers := newProviders(ctx, env.Client)
	queueURL, err := providers.makeInfrastructure(ctx)
	if err != nil {
		b.Fatalf("standing up infrastructure, %v", err)
	}
	// Cleanup the infrastructure after the coretest completes
	defer func() {
		if err := retry.Do(func() error {
			return providers.cleanupInfrastructure(queueURL)
		}); err != nil {
			b.Fatalf("deleting infrastructure, %v", err)
		}
	}()

	// Load all the fundamental components before setting up the controllers
	recorder := coretest.NewEventRecorder()
	unavailableOfferingsCache = awscache.NewUnavailableOfferings()

	// Set-up the controllers
	interruptionController := interruption.NewController(env.Client, fakeClock, recorder, providers.sqsProvider, unavailableOfferingsCache)

	messages, nodes := makeDiverseMessagesAndNodes(messageCount)
	log.FromContext(ctx).Info("provisioning nodes")
	if err := provisionNodes(ctx, env.Client, nodes); err != nil {
		b.Fatalf("provisioning nodes, %v", err)
	}
	log.FromContext(ctx).Info("completed provisioning nodes")

	log.FromContext(ctx).Info("provisioning messages into the SQS Queue")
	if err := providers.provisionMessages(ctx, messages...); err != nil {
		b.Fatalf("provisioning messages, %v", err)
	}
	log.FromContext(ctx).Info("completed provisioning messages into the SQS Queue")

	m, err := controllerruntime.NewManager(env.Config, controllerruntime.Options{
		BaseContext: func() context.Context { return log.IntoContext(ctx, zapr.NewLogger(zap.NewNop())) },
	})
	if err != nil {
		b.Fatalf("creating manager, %v", err)
	}

	// Registering controller with the manager
	if err = interruptionController.Builder(ctx, m).Complete(interruptionController); err != nil {
		b.Fatalf("registering interruption controller, %v", err)
	}

	b.ResetTimer()
	start := time.Now()
	managerErr := make(chan error)
	go func() {
		log.FromContext(ctx).Info("starting controller manager")
		managerErr <- m.Start(ctx)
	}()

	select {
	case <-providers.monitorMessagesProcessed(ctx, recorder, messageCount):
	case err = <-managerErr:
		b.Fatalf("running manager, %v", err)
	}

	duration := time.Since(start)
	b.ReportMetric(float64(messageCount), "Messages")
	b.ReportMetric(duration.Seconds(), "TotalDurationInSeconds")
	b.ReportMetric(float64(messageCount)/duration.Seconds(), "Messages/Second")
}

type providerSet struct {
	kubeClient  client.Client
	sqsAPI      sqs.Client
	sqsProvider sqs.Provider
}

func newProviders(ctx context.Context, kubeClient client.Client) providerSet {
	cfg := lo.Must(config.LoadDefaultConfig(ctx))
	sqsAPI := servicesqs.New(cfg)
	out := lo.Must(sqsAPI.GetQueueUrlWithContext(ctx, &servicesqs.GetQueueUrlInput{QueueName: lo.ToPtr(options.FromContext(ctx).InterruptionQueue)}))
	return providerSet{
		kubeClient:  kubeClient,
		sqsAPI:      sqsAPI,
		sqsProvider: lo.Must(sqs.NewDefaultProvider(sqsAPI, lo.FromPtr(out.QueueUrl))),
	}
}

func (p *providerSet) makeInfrastructure(ctx context.Context) (string, error) {
	out, err := p.sqsAPI.CreateQueueWithContext(ctx, &servicesqs.CreateQueueInput{
		QueueName: lo.ToPtr(options.FromContext(ctx).InterruptionQueue),
		Attributes: map[string]*string{
			servicesqs.QueueAttributeNameMessageRetentionPeriod: aws.String("1200"), // 20 minutes for this test
		},
	})
	if err != nil {
		return "", fmt.Errorf("creating servicesqs queue, %w", err)
	}
	return lo.FromPtr(out.QueueUrl), nil
}

func (p *providerSet) cleanupInfrastructure(queueURL string) error {
	if _, err := p.sqsAPI.DeleteQueueWithContext(ctx, &servicesqs.DeleteQueueInput{
		QueueUrl: lo.ToPtr(queueURL),
	}); err != nil {
		return fmt.Errorf("deleting servicesqs queue, %w", err)
	}
	return nil
}

func (p *providerSet) provisionMessages(ctx context.Context, messages ...any) error {
	errs := make([]error, len(messages))
	workqueue.ParallelizeUntil(ctx, 20, len(messages), func(i int) {
		_, err := p.sqsProvider.SendMessage(ctx, messages[i])
		errs[i] = err
	})
	return multierr.Combine(errs...)
}

func (p *providerSet) monitorMessagesProcessed(ctx context.Context, eventRecorder *coretest.EventRecorder, expectedProcessed int) <-chan struct{} {
	done := make(chan struct{})
	totalProcessed := 0
	go func() {
		for totalProcessed < expectedProcessed {
			totalProcessed = eventRecorder.Calls(events.Stopping(coretest.Node(), coretest.NodeClaim())[0].Reason) +
				eventRecorder.Calls(events.Stopping(coretest.Node(), coretest.NodeClaim())[0].Reason) +
				eventRecorder.Calls(events.Unhealthy(coretest.Node(), coretest.NodeClaim())[0].Reason) +
				eventRecorder.Calls(events.RebalanceRecommendation(coretest.Node(), coretest.NodeClaim())[0].Reason) +
				eventRecorder.Calls(events.SpotInterrupted(coretest.Node(), coretest.NodeClaim())[0].Reason)
			log.FromContext(ctx).WithValues("processed-message-count", totalProcessed).Info("processed messages from the queue")
			time.Sleep(time.Second)
		}
		close(done)
	}()
	return done
}

func provisionNodes(ctx context.Context, kubeClient client.Client, nodes []*corev1.Node) error {
	errs := make([]error, len(nodes))
	workqueue.ParallelizeUntil(ctx, 20, len(nodes), func(i int) {
		if err := retry.Do(func() error {
			return kubeClient.Create(ctx, nodes[i])
		}); err != nil {
			errs[i] = fmt.Errorf("provisioning node, %w", err)
		}
	})
	return multierr.Combine(errs...)
}

func makeDiverseMessagesAndNodes(count int) ([]any, []*corev1.Node) {
	var messages []any
	var nodes []*corev1.Node

	newMessages, newNodes := makeScheduledChangeMessagesAndNodes(count / 3)
	messages = append(messages, newMessages...)
	nodes = append(nodes, newNodes...)

	newMessages, newNodes = makeSpotInterruptionMessagesAndNodes(count / 3)
	messages = append(messages, newMessages...)
	nodes = append(nodes, newNodes...)

	newMessages, newNodes = makeStateChangeMessagesAndNodes(count-len(messages), []string{
		"stopping", "stopped", "shutting-down", "terminated",
	})
	messages = append(messages, newMessages...)
	nodes = append(nodes, newNodes...)

	return messages, nodes
}

func makeScheduledChangeMessagesAndNodes(count int) ([]any, []*corev1.Node) {
	var msgs []any
	var nodes []*corev1.Node
	for i := 0; i < count; i++ {
		instanceID := fake.InstanceID()
		msgs = append(msgs, scheduledChangeMessage(instanceID))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeStateChangeMessagesAndNodes(count int, states []string) ([]any, []*corev1.Node) {
	var msgs []any
	var nodes []*corev1.Node
	for i := 0; i < count; i++ {
		state := states[r.Intn(len(states))]
		instanceID := fake.InstanceID()
		msgs = append(msgs, stateChangeMessage(instanceID, state))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeSpotInterruptionMessagesAndNodes(count int) ([]any, []*corev1.Node) {
	var msgs []any
	var nodes []*corev1.Node
	for i := 0; i < count; i++ {
		instanceID := fake.InstanceID()
		msgs = append(msgs, spotInterruptionMessage(instanceID))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}
