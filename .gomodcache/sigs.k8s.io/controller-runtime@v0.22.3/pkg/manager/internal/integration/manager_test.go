/*
Copyright 2023 The Kubernetes Authors.

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

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crewv1 "sigs.k8s.io/controller-runtime/pkg/manager/internal/integration/api/v1"
	crewv2 "sigs.k8s.io/controller-runtime/pkg/manager/internal/integration/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

var (
	scheme = runtime.NewScheme()

	driverCRD = &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "drivers.crew.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: crewv1.GroupVersion.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "drivers",
				Singular: "driver",
				Kind:     "Driver",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:   crewv1.GroupVersion.Version,
					Served: true,
					// v1 will be the storage version.
					// Reconciler and index will use v2 so we can validate the conversion webhook works.
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
				{
					Name:    crewv2.GroupVersion.Version,
					Served:  true,
					Storage: false,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}

	ctx = ctrl.SetupSignalHandler()
)

var _ = Describe("manger.Manager Start", func() {
	// This test ensure the Manager starts without running into any deadlocks as it can be very tricky
	// to start health probes, webhooks, caches (including informers) and reconcilers in the right order.
	//
	// To verify this we set up a test environment in the following state:
	// * Ensure Informer sync requires a functioning conversion webhook (and thus readiness probe)
	//   * Driver CRD is deployed with v1 as storage version
	//   * A Driver CR is created and stored in the v1 version
	// * Setup manager:
	//   * Set up health probes
	//   * Set up a Driver v2 reconciler to verify reconciliation works
	//   * Set up a conversion webhook which only works if readiness probe succeeds (just like via a Kubernetes service)
	//   * Add an index on v2 Driver to ensure we start and wait for an informer during cache.Start (as part of manager.Start)
	//     * Note: cache.Start would fail if the conversion webhook doesn't work (which in turn depends on the readiness probe)
	//     * Note: Adding the index for v2 ensures the Driver list call during Informer sync goes through conversion.
	DescribeTable("should start all components without deadlock", func(warmupEnabled bool) {
		// Set up schema.
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
		Expect(crewv1.AddToScheme(scheme)).To(Succeed())
		Expect(crewv2.AddToScheme(scheme)).To(Succeed())

		// Set up test environment.
		env := &envtest.Environment{
			Scheme: scheme,
			CRDInstallOptions: envtest.CRDInstallOptions{
				CRDs: []*apiextensionsv1.CustomResourceDefinition{driverCRD},
			},
		}
		// Note: The test env configures a conversion webhook on driverCRD during Start.
		cfg, err := env.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		defer func() {
			Expect(env.Stop()).To(Succeed())
		}()
		c, err := client.New(cfg, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		// Create driver CR (which is stored as v1).
		driverV1 := &unstructured.Unstructured{}
		driverV1.SetGroupVersionKind(crewv1.GroupVersion.WithKind("Driver"))
		driverV1.SetName("driver1")
		driverV1.SetNamespace(metav1.NamespaceDefault)
		Expect(c.Create(ctx, driverV1)).To(Succeed())

		// Set up Manager.
		ctrl.SetLogger(zap.New())
		mgr, err := manager.New(env.Config, manager.Options{
			Scheme:                 scheme,
			HealthProbeBindAddress: ":0",
			// Disable metrics to avoid port conflicts.
			Metrics: metricsserver.Options{BindAddress: "0"},
			WebhookServer: webhook.NewServer(webhook.Options{
				Port:    env.WebhookInstallOptions.LocalServingPort,
				Host:    env.WebhookInstallOptions.LocalServingHost,
				CertDir: env.WebhookInstallOptions.LocalServingCertDir,
			}),
		})
		Expect(err).NotTo(HaveOccurred())

		// Configure health probes.
		Expect(mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker())).To(Succeed())
		Expect(mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker())).To(Succeed())

		// Set up Driver reconciler (using v2).
		driverReconciler := &DriverReconciler{
			Client: mgr.GetClient(),
		}
		Expect(
			ctrl.NewControllerManagedBy(mgr).
				For(&crewv2.Driver{}).
				Named(fmt.Sprintf("driver_warmup_%t", warmupEnabled)).
				WithOptions(controller.Options{EnableWarmup: ptr.To(warmupEnabled)}).
				Complete(driverReconciler),
		).To(Succeed())

		// Set up a conversion webhook.
		conversionWebhook := createConversionWebhook(mgr)
		mgr.GetWebhookServer().Register("/convert", conversionWebhook)

		// Add an index on Driver (using v2).
		// Note: This triggers the creation of an Informer for Driver v2.
		Expect(mgr.GetCache().IndexField(ctx, &crewv2.Driver{}, "name", func(object client.Object) []string {
			return []string{object.GetName()}
		})).To(Succeed())

		// Start the Manager.
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).To(Succeed())
		}()

		// Verify manager.Start successfully started health probes, webhooks, caches (including informers) and reconcilers.
		// Notes:
		// * The cache will only start successfully if the informer for v2 Driver is synced.
		// * The informer for v2 Driver will only sync if a list on v2 Driver succeeds (which requires a working conversion webhook)
		select {
		case <-time.After(30 * time.Second):
			// Don't wait forever if the manager doesn't come up.
			Fail("Manager didn't start in time")
		case <-mgr.Elected():
		}

		// Verify the reconciler reconciles.
		Eventually(func(g Gomega) {
			g.Expect(atomic.LoadUint64(&driverReconciler.ReconcileCount)).Should(BeNumerically(">", 0))
		}, 10*time.Second).Should(Succeed())

		// Verify conversion webhook was called.
		Expect(atomic.LoadUint64(&conversionWebhook.ConversionCount)).Should(BeNumerically(">", 0))

		// Verify the conversion webhook works by getting the Driver as v1 and v2.
		Expect(c.Get(ctx, client.ObjectKeyFromObject(driverV1), driverV1)).To(Succeed())
		driverV2 := &unstructured.Unstructured{}
		driverV2.SetGroupVersionKind(crewv2.GroupVersion.WithKind("Driver"))
		driverV2.SetName("driver1")
		driverV2.SetNamespace(metav1.NamespaceDefault)
		Expect(c.Get(ctx, client.ObjectKeyFromObject(driverV2), driverV2)).To(Succeed())

		// Shutdown the server
		cancel()
	},
		Entry("controller warmup enabled", true),
		Entry("controller warmup not enabled", false),
	)
})

type DriverReconciler struct {
	Client         client.Client
	ReconcileCount uint64
}

func (r *DriverReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling")

	// Fetch the Driver instance.
	cluster := &crewv2.Driver{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	atomic.AddUint64(&r.ReconcileCount, 1)

	return reconcile.Result{}, nil
}

// ConversionWebhook is just a shim around the conversion handler from
// the webhook package. We use it to simulate the behavior of a conversion
// webhook in a real cluster, i.e. the conversion webhook only works after the
// controller Pod is ready (the readiness probe is up).
type ConversionWebhook struct {
	httpClient        http.Client
	conversionHandler http.Handler
	readinessEndpoint string
	ConversionCount   uint64
}

func createConversionWebhook(mgr manager.Manager) *ConversionWebhook {
	conversionHandler := conversion.NewWebhookHandler(mgr.GetScheme())
	httpClient := http.Client{
		// Setting a timeout to not get stuck when calling the readiness probe.
		Timeout: 5 * time.Second,
	}

	// Read the unexported healthProbeListener field of the manager to get the listener address.
	// This is a hack but it's better than using a hard-coded port.
	v := reflect.ValueOf(mgr).Elem()
	field := v.FieldByName("healthProbeListener")
	healthProbeListener := *(*net.Listener)(unsafe.Pointer(field.UnsafeAddr()))
	readinessEndpoint := fmt.Sprint("http://", healthProbeListener.Addr().String(), "/readyz")

	return &ConversionWebhook{
		httpClient:        httpClient,
		conversionHandler: conversionHandler,
		readinessEndpoint: readinessEndpoint,
	}
}

func (c *ConversionWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := c.httpClient.Get(c.readinessEndpoint)
	if err != nil {
		logf.Log.WithName("conversion-webhook").Error(err, "failed to serve conversion: readiness endpoint is not up")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// This simulates the behavior in Kubernetes that conversion webhooks are only served after
		// the controller is ready (and thus the Kubernetes service sends requests to the controller).
		logf.Log.WithName("conversion-webhook").Info("failed to serve conversion: controller is not ready yet")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	atomic.AddUint64(&c.ConversionCount, 1)
	c.conversionHandler.ServeHTTP(w, r)
}
