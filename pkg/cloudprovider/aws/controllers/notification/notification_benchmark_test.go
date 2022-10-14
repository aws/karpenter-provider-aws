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
package notification_test

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
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
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

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	awscloudprovider "github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	awsfake "github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/operator"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
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

func benchmarkNotificationController(b *testing.B, messageCount int) {
	opts := options.Options{
		AWSIsolatedVPC: true,
		ClusterName:    "karpenter-notification-benchmarking",
	}
	fakeClock := &clock.FakeClock{}
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
	recorder := awsfake.NewEventRecorder()
	cfg = test.NewConfig()
	cluster := state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
	cloudProvider = &fake.CloudProvider{}
	ec2api = &awsfake.EC2API{}
	subnetProvider := awscloudprovider.NewSubnetProvider(ec2api)
	instanceTypeProvider = awscloudprovider.NewInstanceTypeProvider(env.Ctx, mock.Session, cloudprovider.Options{}, ec2api, subnetProvider)

	// Set-up the controllers
	nodeStateController := state.NewNodeController(env.Client, cluster)
	notificationController := notification.NewController(env.Client, fakeClock, recorder, cluster, providers.sqsProvider, instanceTypeProvider)

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
	m = operator.RegisterControllers(env.Ctx, m, notificationController, nodeStateController)

	managerErr := make(chan error)
	go func() {
		logging.FromContext(env.Ctx).Infof("Starting controller manager")
		if err := m.Start(env.Ctx); err != nil {
			managerErr <- err
		}
	}()

	b.ResetTimer()
	start := time.Now()

	notificationController.Start(env.Ctx)
	done := providers.monitorMessagesProcessed(env.Ctx, recorder, messageCount)

	select {
	case err := <-managerErr:
		b.Fatalf("starting manager, %v", err)
	case <-done:
	}
	duration := time.Since(start)
	b.ReportMetric(float64(messageCount), "Messages")
	b.ReportMetric(duration.Seconds(), "TotalDurationInSeconds")
	b.ReportMetric(float64(messageCount)/duration.Seconds(), "Messages/Second")
}

type providers struct {
	sqsProvider         *awscloudprovider.SQSProvider
	eventBridgeProvider *awscloudprovider.EventBridgeProvider
}

func newProviders(ctx context.Context) providers {
	sess := session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			awsclient.DefaultRetryer{NumMaxRetries: awsclient.DefaultRetryerMaxNumRetries},
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
	if err := infraProvider.CreateInfrastructure(ctx); err != nil {
		return fmt.Errorf("creating infrastructure, %w", err)
	}
	if err := p.sqsProvider.SetQueueAttributes(ctx, map[string]*string{
		sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("1200"), // 20 minutes for this test
	}); err != nil {
		return fmt.Errorf("updating message retention period, %w", err)
	}
	return nil
}

func (p *providers) cleanupInfrastructure(ctx context.Context) error {
	infraProvider := infrastructure.NewProvider(p.sqsProvider, p.eventBridgeProvider)
	return infraProvider.DeleteInfrastructure(ctx)
}

func (p *providers) provisionMessages(ctx context.Context, messages ...interface{}) error {
	errs := make([]error, len(messages))
	workqueue.ParallelizeUntil(ctx, 20, len(messages), func(i int) {
		_, err := p.sqsProvider.SendMessage(ctx, messages[i])
		errs[i] = err
	})
	return multierr.Combine(errs...)
}

func (p *providers) monitorMessagesProcessed(ctx context.Context, recorder *awsfake.EventRecorder, expectedProcessed int) <-chan struct{} {
	done := make(chan struct{})
	totalProcessed := 0
	go func() {
		for totalProcessed < expectedProcessed {
			totalProcessed = int(recorder.EC2StateStoppingCalled.Load()) +
				int(recorder.EC2StateTerminatingCalled.Load()) +
				int(recorder.EC2HealthWarningCalled.Load()) +
				int(recorder.EC2SpotRebalanceRecommendationCalled.Load()) +
				int(recorder.EC2SpotInterruptionWarningCalled.Load())
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
