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

package config_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/karpenter/pkg/config"
	"github.com/aws/karpenter/pkg/test"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap/informer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *test.Environment
var clientSet *kubernetes.Clientset
var cfg config.Config
var finished func()

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	ctx, finished = context.WithCancel(ctx)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		clientSet = kubernetes.NewForConfigOrDie(e.Config)
		os.Setenv("SYSTEM_NAMESPACE", "default")

		var cm v1.ConfigMap
		cm.Namespace = "default"
		cm.Name = "karpenter-global-settings"
		ExpectApplied(ctx, env.Client, &cm)

		cmw := informer.NewInformedWatcher(clientSet, os.Getenv("SYSTEM_NAMESPACE"))
		var err error
		cfg, err = config.New(ctx, clientSet, cmw)
		Expect(err).To(BeNil())
		Expect(cmw.Start(ctx.Done())).To(Succeed())
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = BeforeEach(func() {
	var cm v1.ConfigMap
	cm.Namespace = "default"
	cm.Name = "karpenter-global-settings"
	env.Client.Delete(ctx, &cm)
	ExpectApplied(ctx, env.Client, &cm)
})

var _ = AfterSuite(func() {
	finished()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Batch Parameter", func() {
	It("should have default values", func() {
		Expect(cfg.BatchIdleDuration()).To(Equal(1 * time.Second))
		Expect(cfg.BatchMaxDuration()).To(Equal(10 * time.Second))
	})
	It("should update if values are changed", func() {
		Expect(cfg.BatchIdleDuration()).To(Equal(1 * time.Second))
		Expect(cfg.BatchMaxDuration()).To(Equal(10 * time.Second))
		var changed int64
		cfg.OnChange(func(c config.Config) {
			defer GinkgoRecover()
			if atomic.LoadInt64(&changed) == 0 {
				atomic.StoreInt64(&changed, 1)
				Expect(cfg.BatchIdleDuration()).To(Equal(2 * time.Second))
				Expect(cfg.BatchMaxDuration()).To(Equal(15 * time.Second))
			}
		})

		// simulate user updating the config map
		var cm v1.ConfigMap
		Expect(env.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: "karpenter-global-settings"}, &cm)).To(Succeed())
		cm.Data = map[string]string{}
		cm.Data["batchIdleDuration"] = "2s"
		cm.Data["batchMaxDuration"] = "15s"
		env.Client.Update(ctx, &cm)

		// we should read the new changes
		Eventually(func() bool {
			return cfg.BatchIdleDuration() == 2*time.Second
		}).Should(BeTrue())
		Eventually(func() bool {
			return cfg.BatchMaxDuration() == 15*time.Second
		}).Should(BeTrue())

		// and have been notified
		Eventually(func() bool {
			return atomic.LoadInt64(&changed) == 1
		}).Should(BeTrue())
	})
	It("should use default values if config map has invalid data", func() {
		Expect(cfg.BatchIdleDuration()).To(Equal(1 * time.Second))
		Expect(cfg.BatchMaxDuration()).To(Equal(10 * time.Second))
		var changed int64
		cfg.OnChange(func(c config.Config) {
			defer GinkgoRecover()
			atomic.StoreInt64(&changed, 1)
		})

		// simulate user updating the config map with a bad max duration
		var cm v1.ConfigMap
		Expect(env.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: "karpenter-global-settings"}, &cm)).To(Succeed())
		cm.Data = map[string]string{}
		cm.Data["batchIdleDuration"] = "-2s" // negative value
		cm.Data["batchMaxDuration"] = "15"   // no units
		ExpectApplied(ctx, env.Client, &cm)

		// we should read the new changes
		Eventually(func() bool {
			return cfg.BatchIdleDuration() == 1*time.Second
		}).Should(BeTrue())
		Eventually(func() bool {
			// and get the default value unchanged
			return cfg.BatchMaxDuration() == 10*time.Second
		}).Should(BeTrue())

		// and have been notified
		Eventually(func() bool {
			return atomic.LoadInt64(&changed) == 1
		}).Should(BeTrue())
	})
})
