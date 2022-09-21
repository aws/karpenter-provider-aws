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

package notification_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"

	. "github.com/aws/karpenter/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	awsfake "github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var sqsapi *awsfake.SQSAPI
var cloudProvider *fake.CloudProvider
var sqsProvider *aws.SQSProvider
var recorder *awsfake.EventRecorder
var fakeClock *clock.FakeClock
var cfg *test.Config
var controller *notification.Controller
var ready func() <-chan struct{}

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Notification")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cfg = test.NewConfig()
		fakeClock = clock.NewFakeClock(time.Now())
		cloudProvider = &fake.CloudProvider{}
		cluster = state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
		recorder = awsfake.NewEventRecorder()
		metadata := aws.NewMetadata("us-east-1", "000000000000")

		sqsapi = &awsfake.SQSAPI{}
		sqsProvider = aws.NewSQSProvider(ctx, sqsapi, metadata)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	sqsapi.Reset()
	ready = func() <-chan struct{} { return make(chan struct{}) }
	controller = notification.NewController(env.Ctx, env.Client, fakeClock, recorder, cluster, sqsProvider, nil, ready)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})
