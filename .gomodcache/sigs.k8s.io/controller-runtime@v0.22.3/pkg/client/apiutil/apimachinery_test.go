/*
Copyright 2024 The Kubernetes Authors.

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

package apiutil_test

import (
	"context"
	"strconv"
	"testing"

	gmg "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TestApiMachinery(t *testing.T) {
	for _, aggregatedDiscovery := range []bool{true, false} {
		t.Run("aggregatedDiscovery="+strconv.FormatBool(aggregatedDiscovery), func(t *testing.T) {
			restCfg := setupEnvtest(t, !aggregatedDiscovery)

			// Details of the GVK registered at initialization.
			initialGvk := metav1.GroupVersionKind{
				Group:   "crew.example.com",
				Version: "v1",
				Kind:    "Driver",
			}

			// A set of GVKs to register at runtime with varying properties.
			runtimeGvks := []struct {
				name   string
				gvk    metav1.GroupVersionKind
				plural string
			}{
				{
					name: "new Kind and Version added to existing Group",
					gvk: metav1.GroupVersionKind{
						Group:   "crew.example.com",
						Version: "v1alpha1",
						Kind:    "Passenger",
					},
					plural: "passengers",
				},
				{
					name: "new Kind added to existing Group and Version",
					gvk: metav1.GroupVersionKind{
						Group:   "crew.example.com",
						Version: "v1",
						Kind:    "Garage",
					},
					plural: "garages",
				},
				{
					name: "new GVK",
					gvk: metav1.GroupVersionKind{
						Group:   "inventory.example.com",
						Version: "v1",
						Kind:    "Taxi",
					},
					plural: "taxis",
				},
			}

			t.Run("IsGVKNamespaced should report scope for GVK registered at initialization", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				s := scheme.Scheme
				err = apiextensionsv1.AddToScheme(s)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// Query the scope of a GVK that was registered at initialization.
				scope, err := apiutil.IsGVKNamespaced(
					schema.GroupVersionKind(initialGvk),
					lazyRestMapper,
				)
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(scope).To(gmg.BeTrue())
			})

			for _, runtimeGvk := range runtimeGvks {
				t.Run("IsGVKNamespaced should report scope for "+runtimeGvk.name, func(t *testing.T) {
					g := gmg.NewWithT(t)

					httpClient, err := rest.HTTPClientFor(restCfg)
					g.Expect(err).NotTo(gmg.HaveOccurred())

					lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
					g.Expect(err).NotTo(gmg.HaveOccurred())

					s := scheme.Scheme
					err = apiextensionsv1.AddToScheme(s)
					g.Expect(err).NotTo(gmg.HaveOccurred())

					c, err := client.New(restCfg, client.Options{Scheme: s})
					g.Expect(err).NotTo(gmg.HaveOccurred())

					// Run a valid query to initialize cache.
					scope, err := apiutil.IsGVKNamespaced(
						schema.GroupVersionKind(initialGvk),
						lazyRestMapper,
					)
					g.Expect(err).NotTo(gmg.HaveOccurred())
					g.Expect(scope).To(gmg.BeTrue())

					// Register a new CRD at runtime.
					crd := newCRD(t.Context(), g, c, runtimeGvk.gvk.Group, runtimeGvk.gvk.Kind, runtimeGvk.plural)
					version := crd.Spec.Versions[0]
					version.Name = runtimeGvk.gvk.Version
					version.Storage = true
					version.Served = true
					crd.Spec.Versions = []apiextensionsv1.CustomResourceDefinitionVersion{version}
					crd.Spec.Scope = apiextensionsv1.NamespaceScoped

					g.Expect(c.Create(t.Context(), crd)).To(gmg.Succeed())
					t.Cleanup(func() {
						g.Expect(c.Delete(context.Background(), crd)).To(gmg.Succeed()) //nolint:forbidigo //t.Context is cancelled in t.Cleanup
					})

					// Wait until the CRD is registered.
					g.Eventually(func(g gmg.Gomega) {
						isRegistered, err := isCrdRegistered(restCfg, runtimeGvk.gvk)
						g.Expect(err).NotTo(gmg.HaveOccurred())
						g.Expect(isRegistered).To(gmg.BeTrue())
					}).Should(gmg.Succeed(), "GVK should be available")

					// Query the scope of the GVK registered at runtime.
					scope, err = apiutil.IsGVKNamespaced(
						schema.GroupVersionKind(runtimeGvk.gvk),
						lazyRestMapper,
					)
					g.Expect(err).NotTo(gmg.HaveOccurred())
					g.Expect(scope).To(gmg.BeTrue())
				})
			}
		})
	}
}

// Check if a slice of APIResource contains a given Kind.
func kindInAPIResources(resources *metav1.APIResourceList, kind string) bool {
	for _, res := range resources.APIResources {
		if res.Kind == kind {
			return true
		}
	}
	return false
}

// Check if a CRD has registered with the API server using a DiscoveryClient.
func isCrdRegistered(cfg *rest.Config, gvk metav1.GroupVersionKind) (bool, error) {
	discHTTP, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return false, err
	}

	discClient, err := discovery.NewDiscoveryClientForConfigAndClient(cfg, discHTTP)
	if err != nil {
		return false, err
	}

	resources, err := discClient.ServerResourcesForGroupVersion(gvk.Group + "/" + gvk.Version)
	if err != nil {
		return false, err
	}

	return kindInAPIResources(resources, gvk.Kind), nil
}
