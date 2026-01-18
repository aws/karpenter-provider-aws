package events_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/events"
	pmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/test"
	. "github.com/awslabs/operatorpkg/test/expectations"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(schema.GroupVersion{Group: test.APIGroup, Version: "v1alpha1"}, &test.CustomObject{})
		return nil
	})
)

var ctx context.Context
var fakeClock *clock.FakeClock
var controller *events.Controller[*test.CustomObject]
var kubeClient client.Client

func Test(t *testing.T) {
	lo.Must0(SchemeBuilder.AddToScheme(scheme.Scheme))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Events")
}

var _ = BeforeSuite(func() {
	fakeClock = clock.NewFakeClock(time.Now())
	kubeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithIndex(&corev1.Event{}, "involvedObject.kind", func(o client.Object) []string {
		evt := o.(*corev1.Event)
		return []string{evt.InvolvedObject.Kind}
	}).Build()
	controller = events.NewController[*test.CustomObject](kubeClient, fakeClock)
	ctx = log.IntoContext(context.Background(), ginkgo.GinkgoLogr)
})

var _ = Describe("Controller", func() {
	BeforeEach(func() {
		controller.EventCount.Reset()
	})
	It("should emit metrics on an event", func() {
		events := []*corev1.Event{}

		for i := range 5 {
			// create an event for custom object
			events = append(events, createEvent("test-object", fmt.Sprintf("Test-type-%d", i), fmt.Sprintf("Test-reason-%d", i)))
			ExpectApplied(ctx, kubeClient, events[i])

			// expect an metrics for custom object to be zero, waiting on controller reconcile
			Expect(GetMetric("operator_customobject_event_total", conditionLabels(fmt.Sprintf("Test-type-%d", i), fmt.Sprintf("Test-reason-%d", i)))).To(BeNil())

			// reconcile on the event
			_, err := reconcile.AsReconciler(kubeClient, controller).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(events[i])})
			Expect(err).ToNot(HaveOccurred())

			// expect an emitted metric to for the event
			Expect(GetMetric("operator_customobject_event_total", conditionLabels(fmt.Sprintf("Test-type-%d", i), fmt.Sprintf("Test-reason-%d", i))).GetCounter().GetValue()).To(BeEquivalentTo(1))
		}
	})
	It("should not fire metrics if the last transition was before controller start-up", func() {
		// create an event for custom object that was produced before the controller start-up time
		event := createEvent("test-name", corev1.EventTypeNormal, "reason")
		event.LastTimestamp.Time = time.Now().Add(30 * time.Minute)
		ExpectApplied(ctx, kubeClient, event)

		// expect an metrics for custom object to be zero, waiting on controller reconcile
		Expect(GetMetric("operator_ustomobject_event_total", conditionLabels(corev1.EventTypeNormal, "reason"))).To(BeNil())

		// reconcile on the event
		_, err := reconcile.AsReconciler(kubeClient, controller).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(event)})
		Expect(err).ToNot(HaveOccurred())

		// expect not have an emitted metric to for the event
		Expect(GetMetric("operator_customobject_event_total", conditionLabels(corev1.EventTypeNormal, "reason")).GetCounter().GetValue()).To(BeEquivalentTo(1))

		// create an event for custom object that was produced after the controller start-up time
		event.LastTimestamp.Time = time.Now().Add(-30 * time.Minute)
		ExpectApplied(ctx, kubeClient, event)

		// reconcile on the event
		_, err = reconcile.AsReconciler(kubeClient, controller).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(event)})
		Expect(err).ToNot(HaveOccurred())

		// expect an emitted metric to for the event
		Expect(GetMetric("operator_customobject_event_total", conditionLabels(corev1.EventTypeNormal, "reason")).GetCounter().GetValue()).To(BeEquivalentTo(1))
	})
})

func createEvent(name string, eventType string, reason string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: test.RandomName(),
		},
		InvolvedObject: corev1.ObjectReference{
			Namespace: "default",
			Name:      name,
			Kind:      object.GVK(&test.CustomObject{}).Kind,
		},
		LastTimestamp: metav1.Time{Time: time.Now().Add(30 * time.Minute)},
		Type:          eventType,
		Reason:        reason,
		Count:         5,
	}
}

func conditionLabels(eventType string, reason string) map[string]string {
	return map[string]string{
		pmetrics.LabelType:   eventType,
		pmetrics.LabelReason: reason,
	}
}
