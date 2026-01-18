/*
Copyright 2018 The Kubernetes Authors.

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

package builder

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ untypedWatchesInput = (*WatchesInput[struct{}])(nil)

type testLogger struct {
	logr.Logger
}

func (l *testLogger) Init(logr.RuntimeInfo) {
}

func (l *testLogger) Enabled(int) bool {
	return true
}

func (l *testLogger) Info(level int, msg string, keysAndValues ...interface{}) {
}

func (l *testLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return l
}

func (l *testLogger) WithName(name string) logr.LogSink {
	return l
}

type empty struct{}

var _ = Describe("application", func() {
	noop := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
		return reconcile.Result{}, nil
	})
	typedNoop := reconcile.TypedFunc[empty](func(context.Context, empty) (reconcile.Result, error) {
		return reconcile.Result{}, nil
	})

	Describe("New", func() {
		It("should return success if given valid objects", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should return error if given two apiType objects in For function", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				For(&appsv1.Deployment{}).
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).To(MatchError(ContainSubstring("For(...) should only be called once, could not assign multiple objects for reconciliation")))
			Expect(instance).To(BeNil())
		})

		It("should return an error if For and Named function are not called", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				Watches(&appsv1.ReplicaSet{}, &handler.EnqueueRequestForObject{}).
				Build(noop)
			Expect(err).To(MatchError(ContainSubstring("one of For() or Named() must be called")))
			Expect(instance).To(BeNil())
		})

		It("should return an error when using Owns without For", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				Named("my_controller").
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).To(MatchError(ContainSubstring("Owns() can only be used together with For()")))
			Expect(instance).To(BeNil())

		})

		It("should return an error when there are no watches", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				Named("my_new_controller").
				Build(noop)
			Expect(err).To(MatchError(ContainSubstring("there are no watches configured, controller will never get triggered. Use For(), Owns(), Watches() or WatchesRawSource() to set them up")))
			Expect(instance).To(BeNil())
		})

		It("should allow creating a controllerw without calling For", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := ControllerManagedBy(m).
				Named("my_other_controller").
				Watches(&appsv1.ReplicaSet{}, &handler.EnqueueRequestForObject{}).
				Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should return an error if there is no GVK for an object, and thus we can't default the controller name", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			By("creating a controller with a bad For type")
			instance, err := ControllerManagedBy(m).
				For(&fakeType{}).
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).To(MatchError(ContainSubstring("no kind is registered for the type builder.fakeType")))
			Expect(instance).To(BeNil())

			// NB(directxman12): we don't test non-for types, since errors for
			// them now manifest on controller.Start, not controller.Watch.  Errors on the For type
			// manifest when we try to default the controller name, which is good to double check.
		})

		It("should return error if in For is used with a custom request type", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := TypedControllerManagedBy[empty](m).
				For(&appsv1.ReplicaSet{}).
				Named("last_controller").
				Build(typedNoop)
			Expect(err).To(MatchError(ContainSubstring("For() can only be used with reconcile.Request, got builder.empty")))
			Expect(instance).To(BeNil())
		})

		It("should return error if in Owns is used with a custom request type", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := TypedControllerManagedBy[empty](m).
				Named("my_controller-0").
				Owns(&appsv1.ReplicaSet{}).
				Build(typedNoop)
				// If we ever allow Owns() without For() we need to update the code to error
				// out on Owns() if the request type is different from reconcile.Request just
				// like we do in For().
			Expect(err).To(MatchError("Owns() can only be used together with For()"))
			Expect(instance).To(BeNil())
		})

		It("should build a controller with a custom request type", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			instance, err := TypedControllerManagedBy[empty](m).
				Named("my_controller-1").
				WatchesRawSource(
					source.TypedKind(
						m.GetCache(),
						&appsv1.ReplicaSet{},
						handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, rs *appsv1.ReplicaSet) []empty {
							return []empty{{}}
						}),
					),
				).
				Build(typedNoop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should return an error if it cannot create the controller", func() {

			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Owns(&appsv1.ReplicaSet{})
			builder.newController = func(name string, mgr manager.Manager, options controller.Options) (
				controller.Controller, error) {
				return nil, fmt.Errorf("expected error")
			}
			instance, err := builder.Build(noop)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
			Expect(instance).To(BeNil())
		})

		It("should override max concurrent reconcilers during creation of controller", func() {
			const maxConcurrentReconciles = 5
			newController := func(name string, mgr manager.Manager, options controller.Options) (
				controller.Controller, error) {
				if options.MaxConcurrentReconciles == maxConcurrentReconciles {
					return controller.New(name, mgr, options)
				}
				return nil, fmt.Errorf("max concurrent reconcilers expected %d but found %d", maxConcurrentReconciles, options.MaxConcurrentReconciles)
			}

			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Named("replicaset-4").
				Owns(&appsv1.ReplicaSet{}).
				WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles})
			builder.newController = newController

			instance, err := builder.Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should override max concurrent reconcilers during creation of controller, when using", func() {
			const maxConcurrentReconciles = 10
			newController := func(name string, mgr manager.Manager, options controller.Options) (
				controller.Controller, error) {
				if options.MaxConcurrentReconciles == maxConcurrentReconciles {
					return controller.New(name, mgr, options)
				}
				return nil, fmt.Errorf("max concurrent reconcilers expected %d but found %d", maxConcurrentReconciles, options.MaxConcurrentReconciles)
			}

			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{
				Controller: config.Controller{
					GroupKindConcurrency: map[string]int{
						"ReplicaSet.apps": maxConcurrentReconciles,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Named("replicaset-3").
				Owns(&appsv1.ReplicaSet{})
			builder.newController = newController

			instance, err := builder.Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should override rate limiter during creation of controller", func() {
			rateLimiter := workqueue.DefaultTypedItemBasedRateLimiter[reconcile.Request]()
			newController := func(name string, mgr manager.Manager, options controller.Options) (controller.Controller, error) {
				if options.RateLimiter == rateLimiter {
					return controller.New(name, mgr, options)
				}
				return nil, fmt.Errorf("rate limiter expected %T but found %T", rateLimiter, options.RateLimiter)
			}

			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Named("replicaset-2").
				Owns(&appsv1.ReplicaSet{}).
				WithOptions(controller.Options{RateLimiter: rateLimiter})
			builder.newController = newController

			instance, err := builder.Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should override logger during creation of controller", func() {
			logger := &testLogger{}
			newController := func(name string, mgr manager.Manager, options controller.Options) (controller.Controller, error) {
				if options.LogConstructor(nil).GetSink() == logger {
					return controller.New(name, mgr, options)
				}
				return nil, fmt.Errorf("logger expected %T but found %T", logger, options.LogConstructor)
			}

			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Named("replicaset-0").
				Owns(&appsv1.ReplicaSet{}).
				WithLogConstructor(func(request *reconcile.Request) logr.Logger {
					return logr.New(logger)
				})
			builder.newController = newController
			instance, err := builder.Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
		})

		It("should not allow multiple reconcilers during creation of controller", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			builder := ControllerManagedBy(m).
				For(&appsv1.ReplicaSet{}).
				Named("replicaset-1").
				Owns(&appsv1.ReplicaSet{}).
				WithOptions(controller.Options{Reconciler: noop})
			instance, err := builder.Build(noop)
			Expect(err).To(HaveOccurred())
			Expect(instance).To(BeNil())
		})

		It("should allow multiple controllers for the same kind", func() {
			By("creating a controller manager")
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			By("registering the type in the Scheme")
			builder := scheme.Builder{GroupVersion: testDefaultValidatorGVK.GroupVersion()}
			builder.Register(&TestDefaultValidator{}, &TestDefaultValidatorList{})
			err = builder.AddToScheme(m.GetScheme())
			Expect(err).NotTo(HaveOccurred())

			By("creating the 1st controller")
			ctrl1, err := ControllerManagedBy(m).
				For(&TestDefaultValidator{}).
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(ctrl1).NotTo(BeNil())

			By("creating the 2nd controller")
			ctrl2, err := ControllerManagedBy(m).
				For(&TestDefaultValidator{}).
				Named("test-default-validator-1").
				Owns(&appsv1.ReplicaSet{}).
				Build(noop)
			Expect(err).NotTo(HaveOccurred())
			Expect(ctrl2).NotTo(BeNil())
		})
	})

	Describe("Start with ControllerManagedBy", func() {
		It("should Reconcile Owns objects", func(ctx SpecContext) {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			bldr := ControllerManagedBy(m).
				For(&appsv1.Deployment{}).
				Named("deployment-0").
				Owns(&appsv1.ReplicaSet{})

			doReconcileTest(ctx, "3", m, false, bldr)
		})

		It("should Reconcile Owns objects for every owner", func(ctx SpecContext) {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			bldr := ControllerManagedBy(m).
				For(&appsv1.Deployment{}).
				Named("deployment-1").
				Owns(&appsv1.ReplicaSet{}, MatchEveryOwner)

			doReconcileTest(ctx, "12", m, false, bldr)
		})

		It("should Reconcile Watches objects", func(ctx SpecContext) {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			bldr := ControllerManagedBy(m).
				For(&appsv1.Deployment{}).
				Watches( // Equivalent of Owns
					&appsv1.ReplicaSet{},
					handler.EnqueueRequestForOwner(m.GetScheme(), m.GetRESTMapper(), &appsv1.Deployment{}, handler.OnlyControllerOwner()),
				)

			doReconcileTest(ctx, "4", m, true, bldr)
		})

		It("should Reconcile without For", func(ctx SpecContext) {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			bldr := ControllerManagedBy(m).
				Named("Deployment").
				Named("deployment-2").
				Watches( // Equivalent of For
						&appsv1.Deployment{}, &handler.EnqueueRequestForObject{}).
				Watches( // Equivalent of Owns
					&appsv1.ReplicaSet{},
					handler.EnqueueRequestForOwner(m.GetScheme(), m.GetRESTMapper(), &appsv1.Deployment{}, handler.OnlyControllerOwner()),
				)

			doReconcileTest(ctx, "9", m, true, bldr)
		})
	})

	Describe("Set custom predicates", func() {
		It("should execute registered predicates only for assigned kind", func(ctx SpecContext) {
			m, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			var (
				deployPrctExecuted     = false
				replicaSetPrctExecuted = false
				allPrctExecuted        = int64(0)
			)

			deployPrct := predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					defer GinkgoRecover()
					// check that it was called only for deployment
					Expect(e.Object).To(BeAssignableToTypeOf(&appsv1.Deployment{}))
					deployPrctExecuted = true
					return true
				},
			}

			replicaSetPrct := predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					defer GinkgoRecover()
					// check that it was called only for replicaset
					Expect(e.Object).To(BeAssignableToTypeOf(&appsv1.ReplicaSet{}))
					replicaSetPrctExecuted = true
					return true
				},
			}

			allPrct := predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					defer GinkgoRecover()
					// check that it was called for all registered kinds
					Expect(e.Object).Should(Or(
						BeAssignableToTypeOf(&appsv1.Deployment{}),
						BeAssignableToTypeOf(&appsv1.ReplicaSet{}),
					))

					atomic.AddInt64(&allPrctExecuted, 1)
					return true
				},
			}

			bldr := ControllerManagedBy(m).
				For(&appsv1.Deployment{}, WithPredicates(deployPrct)).
				Named("deployment-3").
				Owns(&appsv1.ReplicaSet{}, WithPredicates(replicaSetPrct)).
				WithEventFilter(allPrct)

			doReconcileTest(ctx, "5", m, true, bldr)

			Expect(deployPrctExecuted).To(BeTrue(), "Deploy predicated should be called at least once")
			Expect(replicaSetPrctExecuted).To(BeTrue(), "ReplicaSet predicated should be called at least once")
			Expect(allPrctExecuted).To(BeNumerically(">=", 2), "Global Predicated should be called at least twice")
		})
	})

	Describe("watching with projections", func() {
		var mgr manager.Manager
		BeforeEach(func() {
			// use a cache that intercepts requests for fully typed objects to
			// ensure we use the projected versions
			var err error
			mgr, err = manager.New(cfg, manager.Options{NewCache: newNonTypedOnlyCache})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should support multiple controllers watching the same metadata kind", func(ctx SpecContext) {
			bldr1 := ControllerManagedBy(mgr).For(&appsv1.Deployment{}, OnlyMetadata).Named("deployment-4")
			bldr2 := ControllerManagedBy(mgr).For(&appsv1.Deployment{}, OnlyMetadata).Named("deployment-5")

			doReconcileTest(ctx, "6", mgr, true, bldr1, bldr2)
		})

		It("should support watching For, Owns, and Watch as metadata", func(ctx SpecContext) {
			statefulSetMaps := make(chan *metav1.PartialObjectMetadata)

			bldr := ControllerManagedBy(mgr).
				For(&appsv1.Deployment{}, OnlyMetadata).
				Named("deployment-6").
				Owns(&appsv1.ReplicaSet{}, OnlyMetadata).
				Watches(&appsv1.StatefulSet{},
					handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
						defer GinkgoRecover()

						ometa := o.(*metav1.PartialObjectMetadata)
						statefulSetMaps <- ometa

						// Validate that the GVK is not empty when dealing with PartialObjectMetadata objects.
						Expect(o.GetObjectKind().GroupVersionKind()).To(Equal(schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "StatefulSet",
						}))
						return nil
					}),
					OnlyMetadata)

			doReconcileTest(ctx, "8", mgr, true, bldr)

			By("Creating a new stateful set")
			set := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test1",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			err := mgr.GetClient().Create(ctx, set)
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the mapping function has been called")
			Eventually(func() bool {
				metaSet := <-statefulSetMaps
				Expect(metaSet.Name).To(Equal(set.Name))
				Expect(metaSet.Namespace).To(Equal(set.Namespace))
				Expect(metaSet.Labels).To(Equal(set.Labels))
				return true
			}).Should(BeTrue())
		})
	})
})

// newNonTypedOnlyCache returns a new cache that wraps the normal cache,
// returning an error if normal, typed objects have informers requested.
func newNonTypedOnlyCache(config *rest.Config, opts cache.Options) (cache.Cache, error) {
	normalCache, err := cache.New(config, opts)
	if err != nil {
		return nil, err
	}
	return &nonTypedOnlyCache{
		Cache: normalCache,
	}, nil
}

// nonTypedOnlyCache is a cache.Cache that only provides metadata &
// unstructured informers.
type nonTypedOnlyCache struct {
	cache.Cache
}

func (c *nonTypedOnlyCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	switch obj.(type) {
	case (*metav1.PartialObjectMetadata):
		return c.Cache.GetInformer(ctx, obj, opts...)
	default:
		return nil, fmt.Errorf("did not want to provide an informer for normal type %T", obj)
	}
}
func (c *nonTypedOnlyCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return nil, fmt.Errorf("don't try to sidestep the restriction on informer types by calling GetInformerForKind")
}

// TODO(directxman12): this function has too many arguments, and the whole
// "nameSuffix" think is a bit of a hack It should be cleaned up significantly by someone with a bit of time.
func doReconcileTest(ctx context.Context, nameSuffix string, mgr manager.Manager, complete bool, blders ...*TypedBuilder[reconcile.Request]) {
	deployName := "deploy-name-" + nameSuffix
	rsName := "rs-name-" + nameSuffix

	By("Creating the application")
	ch := make(chan reconcile.Request)
	fn := reconcile.Func(func(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
		defer GinkgoRecover()
		if !strings.HasSuffix(req.Name, nameSuffix) {
			// From different test, ignore this request.  Etcd is shared across tests.
			return reconcile.Result{}, nil
		}
		ch <- req
		return reconcile.Result{}, nil
	})

	for _, blder := range blders {
		if complete {
			err := blder.Complete(fn)
			Expect(err).NotTo(HaveOccurred())
		} else {
			var err error
			var c controller.Controller
			c, err = blder.Build(fn)
			Expect(err).NotTo(HaveOccurred())
			Expect(c).NotTo(BeNil())
		}
	}

	By("Starting the application")
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
	}()

	By("Creating a Deployment")
	// Expect a Reconcile when the Deployment is managedObjects.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      deployName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
	err := mgr.GetClient().Create(ctx, dep)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for the Deployment Reconcile")
	Eventually(ch).Should(Receive(Equal(reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: deployName}})))

	By("Creating a ReplicaSet")
	// Expect a Reconcile when an Owned object is managedObjects.
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      rsName,
			Labels:    dep.Spec.Selector.MatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       deployName,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Controller: ptr.To(true),
					UID:        dep.UID,
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: dep.Spec.Selector,
			Template: dep.Spec.Template,
		},
	}
	err = mgr.GetClient().Create(ctx, rs)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for the ReplicaSet Reconcile")
	Eventually(ch).Should(Receive(Equal(reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: deployName}})))
}

var _ runtime.Object = &fakeType{}

type fakeType struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (*fakeType) GetObjectKind() schema.ObjectKind { return nil }
func (*fakeType) DeepCopyObject() runtime.Object   { return nil }
