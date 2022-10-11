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

package infrastructure_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/controllers/polling"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	awsfake "github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *test.Environment
var sqsapi *awsfake.SQSAPI
var sqsProvider *aws.SQSProvider
var eventbridgeapi *awsfake.EventBridgeAPI
var eventBridgeProvider *aws.EventBridgeProvider
var controller *polling.ControllerWithHealth
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
	RunSpecs(t, "AWS Notification")
}

var _ = BeforeEach(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		opts = defaultOpts
		Expect(opts.Validate()).To(Succeed(), "Failed to validate options")
		e.Ctx = injection.WithOptions(e.Ctx, opts)

		sqsapi = &awsfake.SQSAPI{}
		eventbridgeapi = &awsfake.EventBridgeAPI{}
		sqsProvider = aws.NewSQSProvider(e.Ctx, sqsapi)
		eventBridgeProvider = aws.NewEventBridgeProvider(eventbridgeapi, sqsProvider)

		controller = polling.NewController(infrastructure.NewReconciler(infrastructure.NewProvider(sqsProvider, eventBridgeProvider))).WithHealth()
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Reconciliation", func() {
	It("should reconcile the queue and the eventbridge rules on start", func() {
		sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(1)) // This mocks the queue not existing

		_, err := controller.Reconcile(ctx, reconcile.Request{})
		Expect(err).To(Succeed())

		Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
		Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
		Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))
	})
	It("should throw an error but wait with backoff if we get AccessDenied", func() {
		sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0)) // This mocks the queue not existing
		sqsapi.CreateQueueBehavior.Error.Set(awsErrWithCode(aws.AccessDeniedCode), awsfake.MaxCalls(0))
		eventbridgeapi.PutRuleBehavior.Error.Set(awsErrWithCode(aws.AccessDeniedExceptionCode), awsfake.MaxCalls(0))
		eventbridgeapi.PutTargetsBehavior.Error.Set(awsErrWithCode(aws.AccessDeniedExceptionCode), awsfake.MaxCalls(0))

		_, err := controller.Reconcile(ctx, reconcile.Request{})
		Expect(err).ToNot(Succeed())
		Expect(sqsapi.CreateQueueBehavior.FailedCalls()).To(Equal(1))

		// Simulating AccessDenied being resolved
		sqsapi.CreateQueueBehavior.Reset()
		eventbridgeapi.PutRuleBehavior.Reset()
		eventbridgeapi.PutTargetsBehavior.Reset()

		_, err = controller.Reconcile(ctx, reconcile.Request{})
		Expect(err).To(Succeed())
		Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
		Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
		Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))
	})
	It("should throw an error and wait with backoff if we get QueueDeletedRecently", func() {
		sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0)) // This mocks the queue not existing
		sqsapi.CreateQueueBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDeletedRecently), awsfake.MaxCalls(0))

		_, err := controller.Reconcile(ctx, reconcile.Request{})
		Expect(err).ToNot(Succeed())
		Expect(sqsapi.CreateQueueBehavior.FailedCalls()).To(Equal(1))
	})
})

// TODO: Fix the Cleanup tests
//var _ = Describe("Cleanup", func() {
//	It("should cleanup the infrastructure when the cleanup channel is triggered", func() {
//		ExpectDeleted(env.Ctx, env.Client, test.KarpenterDeployment())
//		ExpectClosed(cleanupChan)
//		Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
//		Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(4))
//		Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(4))
//	})
//	It("should cleanup when queue is already deleted", func() {
//		ExpectDeleted(env.Ctx, env.Client, test.KarpenterDeployment())
//		sqsapi.DeleteQueueBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist))
//		ExpectClosed(cleanupChan)
//
//		// Test that we cleanup in a reasonable amount of time with a DoesNotExist error
//		select {
//		case <-time.After(time.Second * 2):
//			Fail("controller should have completed cleanup in time")
//		case <-controller.Done():
//		}
//		Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(0))
//		Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(4))
//		Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(4))
//	})
//	It("should cleanup when a single rule is already deleted", func() {
//		ExpectDeleted(env.Ctx, env.Client, test.KarpenterDeployment())
//		eventbridgeapi.RemoveTargetsBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()))
//		eventbridgeapi.DeleteRuleBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()))
//		close(cleanupChan)
//
//		// Test that we cleanup in a reasonable amount of time with a DoesNotExist error
//		select {
//		case <-time.After(time.Second * 5):
//			Fail("controller should have completed cleanup in time")
//		case <-controller.Done():
//		}
//		Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
//		Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(3))
//		Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(3))
//	})
//	It("should cleanup when all rule targets and rules are already deleted", func() {
//		ExpectDeleted(env.Ctx, env.Client, test.KarpenterDeployment())
//		eventbridgeapi.RemoveTargetsBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()), awsfake.MaxCalls(0))
//		eventbridgeapi.DeleteRuleBehavior.Error.Set(awsErrWithCode((&eventbridge.ResourceNotFoundException{}).Code()), awsfake.MaxCalls(0))
//		close(cleanupChan)
//
//		// Test that we cleanup in a reasonable amount of time with a DoesNotExist error
//		select {
//		case <-time.After(time.Second * 2):
//			Fail("controller should have completed cleanup in time")
//		case <-controller.Done():
//		}
//		Expect(sqsapi.DeleteQueueBehavior.SuccessfulCalls()).To(Equal(1))
//		Expect(eventbridgeapi.RemoveTargetsBehavior.SuccessfulCalls()).To(Equal(0))
//		Expect(eventbridgeapi.DeleteRuleBehavior.SuccessfulCalls()).To(Equal(0))
//	})
//})

func awsErrWithCode(code string) awserr.Error {
	return awserr.New(code, "", fmt.Errorf(""))
}
