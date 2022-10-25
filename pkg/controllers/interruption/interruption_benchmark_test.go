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
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/patrickmn/go-cache"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	awscache "github.com/aws/karpenter/pkg/cache"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/providers"
	"github.com/aws/karpenter/pkg/events"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider/fake"

	awsfake "github.com/aws/karpenter/pkg/fake"

	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/test"
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
	opts := options.Options{
		AWSIsolatedVPC: true,
		ClusterName:    "karpenter-notification-benchmarking",
	}
	fakeClock = &clock.FakeClock{}
	ctx = injection.WithOptions(context.Background(), opts)
	env = test.NewEnvironment(ctx, func(e *test.Environment) {})
	if err := env.Start(); err != nil {
		b.Fatalf("Starting envirionment, %v", err)
	}
	// Stop the test environment after the test completes
	defer func() {
		if err := retry.Do(func() error {
			return env.Stop()
		}); err != nil {
			b.Fatalf("stopping test environment, %v", err)
		}
	}()

	providers := newProviders(env.Ctx)
	if err := providers.makeInfrastructure(env.Ctx); err != nil {
		b.Fatalf("standing up infrastructure, %v", err)
	}
	// Cleanup the infrastructure after the test completes
	defer func() {
		if err := retry.Do(func() error {
			return providers.cleanupInfrastructure(env.Ctx)
		}); err != nil {
			b.Fatalf("deleting infrastructure, %v", err)
		}
	}()

	// Load all the fundamental components before setting up the controllers
	eventRecorder := awsfake.NewEventRecorder()
	recorder = events.NewRecorder(eventRecorder)
	cluster = state.NewCluster(ctx, fakeClock, env.Client, cloudProvider)
	cloudProvider = &fake.CloudProvider{}

	unavailableOfferingsCache = awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, awscontext.CacheCleanupInterval))

	// Set-up the controllers
	nodeStateController = state.NewNodeController(env.Client, cluster)
	interruptionController := interruption.NewController(env.Client, fakeClock, recorder, cluster, providers.sqsProvider, unavailableOfferingsCache)

	messages, nodes := makeDiverseMessagesAndNodes(messageCount)

	logging.FromContext(env.Ctx).Infof("Provisioning %d nodes", messageCount)
	if err := provisionNodes(env.Ctx, env.Client, nodes, nodeStateController); err != nil {
		b.Fatalf("provisioning nodes, %v", err)
	}
	logging.FromContext(env.Ctx).Infof("Completed provisioning %d nodes", messageCount)

	logging.FromContext(env.Ctx).Infof("Provisioning %d messages into the SQS Queue", messageCount)
	if err := providers.provisionMessages(env.Ctx, messages...); err != nil {
		b.Fatalf("provisioning messages, %v", err)
	}
	logging.FromContext(env.Ctx).Infof("Completed provisioning %d messages into the SQS Queue", messageCount)

	m, err := controllerruntime.NewManager(env.Config, controllerruntime.Options{
		BaseContext: func() context.Context { return logging.WithLogger(env.Ctx, zap.NewNop().Sugar()) },
	})
	if err != nil {
		b.Fatalf("creating manager, %v", err)
	}

	// Registering controller with the manager
	if err = interruptionController.Builder(ctx, m).Complete(interruptionController); err != nil {
		b.Fatalf("registering interruption controller, %v", err)
	}
	if err = nodeStateController.Builder(ctx, m).Complete(nodeStateController); err != nil {
		b.Fatalf("registering nodeState controller, %v", err)
	}

	b.ResetTimer()
	start := time.Now()
	managerErr := make(chan error)
	go func() {
		logging.FromContext(env.Ctx).Infof("Starting controller manager")
		managerErr <- m.Start(env.Ctx)
	}()

	select {
	case <-providers.monitorMessagesProcessed(env.Ctx, eventRecorder, messageCount):
	case err = <-managerErr:
		b.Fatalf("running manager, %v", err)
	}

	duration := time.Since(start)
	b.ReportMetric(float64(messageCount), "Messages")
	b.ReportMetric(duration.Seconds(), "TotalDurationInSeconds")
	b.ReportMetric(float64(messageCount)/duration.Seconds(), "Messages/Second")
}

