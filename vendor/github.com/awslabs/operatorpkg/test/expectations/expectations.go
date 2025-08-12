package test

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	prometheus "github.com/prometheus/client_model/go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SlowTimeout = 100 * time.Second
	SlowPolling = 10 * time.Second
	FastTimeout = 1 * time.Second
	FastPolling = 10 * time.Millisecond
)

func ExpectObjectReconciled[T client.Object](ctx context.Context, c client.Client, reconciler reconcile.ObjectReconciler[T], object T) types.Assertion {
	GinkgoHelper()
	result, err := reconcile.AsReconciler(c, reconciler).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	Expect(err).ToNot(HaveOccurred())
	return Expect(result)
}

func ExpectObjectReconcileFailed[T client.Object](ctx context.Context, c client.Client, reconciler reconcile.ObjectReconciler[T], object T) types.Assertion {
	GinkgoHelper()
	_, err := reconcile.AsReconciler(c, reconciler).Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	Expect(err).To(HaveOccurred())
	return Expect(err)
}

func ExpectSingletonReconciled(ctx context.Context, reconciler singleton.Reconciler) reconcile.Result {
	GinkgoHelper()
	result, err := singleton.AsReconciler(reconciler).Reconcile(ctx, reconcile.Request{})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectSingletonReconcileFailed(ctx context.Context, reconciler singleton.Reconciler) error {
	GinkgoHelper()
	_, err := singleton.AsReconciler(reconciler).Reconcile(ctx, reconcile.Request{})
	Expect(err).To(HaveOccurred())
	return err
}

// Deprecated: Use the more modern ExpectObjectReconciled and reconcile.AsReconciler instead
func ExpectReconciled(ctx context.Context, reconciler reconcile.Reconciler, object client.Object) reconcile.Result {
	GinkgoHelper()
	result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
	Expect(err).ToNot(HaveOccurred())
	return result
}

func ExpectRequeued(result reconcile.Result) {
	GinkgoHelper()
	Expect(result.Requeue || result.RequeueAfter != lo.Empty[time.Duration]())
}

func ExpectNotRequeued(result reconcile.Result) {
	GinkgoHelper()
	Expect(!result.Requeue && result.RequeueAfter == lo.Empty[time.Duration]())
}

func ExpectObject[T client.Object](ctx context.Context, c client.Client, obj T) types.Assertion {
	GinkgoHelper()
	Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	return Expect(obj)
}

func ExpectNotFound(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		Eventually(func() bool { return errors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(o), o)) }).
			WithTimeout(FastTimeout).
			WithPolling(FastPolling).
			Should(BeTrue(), func() string {
				return fmt.Sprintf("expected %s to be deleted, but it still exists", object.GVKNN(o))
			})
	}
}

func ExpectApplied(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		// Create or Update
		if err := c.Get(ctx, client.ObjectKeyFromObject(current), current); err != nil {
			if errors.IsNotFound(err) {
				Expect(c.Create(ctx, o)).To(Succeed())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		} else {
			o.SetResourceVersion(current.GetResourceVersion())
			Expect(c.Update(ctx, o)).To(Succeed())
		}

		// Re-get the object to grab the updated spec and status
		ExpectObject(ctx, c, o)
	}
}

// ExpectDeletionTimestampSet ensures that the deletion timestamp is set on the objects by adding a finalizer
// and then deleting the object immediately after. This will hold the object until the finalizer is patched out
func ExpectDeletionTimestampSet(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(o), o)).To(Succeed())
		controllerutil.AddFinalizer(o, "testing/finalizer")
		Expect(c.Update(ctx, o)).To(Succeed())
		Expect(c.Delete(ctx, o)).To(Succeed())
	}
}

func ExpectStatusConditions(ctx context.Context, c client.Client, timeout time.Duration, obj status.Object, conditions ...status.Condition) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(BeNil())
		objStatus := obj.StatusConditions()
		for _, cond := range conditions {
			objCondition := objStatus.Get(cond.Type)
			g.Expect(objCondition).ToNot(BeNil())
			if cond.Status != "" {
				g.Expect(objCondition.Status).To(Equal(cond.Status))
			}
			if cond.Message != "" {
				g.Expect(objCondition.Message).To(Equal(cond.Message))
			}
			if cond.Reason != "" {
				g.Expect(objCondition.Reason).To(Equal(cond.Reason))
			}
		}
	}).
		WithTimeout(timeout).
		// each polling interval
		WithPolling(timeout / 20).
		Should(Succeed())
}

