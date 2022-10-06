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

package disruption_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/cloudprovider"
	. "github.com/aws/karpenter/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/disruption"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *test.Environment
var cloudProvider *fake.CloudProvider
var fakeClock *clock.FakeClock
var start chan struct{}

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Disruption")
}

var _ = BeforeEach(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		start = make(chan struct{})
		cloudProvider = fake.NewCloudProvider()
		fakeClock = clock.NewFakeClock(time.Now())
		controller := disruption.NewController(env.Ctx, fakeClock, env.Client, start)
		controller.RegisterWatcher(cloudProvider.NodeEventWatcher())
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterEach(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Disruption", func() {
	It("should process a delete node message", func() {
		close(start)
		node := test.Node()
		ExpectApplied(env.Ctx, env.Client, node)
		cloudProvider.EnqueueDeleteEvent("test", client.ObjectKeyFromObject(node), cloudprovider.NoOp)
		EventuallyExpectNotFound(env.Ctx, env.Client, node)
	})
	It("should process messages, add new messages, and process those", func() {
		close(start)

		var nodes []*v1.Node
		for i := 0; i < 10; i++ {
			nodes = append(nodes, test.Node())
		}
		objs := lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })

		ExpectApplied(env.Ctx, env.Client, objs...)
		for i := 0; i < 7; i++ {
			cloudProvider.EnqueueDeleteEvent("test", client.ObjectKeyFromObject(nodes[i]), cloudprovider.NoOp)
		}
		EventuallyExpectNotFound(env.Ctx, env.Client, objs[:7]...)
		for i := 7; i < 10; i++ {
			cloudProvider.EnqueueDeleteEvent("test", client.ObjectKeyFromObject(nodes[i]), cloudprovider.NoOp)
		}
		EventuallyExpectNotFound(env.Ctx, env.Client, objs[7:]...)
	})
	It("should retry a complete operation on failure with backoff", func() {
		close(start)
		node := test.Node()
		succeeded := &atomic.Bool{}
		callCount := 0
		onComplete := func() error {
			callCount++
			if callCount > 1 {
				succeeded.Store(true)
				return nil
			}
			return fmt.Errorf("failed")
		}
		ExpectApplied(env.Ctx, env.Client, node)
		cloudProvider.EnqueueDeleteEvent("test", client.ObjectKeyFromObject(node), onComplete)
		EventuallyExpectNotFound(env.Ctx, env.Client, node) // node should get deleted despite on complete failure

		fakeClock.Step(time.Minute)
		Eventually(func(g Gomega) {
			g.Expect(succeeded.Load()).To(BeTrue())
		}).Should(Succeed())
	})
	It("should retry deletion of the node if initial calls fail", func() {
		fakeClient := newFakeClientWithErrs(env.Client)
		fakeClient.deleteErr.Store(true)

		// This actually creates two controllers since the other one isn't destroyed until its context is canceled
		controller := disruption.NewController(env.Ctx, fakeClock, fakeClient, start)
		controller.RegisterWatcher(cloudProvider.NodeEventWatcher())
		close(start)

		node := test.Node()
		ExpectApplied(env.Ctx, fakeClient, node)
		cloudProvider.EnqueueDeleteEvent("test", client.ObjectKeyFromObject(node), cloudprovider.NoOp)

		// Tell the fakeClient to not return an error after a fixed number of delete calls
		var once sync.Once
		Eventually(func(g Gomega) {
			if fakeClient.deleteCallCount.Load() > int32(2) {
				once.Do(func() {
					fakeClient.deleteErr.Store(false)
				})
			}
			fakeClock.Step(time.Second)
			g.Expect(errors.IsNotFound(fakeClient.Get(env.Ctx, client.ObjectKeyFromObject(node), node))).To(BeTrue())
		}, time.Second*5, time.Millisecond*500).Should(Succeed())
	})
})

type fakeClientWithErrs struct {
	client.Client

	deleteErr       atomic.Bool
	deleteCallCount atomic.Int32
}

func newFakeClientWithErrs(c client.Client) *fakeClientWithErrs {
	return &fakeClientWithErrs{
		Client: c,
	}
}

func (f *fakeClientWithErrs) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	f.deleteCallCount.Add(1)
	if f.deleteErr.Load() {
		return fmt.Errorf("failed")
	}
	return f.Client.Delete(ctx, obj, opts...)
}
