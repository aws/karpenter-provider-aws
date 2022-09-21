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

package deployment_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var provisioner *provisioning.Provisioner
var cloudProvider *fake.CloudProvider
var clientSet *kubernetes.Clientset
var recorder *test.EventRecorder
var fakeClock *clock.FakeClock
var cfg *test.Config

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Infrastructure")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider = &fake.CloudProvider{}
		cfg = test.NewConfig()
		fakeClock = clock.NewFakeClock(time.Now())
		cluster = state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
		clientSet = kubernetes.NewForConfigOrDie(e.Config)
		recorder = test.NewEventRecorder()
		provisioner = provisioning.NewProvisioner(ctx, cfg, env.Client, clientSet.CoreV1(), recorder, cloudProvider, cluster)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
})
var _ = AfterEach(func() {
})