func ExpectStatusUpdated(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		// Previous implementations attempted the following:
		// 1. Using merge patch, instead
		// 2. Including this logic in ExpectApplied to simplify test code
		// The former doesn't work, as merge patches cannot reset
		// primitives like strings and integers to "" or 0, and CRDs
		// don't support strategic merge patch. The latter doesn't work
		// since status must be updated in another call, which can cause
		// optimistic locking issues if other threads are updating objects
		// e.g. pod statuses being updated during integration tests.
		Expect(c.Status().Update(ctx, o.DeepCopyObject().(client.Object))).To(Succeed())
		ExpectObject(ctx, c, o)
	}
}

func ExpectDeleted(ctx context.Context, c client.Client, objects ...client.Object) {
	GinkgoHelper()
	for _, o := range objects {
		Expect(client.IgnoreNotFound(c.Delete(ctx, o))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Get(ctx, client.ObjectKeyFromObject(o), o))).To(Succeed())
	}
}

// ExpectCleanedUp waits to cleanup all items passed through objectLists
func ExpectCleanedUp(ctx context.Context, c client.Client, objectLists ...client.ObjectList) {
	expectCleanedUp(ctx, c, false, objectLists...)
}

// ExpectForceCleanedUp waits to cleanup all items passed through objectLists
// It forcefully removes any finalizers from all of these objects to unblock delete
func ExpectForceCleanedUp(ctx context.Context, c client.Client, objectLists ...client.ObjectList) {
	expectCleanedUp(ctx, c, true, objectLists...)
}

func expectCleanedUp(ctx context.Context, c client.Client, force bool, objectLists ...client.ObjectList) {
	GinkgoHelper()
	wg := sync.WaitGroup{}
	for _, objectList := range objectLists {
		wg.Add(1)
		go func(objectList client.ObjectList) {
			defer GinkgoRecover()
			defer wg.Done()

			Eventually(func(g Gomega) {
				metaList := &metav1.PartialObjectMetadataList{}
				metaList.SetGroupVersionKind(lo.Must(apiutil.GVKForObject(objectList, c.Scheme())))
				g.Expect(c.List(ctx, metaList)).To(Succeed())

				for _, item := range metaList.Items {
					if force {
						stored := item.DeepCopy()
						item.SetFinalizers([]string{})
						g.Expect(c.Patch(ctx, &item, client.MergeFrom(stored))).To(Succeed())
					}
					if item.GetDeletionTimestamp().IsZero() {
						g.Expect(client.IgnoreNotFound(c.Delete(ctx, &item, client.PropagationPolicy(metav1.DeletePropagationForeground), &client.DeleteOptions{GracePeriodSeconds: lo.ToPtr(int64(0))}))).To(Succeed())
					}
				}
				g.Expect(c.List(ctx, metaList, client.Limit(1))).To(Succeed())
				g.Expect(metaList.Items).To(HaveLen(0))
			}).Should(Succeed())
		}(objectList)
	}
	wg.Wait()
}

// GetMetric attempts to find a metric given name and labels
// If no metric is found, the *prometheus.Metric will be nil
func GetMetric(name string, labels ...map[string]string) *prometheus.Metric {
	family, found := lo.Find(lo.Must(metrics.Registry.Gather()), func(family *prometheus.MetricFamily) bool { return family.GetName() == name })
	if !found {
		return nil
	}
	for _, m := range family.Metric {
		temp := lo.Assign(labels...)
		for _, labelPair := range m.Label {
			if v, ok := temp[labelPair.GetName()]; ok && v == labelPair.GetValue() {
				delete(temp, labelPair.GetName())
			}
		}
		if len(temp) == 0 {
			return m
		}
	}
	return nil
}
