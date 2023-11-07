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
	"github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	servicesqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter/pkg/apis/settings"
	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/interruption/events"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/sqs"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	coretest "github.com/aws/karpenter-core/pkg/test"
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
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("message-count", messageCount))
	fakeClock = &clock.FakeClock{}
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		ClusterName:       lo.ToPtr("karpenter-notification-benchmarking"),
		IsolatedVPC:       lo.ToPtr(true),
		InterruptionQueue: lo.ToPtr("test-cluster"),
	}))
	ctx = settings.ToContext(ctx, test.Settings())
	env = coretest.NewEnvironment(scheme.Scheme)
	// Stop the coretest environment after the coretest completes
	defer func() {
		if err := retry.Do(func() error {
			return env.Stop()
		}); err != nil {
			b.Fatalf("stopping coretest environment, %v", err)
		}
	}()

	providers := newProviders(env.Context, env.Client)
	if err := providers.makeInfrastructure(ctx); err != nil {
		b.Fatalf("standing up infrastructure, %v", err)
	}
	// Cleanup the infrastructure after the coretest completes
	defer func() {
		if err := retry.Do(func() error {
			return providers.cleanupInfrastructure(ctx)
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
	logging.FromContext(ctx).Infof("provisioning nodes")
	if err := provisionNodes(ctx, env.Client, nodes); err != nil {
		b.Fatalf("provisioning nodes, %v", err)
	}
	logging.FromContext(ctx).Infof("completed provisioning nodes")

	logging.FromContext(ctx).Infof("provisioning messages into the SQS Queue")
	if err := providers.provisionMessages(ctx, messages...); err != nil {
		b.Fatalf("provisioning messages, %v", err)
	}
	logging.FromContext(ctx).Infof("completed provisioning messages into the SQS Queue")

	m, err := controllerruntime.NewManager(env.Config, controllerruntime.Options{
		BaseContext: func() context.Context { return logging.WithLogger(ctx, zap.NewNop().Sugar()) },
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
		logging.FromContext(ctx).Infof("starting controller manager")
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
	sqsAPI      *servicesqs.SQS
	sqsProvider *sqs.Provider
}

func newProviders(ctx context.Context, kubeClient client.Client) providerSet {
	sess := session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			awsclient.DefaultRetryer{NumMaxRetries: awsclient.DefaultRetryerMaxNumRetries},
		),
	))
	sqsAPI := servicesqs.New(sess)
	return providerSet{
		kubeClient:  kubeClient,
		sqsAPI:      sqsAPI,
		sqsProvider: sqs.NewProvider(ctx, sqsAPI, "test-cluster"),
	}
}

func (p *providerSet) makeInfrastructure(ctx context.Context) error {
	if _, err := p.sqsAPI.CreateQueueWithContext(ctx, &servicesqs.CreateQueueInput{
		QueueName: lo.ToPtr(options.FromContext(ctx).InterruptionQueueName),
		Attributes: map[string]*string{
			servicesqs.QueueAttributeNameMessageRetentionPeriod: aws.String("1200"), // 20 minutes for this test
		},
	}); err != nil {
		return fmt.Errorf("creating servicesqs queue, %w", err)
	}
	return nil
}

func (p *providerSet) cleanupInfrastructure(ctx context.Context) error {
	queueURL, err := p.sqsProvider.DiscoverQueueURL(ctx)
	if err != nil {
		return fmt.Errorf("discovering queue url for deletion, %w", err)
	}
	if _, err = p.sqsAPI.DeleteQueueWithContext(ctx, &servicesqs.DeleteQueueInput{
		QueueUrl: lo.ToPtr(queueURL),
	}); err != nil {
		return fmt.Errorf("deleting servicesqs queue, %w", err)
	}
	return nil
}

func (p *providerSet) provisionMessages(ctx context.Context, messages ...interface{}) error {
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
			totalProcessed = eventRecorder.Calls(events.InstanceStopping(coretest.Node()).Reason) +
				eventRecorder.Calls(events.InstanceTerminating(coretest.Node()).Reason) +
				eventRecorder.Calls(events.InstanceUnhealthy(coretest.Node()).Reason) +
				eventRecorder.Calls(events.InstanceRebalanceRecommendation(coretest.Node()).Reason) +
				eventRecorder.Calls(events.InstanceSpotInterrupted(coretest.Node()).Reason)
			logging.FromContext(ctx).With("processed-message-count", totalProcessed).Infof("processed messages from the queue")
			time.Sleep(time.Second)
		}
		close(done)
	}()
	return done
}

func provisionNodes(ctx context.Context, kubeClient client.Client, nodes []*v1.Node) error {
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

func makeDiverseMessagesAndNodes(count int) ([]interface{}, []*v1.Node) {
	var messages []interface{}
	var nodes []*v1.Node

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

func makeScheduledChangeMessagesAndNodes(count int) ([]interface{}, []*v1.Node) {
	var msgs []interface{}
	var nodes []*v1.Node
	for i := 0; i < count; i++ {
		instanceID := fake.InstanceID()
		msgs = append(msgs, scheduledChangeMessage(instanceID))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeStateChangeMessagesAndNodes(count int, states []string) ([]interface{}, []*v1.Node) {
	var msgs []interface{}
	var nodes []*v1.Node
	for i := 0; i < count; i++ {
		state := states[r.Intn(len(states))]
		instanceID := fake.InstanceID()
		msgs = append(msgs, stateChangeMessage(instanceID, state))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeSpotInterruptionMessagesAndNodes(count int) ([]interface{}, []*v1.Node) {
	var msgs []interface{}
	var nodes []*v1.Node
	for i := 0; i < count; i++ {
		instanceID := fake.InstanceID()
		msgs = append(msgs, spotInterruptionMessage(instanceID))
		nodes = append(nodes, coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: fake.ProviderID(instanceID),
		}))
	}
	return msgs, nodes
}
