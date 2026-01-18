/*
Copyright The Kubernetes Authors.

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

package pod_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/metrics/pod"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var podController *pod.Controller
var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var cloudProvider *fake.CloudProvider
var fakeClock *clock.FakeClock

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodMetrics")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment()
	fakeClock = clock.NewFakeClock(time.Now())
	cloudProvider = fake.NewCloudProvider()
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	podController = pod.NewController(env.Client, cluster)
})

var _ = AfterEach(func() {
	cluster.Reset()
	pod.PodStartupDurationSeconds.Reset()
	pod.PodProvisioningStartupDurationSeconds.Reset()
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Pod Metrics", func() {
	It("should update the pod state metrics", func() {
		p := test.Pod()
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found := FindMetricWithLabelValues("karpenter_pods_state", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
	})
	It("should update the pod state metrics with pod phase", func() {
		p := test.Pod()
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found := FindMetricWithLabelValues("karpenter_pods_state", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		p.Status.Phase = corev1.PodRunning
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found = FindMetricWithLabelValues("karpenter_pods_state", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
			"phase":     string(p.Status.Phase),
		})
		Expect(found).To(BeTrue())
	})
	It("should update the pod bound and unbound time metrics", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending

		fakeClock.Step(1 * time.Hour)
		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})

		// PodScheduled condition does not exist, emit pods_unbound_time_seconds metric
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will add pod to pending pods and unscheduled pods set
		_, found := FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionUnknown, LastTransitionTime: metav1.Now()}}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will add pod to pending pods and unscheduled pods set
		metric, found := FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		unboundTime := metric.GetGauge().Value
		Expect(found).To(BeTrue())
		metric, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		provisioningUnboundTime := metric.GetGauge().Value

		// Pod is still pending but has bound. At this step pods_unbound_duration should not change.
		p.Status.Phase = corev1.PodPending
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled or not
		metric, found = FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		Expect(metric.GetGauge().Value).To(Equal(unboundTime))

		metric, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		Expect(metric.GetGauge().Value).To(Equal(provisioningUnboundTime))

		// Pod is still running and has bound. At this step pods_bound_duration should be fired and pods_unbound_time_seconds should be deleted
		p.Status.Phase = corev1.PodRunning
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled or not
		_, found = FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		_, found = FindMetricWithLabelValues("karpenter_pods_bound_duration_seconds", map[string]string{})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_bound_duration_seconds", map[string]string{})
		Expect(found).To(BeTrue())
	})
	It("should update the pod startup and unstarted time metrics", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending

		fakeClock.Step(1 * time.Hour)
		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will add pod to pending pods and unscheduled pods set
		_, found := FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		// Pod is now running but readiness condition is not set
		p.Status.Phase = corev1.PodRunning
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled or not
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		// Pod is now running but readiness is unknown
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionUnknown, LastTransitionTime: metav1.Now()}}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled or not
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		// Pod is now running and ready. At this step pods_startup_duration should be fired and pods_unstarted_time should be deleted
		p.Status.Phase = corev1.PodRunning
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled or not
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		_, found = FindMetricWithLabelValues("karpenter_pods_startup_duration_seconds", nil)
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_startup_duration_seconds", nil)
		Expect(found).To(BeTrue())
	})
	It("should update the pod unstarted time metrics when the pod has succeeded", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending

		fakeClock.Step(1 * time.Hour)
		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will add pod to pending pods and unscheduled pods set
		_, found := FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		// Pod has now succeeded but readiness condition is set to false because the pod is now completed
		p.Status.Phase = corev1.PodSucceeded
		p.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.Now()},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()},
		}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled and completed
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		_, found = FindMetricWithLabelValues("karpenter_pods_startup_duration_seconds", nil)
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_startup_duration_seconds", nil)
		Expect(found).To(BeFalse())
	})
	It("should update the pod unstarted time metrics when the pod has failed", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending

		fakeClock.Step(1 * time.Hour)
		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will add pod to pending pods and unscheduled pods set
		_, found := FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		// Pod has now failed and readiness condition is set to false
		p.Status.Phase = corev1.PodFailed
		p.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.Now()},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()},
		}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p)) //This will check if the pod was scheduled and completed
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		_, found = FindMetricWithLabelValues("karpenter_pods_startup_duration_seconds", nil)
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_startup_duration_seconds", nil)
		Expect(found).To(BeFalse())
	})
	It("should create and delete provisioning undecided metrics based on scheduling simulatinos", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending
		ExpectApplied(ctx, env.Client, p)

		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		fakeClock.Step(1 * time.Hour)
		_, found := FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		// Expect the metric to exist now that we've ack'd the pod
		cluster.AckPods(p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		fakeClock.Step(1 * time.Hour)

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
	It("should delete provisioning undecided metrics based on pod deletes", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending
		ExpectApplied(ctx, env.Client, p)

		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		fakeClock.Step(1 * time.Hour)
		_, found := FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())

		// Expect the metric to exist now that we've ack'd the pod
		cluster.AckPods(p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		fakeClock.Step(1 * time.Hour)

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		ExpectDeleted(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
	It("should delete provisioning undecided metrics when the pod is already bound", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending
		ExpectApplied(ctx, env.Client, p)

		cluster.AckPods(p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		fakeClock.Step(1 * time.Hour)

		_, found := FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_scheduling_undecided_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
	It("should delete pod unbound and unstarted time metrics on pod delete", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending
		ExpectApplied(ctx, env.Client, p)

		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found := FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())

		ExpectDeleted(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		_, found = FindMetricWithLabelValues("karpenter_pods_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
	It("should delete pod unbound and unstarted time metrics when pod schedulable time is zero", func() {
		p := test.Pod()
		p.Status.Phase = corev1.PodPending
		ExpectApplied(ctx, env.Client, p)

		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{}, map[string][]*corev1.Pod{"n1": {p}}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		_, found := FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeTrue())
		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{p: fmt.Errorf("ignoring pod")}, map[string][]*corev1.Pod{}, map[string][]*corev1.Pod{"nc1": {p}})
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unbound_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
		_, found = FindMetricWithLabelValues("karpenter_pods_provisioning_unstarted_time_seconds", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
	It("should delete the pod state metric on pod delete", func() {
		p := test.Pod()
		ExpectApplied(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		ExpectDeleted(ctx, env.Client, p)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(p))

		_, found := FindMetricWithLabelValues("karpenter_pods_state", map[string]string{
			"name":      p.GetName(),
			"namespace": p.GetNamespace(),
		})
		Expect(found).To(BeFalse())
	})
})
