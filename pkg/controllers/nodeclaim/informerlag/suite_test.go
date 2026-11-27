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

package informerlag_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	clocktesting "k8s.io/utils/clock/testing"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	coreapis "sigs.k8s.io/karpenter/pkg/apis"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/informerlag"
)

var ctx context.Context
var env *coretest.Environment

func TestInformerLag(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InformerLag")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(coreapis.CRDs...))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("InformerLag", func() {
	var now time.Time
	var oldNodeClaim, newNodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		now = time.Date(2026, 11, 16, 11, 19, 12, 0, time.UTC)
		oldNodeClaim = coretest.NodeClaim()
		oldNodeClaim.ResourceVersion = "1"
		oldNodeClaim.Finalizers = []string{karpv1.TerminationFinalizer}
		oldNodeClaim.ManagedFields = []metav1.ManagedFieldsEntry{
			{Manager: "karpenter", Operation: metav1.ManagedFieldsOperationUpdate, Time: &metav1.Time{Time: now.Add(-50 * time.Second)}},
			{Manager: "karpenter", Operation: metav1.ManagedFieldsOperationUpdate, Subresource: "status", Time: &metav1.Time{Time: now.Add(-40 * time.Second)}},
			{Manager: "other", Operation: metav1.ManagedFieldsOperationUpdate},
		}
		newNodeClaim = oldNodeClaim.DeepCopy()
		newNodeClaim.ResourceVersion = "2"
		newNodeClaim.ManagedFields[1].Time = &metav1.Time{Time: now.Add(-19 * time.Second)}
		informerlag.LagSeconds.Reset()
	})
	It("should measure lag from the newest managedFields write", func() {
		lag, ok := informerlag.Lag(now, oldNodeClaim, newNodeClaim)
		Expect(ok).To(BeTrue())
		Expect(lag).To(Equal(19 * time.Second))
	})
	It("should observe the lag on update events", func() {
		informerlag.NewController(clocktesting.NewFakeClock(now)).Handler().OnUpdate(oldNodeClaim, newNodeClaim)
		m, found := FindMetricWithLabelValues("karpenter_nodeclaims_informer_lag_seconds", map[string]string{})
		Expect(found).To(BeTrue())
		Expect(m.GetHistogram().GetSampleCount()).To(BeNumerically("==", 1))
		Expect(m.GetHistogram().GetSampleSum()).To(BeNumerically("==", 19))
	})
	It("should ignore resyncs", func() {
		_, ok := informerlag.Lag(now, oldNodeClaim, oldNodeClaim.DeepCopy())
		Expect(ok).To(BeFalse())
	})
	It("should ignore updates that only remove fields", func() {
		newNodeClaim = oldNodeClaim.DeepCopy()
		newNodeClaim.ResourceVersion = "2"
		newNodeClaim.Finalizers = nil
		_, ok := informerlag.Lag(now, oldNodeClaim, newNodeClaim)
		Expect(ok).To(BeFalse())
	})
	It("should ignore the update that sets the deletionTimestamp", func() {
		newNodeClaim = oldNodeClaim.DeepCopy()
		newNodeClaim.ResourceVersion = "2"
		newNodeClaim.DeletionTimestamp = &metav1.Time{Time: now}
		_, ok := informerlag.Lag(now, oldNodeClaim, newNodeClaim)
		Expect(ok).To(BeFalse())
	})
	It("should ignore objects without managedFields times", func() {
		oldNodeClaim.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "other"}}
		newNodeClaim.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "other"}}
		_, ok := informerlag.Lag(now, oldNodeClaim, newNodeClaim)
		Expect(ok).To(BeFalse())
	})
	It("should not report negative lag when clocks are skewed", func() {
		lag, ok := informerlag.Lag(now.Add(-time.Minute), oldNodeClaim, newNodeClaim)
		Expect(ok).To(BeTrue())
		Expect(lag).To(BeZero())
	})
	It("should observe NodeClaim updates from a manager's informer", func() {
		mgr, err := controllerruntime.NewManager(env.Config, controllerruntime.Options{
			Scheme:     scheme.Scheme,
			Metrics:    metricsserver.Options{BindAddress: "0"},
			Controller: config.Controller{SkipNameValidation: lo.ToPtr(true)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(informerlag.NewController(clock.RealClock{}).Register(ctx, mgr)).To(Succeed())
		mgrCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()
		DeferCleanup(func() {
			cancel()
			Eventually(done).Should(BeClosed())
		})
		Expect(mgr.GetCache().WaitForCacheSync(ctx)).To(BeTrue())

		nodeClaim := coretest.NodeClaim()
		ExpectApplied(ctx, env.Client, nodeClaim)
		// managedFields times are whole seconds, so the update has to land in a later second to advance them
		time.Sleep(time.Second)
		nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{"updated": "true"})
		ExpectApplied(ctx, env.Client, nodeClaim)
		Eventually(func(g Gomega) {
			m, found := FindMetricWithLabelValues("karpenter_nodeclaims_informer_lag_seconds", map[string]string{})
			g.Expect(found).To(BeTrue())
			g.Expect(m.GetHistogram().GetSampleCount()).To(BeNumerically(">=", 1))
			// envtest runs on this host, so the lag is within the one-second timestamp resolution
			g.Expect(m.GetHistogram().GetSampleSum()).To(BeNumerically("<", 2*m.GetHistogram().GetSampleCount()))
		}).Should(Succeed())
		ExpectDeleted(ctx, env.Client, nodeClaim)
	})
})
