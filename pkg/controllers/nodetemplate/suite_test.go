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

package nodetemplate_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/providers"
	"github.com/aws/karpenter/pkg/errors"

	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	awsfake "github.com/aws/karpenter/pkg/fake"
	awstest "github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *test.Environment
var sqsapi *awsfake.SQSAPI
var sqsProvider *providers.SQS
var eventbridgeapi *awsfake.EventBridgeAPI
var eventBridgeProvider *providers.EventBridge
var controller *nodetemplate.Controller
var opts options.Options

var defaultOpts = options.Options{
	ClusterName:               "test-cluster",
	ClusterEndpoint:           "https://test-cluster",
	AWSNodeNameConvention:     string(options.IPName),
	AWSENILimitedPodDensity:   true,
	AWSEnablePodENI:           true,
	AWSDefaultInstanceProfile: "test-instance-profile",
}

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Node Template")
}

var _ = BeforeEach(func() {
	settingsStore := test.SettingsStore{
		settings.ContextKey: test.Settings(),
		awssettings.ContextKey: awssettings.Settings{
			EnableInterruptionHandling: true,
		},
	}
	ctx = settingsStore.InjectSettings(ctx)
	ctx = injection.WithOptions(ctx, defaultOpts)
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		opts = defaultOpts
		Expect(opts.Validate()).To(Succeed(), "Failed to validate options")
		e.Ctx = injection.WithOptions(e.Ctx, opts)

		sqsapi = &awsfake.SQSAPI{}
		eventbridgeapi = &awsfake.EventBridgeAPI{}
		sqsProvider = providers.NewSQS(sqsapi)
		eventBridgeProvider = providers.NewEventBridge(eventbridgeapi, sqsProvider)

		controller = nodetemplate.NewController(e.Client, sqsProvider, eventBridgeProvider)
	})
	env.CRDDirectoryPaths = append(env.CRDDirectoryPaths, relativeToRoot("charts/karpenter/crds"))
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Infrastructure", func() {
	Context("Creation", func() {
		var provider *v1alpha1.AWSNodeTemplate
		BeforeEach(func() {
			provider = awstest.AWSNodeTemplate()
			ExpectApplied(env.Ctx, env.Client, provider)
		})
		AfterEach(func() {
			ExpectFinalizersRemoved(env.Ctx, env.Client, provider)
			ExpectDeleted(env.Ctx, env.Client, provider)
		})
		It("should reconcile the queue and the eventbridge rules on start", func() {
			sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(1)) // This mocks the queue not existing

			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should throw an error but wait with backoff if we get AccessDenied", func() {
			sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0)) // This mocks the queue not existing
			sqsapi.CreateQueueBehavior.Error.Set(awsErrWithCode(errors.AccessDeniedCode), awsfake.MaxCalls(0))
			eventbridgeapi.PutRuleBehavior.Error.Set(awsErrWithCode(errors.AccessDeniedExceptionCode), awsfake.MaxCalls(0))
			eventbridgeapi.PutTargetsBehavior.Error.Set(awsErrWithCode(errors.AccessDeniedExceptionCode), awsfake.MaxCalls(0))

			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(provider))
			Expect(sqsapi.CreateQueueBehavior.FailedCalls()).To(Equal(1))

			// Simulating AccessDenied being resolved
			sqsapi.CreateQueueBehavior.Reset()
			eventbridgeapi.PutRuleBehavior.Reset()
			eventbridgeapi.PutTargetsBehavior.Reset()

			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))
			Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should throw an error and wait with backoff if we get QueueDeletedRecently", func() {
			sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0)) // This mocks the queue not existing
			sqsapi.CreateQueueBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDeletedRecently), awsfake.MaxCalls(0))

			ExpectReconcileFailed(ctx, controller, client.ObjectKeyFromObject(provider))
			Expect(sqsapi.CreateQueueBehavior.FailedCalls()).To(Equal(1))
		})
	})
	Context("Deletion", func() {
		It("should cleanup the infrastructure when the last AWSNodeTemplate is removed", func() {
			provider := awstest.AWSNodeTemplate()
			sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(1)) // This mocks the queue not existing

			ExpectApplied(ctx, env.Client, provider)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))

			// Set the output of ListRules to mock rule creation
			eventbridgeapi.ListRulesBehavior.Output.Set(&eventbridge.ListRulesOutput{
				Rules: []*eventbridge.Rule{
					{
						Name: aws.String(providers.DefaultRules[providers.ScheduledChangedRule].Name),
						Arn:  aws.String("test-arn1"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.SpotTerminationRule].Name),
						Arn:  aws.String("test-arn2"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.RebalanceRule].Name),
						Arn:  aws.String("test-arn3"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.StateChangeRule].Name),
						Arn:  aws.String("test-arn4"),
					},
				},
			})
			eventbridgeapi.ListTagsForResourceBehavior.Output.Set(&eventbridge.ListTagsForResourceOutput{
				Tags: []*eventbridge.Tag{
					{
						Key:   aws.String(v1alpha5.DiscoveryTagKey),
						Value: aws.String(defaultOpts.ClusterName),
					},
				},
			})

			// Delete the AWSNodeTemplate and then re-reconcile it to delete the infrastructure
			Expect(env.Client.Delete(ctx, provider)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should cleanup when queue is already deleted", func() {
			provider := awstest.AWSNodeTemplate()
			ExpectApplied(ctx, env.Client, provider)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			sqsapi.DeleteQueueBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0))

			// Set the output of ListRules to mock rule creation
			eventbridgeapi.ListRulesBehavior.Output.Set(&eventbridge.ListRulesOutput{
				Rules: []*eventbridge.Rule{
					{
						Name: aws.String(providers.DefaultRules[providers.ScheduledChangedRule].Name),
						Arn:  aws.String("test-arn1"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.SpotTerminationRule].Name),
						Arn:  aws.String("test-arn2"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.RebalanceRule].Name),
						Arn:  aws.String("test-arn3"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.StateChangeRule].Name),
						Arn:  aws.String("test-arn4"),
					},
				},
			})
			eventbridgeapi.ListTagsForResourceBehavior.Output.Set(&eventbridge.ListTagsForResourceOutput{
				Tags: []*eventbridge.Tag{
					{
						Key:   aws.String(v1alpha5.DiscoveryTagKey),
						Value: aws.String(defaultOpts.ClusterName),
					},
				},
			})

			// Delete the AWSNodeTemplate and then re-reconcile it to delete the infrastructure
			Expect(env.Client.Delete(ctx, provider)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(0))
			Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should cleanup with a success when a few rules aren't in list call", func() {
			provider := awstest.AWSNodeTemplate()
			ExpectApplied(ctx, env.Client, provider)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			// Set the output of ListRules to mock rule creation
			eventbridgeapi.ListRulesBehavior.Output.Set(&eventbridge.ListRulesOutput{
				Rules: []*eventbridge.Rule{
					{
						Name: aws.String(providers.DefaultRules[providers.ScheduledChangedRule].Name),
						Arn:  aws.String("test-arn1"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.SpotTerminationRule].Name),
						Arn:  aws.String("test-arn2"),
					},
				},
			})
			eventbridgeapi.ListTagsForResourceBehavior.Output.Set(&eventbridge.ListTagsForResourceOutput{
				Tags: []*eventbridge.Tag{
					{
						Key:   aws.String(v1alpha5.DiscoveryTagKey),
						Value: aws.String(defaultOpts.ClusterName),
					},
				},
			})

			// Delete the AWSNodeTemplate and then re-reconcile it to delete the infrastructure
			Expect(env.Client.Delete(ctx, provider)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(2))
			Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(2))
		})
		It("should cleanup with a success when getting not found errors", func() {
			provider := awstest.AWSNodeTemplate()
			ExpectApplied(ctx, env.Client, provider)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			eventbridgeapi.RemoveTargetsBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()), awsfake.MaxCalls(0))
			eventbridgeapi.DeleteRuleBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()), awsfake.MaxCalls(0))

			// Delete the AWSNodeTemplate and then re-reconcile it to delete the infrastructure
			Expect(env.Client.Delete(ctx, provider)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provider))

			Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(0))
			Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(0))
		})
		It("should only attempt to delete the infrastructure when the last node template is removed", func() {
			var nodeTemplates []*v1alpha1.AWSNodeTemplate
			for i := 0; i < 10; i++ {
				p := awstest.AWSNodeTemplate()
				nodeTemplates = append(nodeTemplates, p)
				ExpectApplied(ctx, env.Client, p)
				ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(p))
			}

			for i := 0; i < len(nodeTemplates)-1; i++ {
				Expect(env.Client.Delete(ctx, nodeTemplates[i])).To(Succeed())
				ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplates[i]))
			}

			// It shouldn't attempt to delete at this point
			Expect(sqsapi.DeleteQueueBehavior.Calls()).To(Equal(0))
			Expect(eventbridgeapi.RemoveTargetsBehavior.Calls()).To(Equal(0))
			Expect(eventbridgeapi.DeleteRuleBehavior.Calls()).To(Equal(0))

			// Set the output of ListRules to mock rule creation
			eventbridgeapi.ListRulesBehavior.Output.Set(&eventbridge.ListRulesOutput{
				Rules: []*eventbridge.Rule{
					{
						Name: aws.String(providers.DefaultRules[providers.ScheduledChangedRule].Name),
						Arn:  aws.String("test-arn1"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.SpotTerminationRule].Name),
						Arn:  aws.String("test-arn2"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.RebalanceRule].Name),
						Arn:  aws.String("test-arn3"),
					},
					{
						Name: aws.String(providers.DefaultRules[providers.StateChangeRule].Name),
						Arn:  aws.String("test-arn4"),
					},
				},
			})
			eventbridgeapi.ListTagsForResourceBehavior.Output.Set(&eventbridge.ListTagsForResourceOutput{
				Tags: []*eventbridge.Tag{
					{
						Key:   aws.String(v1alpha5.DiscoveryTagKey),
						Value: aws.String(defaultOpts.ClusterName),
					},
				},
			})

			// Last AWSNodeTemplate, so now it should delete it
			Expect(env.Client.Delete(ctx, nodeTemplates[len(nodeTemplates)-1])).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplates[len(nodeTemplates)-1]))

			Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(4))
			Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(4))
		})
	})
})

func awsErrWithCode(code string) awserr.Error {
	return awserr.New(code, "", fmt.Errorf(""))
}

func relativeToRoot(path string) string {
	_, file, _, _ := runtime.Caller(0)
	manifestsRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	return filepath.Join(manifestsRoot, path)
}