type providerSet struct {
	sqsProvider         *providers.SQS
	eventBridgeProvider *providers.EventBridge
}

func newProviders(ctx context.Context) providerSet {
	sess := session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			awsclient.DefaultRetryer{NumMaxRetries: awsclient.DefaultRetryerMaxNumRetries},
		),
	))
	sqsProvider = providers.NewSQS(ctx, sqs.New(sess))
	eventBridgeProvider = providers.NewEventBridge(eventbridge.New(sess), sqsProvider)
	return providerSet{
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

func (p *providerSet) makeInfrastructure(ctx context.Context) error {
	infraProvider := providers.NewInfrastructure(p.sqsProvider, p.eventBridgeProvider)
	if err := infraProvider.Create(ctx); err != nil {
		return fmt.Errorf("creating infrastructure, %w", err)
	}
	if err := p.sqsProvider.SetQueueAttributes(ctx, map[string]*string{
		sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("1200"), // 20 minutes for this test
	}); err != nil {
		return fmt.Errorf("updating message retention period, %w", err)
	}
	return nil
}

func (p *providerSet) cleanupInfrastructure(ctx context.Context) error {
	infraProvider := providers.NewInfrastructure(p.sqsProvider, p.eventBridgeProvider)
	return infraProvider.Delete(ctx)
}

func (p *providerSet) provisionMessages(ctx context.Context, messages ...interface{}) error {
	errs := make([]error, len(messages))
	workqueue.ParallelizeUntil(ctx, 20, len(messages), func(i int) {
		_, err := p.sqsProvider.SendMessage(ctx, messages[i])
		errs[i] = err
	})
	return multierr.Combine(errs...)
}

func (p *providerSet) monitorMessagesProcessed(ctx context.Context, eventRecorder *awsfake.EventRecorder, expectedProcessed int) <-chan struct{} {
	done := make(chan struct{})
	totalProcessed := 0
	go func() {
		for totalProcessed < expectedProcessed {
			totalProcessed = int(eventRecorder.InstanceStoppingCalled.Load()) +
				int(eventRecorder.InstanceTerminatingCalled.Load()) +
				int(eventRecorder.InstanceUnhealthyCalled.Load()) +
				int(eventRecorder.InstanceRebalanceRecommendationCalled.Load()) +
				int(eventRecorder.InstanceSpotInterruptedCalled.Load())
			logging.FromContext(ctx).Infof("Processed %d messages from the queue", totalProcessed)
			time.Sleep(time.Second)
		}
		close(done)
	}()
	return done
}

func provisionNodes(ctx context.Context, kubeClient client.Client, nodes []*v1.Node, nodeController *state.NodeController) error {
	errs := make([]error, len(nodes))
	workqueue.ParallelizeUntil(ctx, 20, len(nodes), func(i int) {
		if err := retry.Do(func() error {
			return kubeClient.Create(ctx, nodes[i])
		}); err != nil {
			errs[i] = fmt.Errorf("provisioning node, %w", err)
		}
		if err := retry.Do(func() error {
			_, err := nodeController.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(nodes[i])})
			return err
		}); err != nil {
			errs[i] = fmt.Errorf("reconciling node, %w", err)
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
		instanceID := makeInstanceID()
		msgs = append(msgs, scheduledChangeMessage(instanceID))
		nodes = append(nodes, test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeStateChangeMessagesAndNodes(count int, states []string) ([]interface{}, []*v1.Node) {
	var msgs []interface{}
	var nodes []*v1.Node
	for i := 0; i < count; i++ {
		state := states[r.Intn(len(states))]
		instanceID := makeInstanceID()
		msgs = append(msgs, stateChangeMessage(instanceID, state))
		nodes = append(nodes, test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(instanceID),
		}))
	}
	return msgs, nodes
}

func makeSpotInterruptionMessagesAndNodes(count int) ([]interface{}, []*v1.Node) {
	var msgs []interface{}
	var nodes []*v1.Node
	for i := 0; i < count; i++ {
		instanceID := makeInstanceID()
		msgs = append(msgs, spotInterruptionMessage(instanceID))
		nodes = append(nodes, test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(instanceID),
		}))
	}
	return msgs, nodes
}
