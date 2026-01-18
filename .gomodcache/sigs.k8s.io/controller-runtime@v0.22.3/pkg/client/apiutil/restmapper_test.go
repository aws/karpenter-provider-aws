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

package apiutil_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"

	_ "github.com/onsi/ginkgo/v2"
	gmg "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// countingRoundTripper is used to count HTTP requests.
type countingRoundTripper struct {
	roundTripper http.RoundTripper
	requestCount int
}

func newCountingRoundTripper(rt http.RoundTripper) *countingRoundTripper {
	return &countingRoundTripper{roundTripper: rt}
}

// RoundTrip implements http.RoundTripper.RoundTrip that additionally counts requests.
func (crt *countingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	crt.requestCount++

	return crt.roundTripper.RoundTrip(r)
}

// GetRequestCount returns how many requests have been made.
func (crt *countingRoundTripper) GetRequestCount() int {
	return crt.requestCount
}

// Reset sets the counter to 0.
func (crt *countingRoundTripper) Reset() {
	crt.requestCount = 0
}

func setupEnvtest(t *testing.T, disableAggregatedDiscovery bool) *rest.Config {
	t.Log("Setup envtest")

	g := gmg.NewWithT(t)
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{"testdata"},
	}
	if disableAggregatedDiscovery {
		testEnv.DownloadBinaryAssets = true
		testEnv.DownloadBinaryAssetsVersion = "v1.28.0"
		binaryAssetsDirectory, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
		g.Expect(err).ToNot(gmg.HaveOccurred())
		testEnv.BinaryAssetsDirectory = binaryAssetsDirectory
		testEnv.ControlPlane.GetAPIServer().Configure().Append("feature-gates", "AggregatedDiscoveryEndpoint=false")
	}

	cfg, err := testEnv.Start()
	g.Expect(err).NotTo(gmg.HaveOccurred())
	g.Expect(cfg).NotTo(gmg.BeNil())

	t.Cleanup(func() {
		t.Log("Stop envtest")
		g.Expect(testEnv.Stop()).To(gmg.Succeed())
	})

	return cfg
}

func TestLazyRestMapperProvider(t *testing.T) {
	for _, aggregatedDiscovery := range []bool{true, false} {
		t.Run("aggregatedDiscovery="+strconv.FormatBool(aggregatedDiscovery), func(t *testing.T) {
			restCfg := setupEnvtest(t, !aggregatedDiscovery)

			t.Run("LazyRESTMapper should fetch data based on the request", func(t *testing.T) {
				g := gmg.NewWithT(t)

				// For each new group it performs just one request to the API server:
				// GET https://host/apis/<group>/<version>

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// There are no requests before any call
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "deployment"}, "v1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("deployment"))
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				mappings, err := lazyRestMapper.RESTMappings(schema.GroupKind{Group: "", Kind: "pod"}, "v1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mappings).To(gmg.HaveLen(1))
				g.Expect(mappings[0].GroupVersionKind.Kind).To(gmg.Equal("pod"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				kind, err := lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kind.Kind).To(gmg.Equal("Ingress"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				kinds, err := lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "authentication.k8s.io", Version: "v1", Resource: "tokenreviews"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kinds).To(gmg.HaveLen(1))
				g.Expect(kinds[0].Kind).To(gmg.Equal("TokenReview"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				resource, err := lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(resource.Resource).To(gmg.Equal("priorityclasses"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				resources, err := lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(resources).To(gmg.HaveLen(1))
				g.Expect(resources[0].Resource).To(gmg.Equal("poddisruptionbudgets"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
			})

			t.Run("LazyRESTMapper should cache fetched data and doesn't perform any additional requests", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "deployment"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("deployment"))
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				// Data taken from cache - there are no more additional requests.

				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "deployment"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("deployment"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				kind, err := lazyRestMapper.KindFor((schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"}))
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kind.Kind).To(gmg.Equal("Deployment"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				resource, err := lazyRestMapper.ResourceFor((schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployment"}))
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(resource.Resource).To(gmg.Equal("deployments"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
			})

			t.Run("LazyRESTMapper should work correctly with empty versions list", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// crew.example.com has 2 versions: v1 and v2

				// If no versions were provided by user, we fetch all of them.
				// Here we expect 4 calls.
				// To initialize:
				// 	#1: GET https://host/api
				// 	#2: GET https://host/apis
				// Then, for each version it performs one request to the API server:
				// 	#3: GET https://host/apis/crew.example.com/v1
				//	#4: GET https://host/apis/crew.example.com/v2
				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				expectedAPIRequestCount := 4
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				// All subsequent calls won't send requests to the server.
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
			})

			t.Run("LazyRESTMapper should work correctly with multiple API group versions", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// We explicitly ask for 2 versions: v1 and v2.
				// For each version it performs one request to the API server:
				// 	#1: GET https://host/apis/crew.example.com/v1
				//	#2: GET https://host/apis/crew.example.com/v2
				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v1", "v2")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				expectedAPIRequestCount := 4
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				// All subsequent calls won't send requests to the server as everything is stored in the cache.
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
			})

			t.Run("LazyRESTMapper should work correctly with different API group versions", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// Now we want resources for crew.example.com/v1 version only.
				// Here we expect 1 call:
				// #1: GET https://host/apis/crew.example.com/v1
				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				// Get additional resources from v2.
				// It sends another request:
				// #2: GET https://host/apis/crew.example.com/v2
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v2")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				if !aggregatedDiscovery {
					expectedAPIRequestCount++
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				// No subsequent calls require additional API requests.
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"}, "v1", "v2")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
			})

			t.Run("LazyRESTMapper should return an error if the group doesn't exist", func(t *testing.T) {
				g := gmg.NewWithT(t)

				// After initialization for each invalid group the mapper performs just 1 request to the API server.

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// A version is specified but the group doesn't exist.
				// For each group, we expect 1 call to the version-specific discovery endpoint:
				// 	#1: GET https://host/apis/<group>/<version>

				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "INVALID1"}, "v1")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
				crt.Reset()

				_, err = lazyRestMapper.RESTMappings(schema.GroupKind{Group: "INVALID2"}, "v1")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(1))

				_, err = lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "INVALID3", Version: "v1", Resource: "invalid"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(2))

				_, err = lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "INVALID4", Version: "v1", Resource: "invalid"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(3))

				_, err = lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "INVALID5", Version: "v1", Resource: "invalid"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(4))

				_, err = lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "INVALID6", Version: "v1", Resource: "invalid"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(5))

				// No version is specified but the group doesn't exist.
				// For each group, we expect 2 calls to discover all group versions:
				// 	#1: GET https://host/api
				// 	#2: GET https://host/apis

				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "INVALID7"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(7))

				_, err = lazyRestMapper.RESTMappings(schema.GroupKind{Group: "INVALID8"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(9))

				_, err = lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "INVALID9", Resource: "invalid"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(11))

				_, err = lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "INVALID10", Resource: "invalid"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(13))

				_, err = lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "INVALID11", Resource: "invalid"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(15))

				_, err = lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "INVALID12", Resource: "invalid"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(17))
			})

			t.Run("LazyRESTMapper should return an error if a resource doesn't exist", func(t *testing.T) {
				g := gmg.NewWithT(t)

				// For each invalid resource the mapper performs just 1 request to the API server.

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "INVALID"}, "v1")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
				crt.Reset()

				_, err = lazyRestMapper.RESTMappings(schema.GroupKind{Group: "", Kind: "INVALID"}, "v1")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(1))

				_, err = lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "INVALID"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(2))

				_, err = lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "authentication.k8s.io", Version: "v1", Resource: "INVALID"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(3))

				_, err = lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "INVALID"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(4))

				_, err = lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "INVALID"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(5))
			})

			t.Run("LazyRESTMapper should return an error if the version doesn't exist", func(t *testing.T) {
				g := gmg.NewWithT(t)

				// After initialization, for each invalid resource mapper performs 1 requests to the API server.

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "deployment"}, "INVALID")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				expectedAPIRequestCount := 3
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
				crt.Reset()

				_, err = lazyRestMapper.RESTMappings(schema.GroupKind{Group: "", Kind: "pod"}, "INVALID")
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(1))

				_, err = lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "networking.k8s.io", Version: "INVALID", Resource: "ingresses"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(2))

				_, err = lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "authentication.k8s.io", Version: "INVALID", Resource: "tokenreviews"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(3))

				_, err = lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "INVALID", Resource: "priorityclasses"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(4))

				_, err = lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "policy", Version: "INVALID", Resource: "poddisruptionbudgets"})
				g.Expect(err).To(gmg.HaveOccurred())
				g.Expect(meta.IsNoMatchError(err)).To(gmg.BeTrue())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(5))
			})

			t.Run("LazyRESTMapper should work correctly if the version isn't specified", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				kind, err := lazyRestMapper.KindFor(schema.GroupVersionResource{Group: "networking.k8s.io", Resource: "ingress"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kind.Version).ToNot(gmg.BeEmpty())

				kinds, err := lazyRestMapper.KindsFor(schema.GroupVersionResource{Group: "authentication.k8s.io", Resource: "tokenreviews"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kinds).ToNot(gmg.BeEmpty())
				g.Expect(kinds[0].Version).ToNot(gmg.BeEmpty())

				resorce, err := lazyRestMapper.ResourceFor(schema.GroupVersionResource{Group: "scheduling.k8s.io", Resource: "priorityclasses"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(resorce.Version).ToNot(gmg.BeEmpty())

				resorces, err := lazyRestMapper.ResourcesFor(schema.GroupVersionResource{Group: "policy", Resource: "poddisruptionbudgets"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(kinds).ToNot(gmg.BeEmpty())
				g.Expect(resorces[0].Version).ToNot(gmg.BeEmpty())
			})

			t.Run("LazyRESTMapper can fetch CRDs if they were created at runtime", func(t *testing.T) {
				g := gmg.NewWithT(t)

				// To fetch all versions mapper does 2 requests:
				// GET https://host/api
				// GET https://host/apis
				// Then, for each version it performs just one request to the API server as usual:
				// GET https://host/apis/<group>/<version>

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// There are no requests before any call
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// Since we don't specify what version we expect, restmapper will fetch them all and search there.
				// To fetch a list of available versions
				//  #1: GET https://host/api
				//  #2: GET https://host/apis
				// Then, for each currently registered version:
				// 	#3: GET https://host/apis/crew.example.com/v1
				//	#4: GET https://host/apis/crew.example.com/v2
				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "driver"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("driver"))
				expectedAPIRequestCount := 4
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))

				s := scheme.Scheme
				err = apiextensionsv1.AddToScheme(s)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				c, err := client.New(restCfg, client.Options{Scheme: s})
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// Register another CRD in runtime - "riders.crew.example.com".
				createNewCRD(t.Context(), g, c, "crew.example.com", "Rider", "riders")

				// Wait a bit until the CRD is registered.
				g.Eventually(func() error {
					_, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "rider"})
					return err
				}).Should(gmg.Succeed())

				// Since we don't specify what version we expect, restmapper will fetch them all and search there.
				// To fetch a list of available versions
				//  #1: GET https://host/api
				//  #2: GET https://host/apis
				// Then, for each currently registered version:
				// 	#3: GET https://host/apis/crew.example.com/v1
				//	#4: GET https://host/apis/crew.example.com/v2
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: "crew.example.com", Kind: "rider"})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal("rider"))
			})

			t.Run("LazyRESTMapper should invalidate the group cache if a version is not found", func(t *testing.T) {
				g := gmg.NewWithT(t)

				httpClient, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				crt := newCountingRoundTripper(httpClient.Transport)
				httpClient.Transport = crt

				lazyRestMapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				s := scheme.Scheme
				err = apiextensionsv1.AddToScheme(s)
				g.Expect(err).NotTo(gmg.HaveOccurred())

				c, err := client.New(restCfg, client.Options{Scheme: s})
				g.Expect(err).NotTo(gmg.HaveOccurred())

				// Register a new CRD ina  new group to avoid collisions when deleting versions - "taxis.inventory.example.com".
				group := "inventory.example.com"
				kind := "Taxi"
				plural := "taxis"
				crdName := plural + "." + group
				// Create a CRD with two versions: v1alpha1 and v1 where both are served and
				// v1 is the storage version so we can easily remove v1alpha1 later.
				crd := newCRD(t.Context(), g, c, group, kind, plural)
				v1alpha1 := crd.Spec.Versions[0]
				v1alpha1.Name = "v1alpha1"
				v1alpha1.Storage = false
				v1alpha1.Served = true
				v1 := crd.Spec.Versions[0]
				v1.Name = "v1"
				v1.Storage = true
				v1.Served = true
				crd.Spec.Versions = []apiextensionsv1.CustomResourceDefinitionVersion{v1alpha1, v1}
				g.Expect(c.Create(t.Context(), crd)).To(gmg.Succeed())
				t.Cleanup(func() {
					g.Expect(c.Delete(context.Background(), crd)).To(gmg.Succeed()) //nolint:forbidigo //t.Context is cancelled in t.Cleanup
				})

				// Wait until the CRD is registered.
				discHTTP, err := rest.HTTPClientFor(restCfg)
				g.Expect(err).NotTo(gmg.HaveOccurred())
				discClient, err := discovery.NewDiscoveryClientForConfigAndClient(restCfg, discHTTP)
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Eventually(func(g gmg.Gomega) {
					_, err = discClient.ServerResourcesForGroupVersion(group + "/v1")
					g.Expect(err).NotTo(gmg.HaveOccurred())
				}).Should(gmg.Succeed(), "v1 should be available")

				// There are no requests before any call
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// Since we don't specify what version we expect, restmapper will fetch them all and search there.
				// To fetch a list of available versions
				//  #1: GET https://host/api
				//  #2: GET https://host/apis
				// Then, for all available versions:
				// 	#3: GET https://host/apis/inventory.example.com/v1alpha1
				//	#4: GET https://host/apis/inventory.example.com/v1
				// This should fill the cache for apiGroups and versions.
				mapping, err := lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.GroupVersionKind.Kind).To(gmg.Equal(kind))
				expectedAPIRequestCount := 4
				if aggregatedDiscovery {
					expectedAPIRequestCount = 2
				}
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(expectedAPIRequestCount))
				crt.Reset() // We reset the counter to check how many additional requests are made later.

				// At this point v1alpha1 should be cached
				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, "v1alpha1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// We update the CRD to only have v1 version.
				g.Expect(c.Get(t.Context(), types.NamespacedName{Name: crdName}, crd)).To(gmg.Succeed())
				for _, version := range crd.Spec.Versions {
					if version.Name == "v1" {
						v1 = version
						break
					}
				}
				crd.Spec.Versions = []apiextensionsv1.CustomResourceDefinitionVersion{v1}
				g.Expect(c.Update(t.Context(), crd)).To(gmg.Succeed())

				// We wait until v1alpha1 is not available anymore.
				g.Eventually(func(g gmg.Gomega) {
					_, err = discClient.ServerResourcesForGroupVersion(group + "/v1alpha1")
					g.Expect(apierrors.IsNotFound(err)).To(gmg.BeTrue(), "v1alpha1 should not be available anymore")
				}).Should(gmg.Succeed())

				// Although v1alpha1 is not available anymore, the cache is not invalidated yet so it should return a mapping.
				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, "v1alpha1")
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(0))

				// We request Limo, which is not in the mapper because it doesn't exist.
				// This will trigger a reload of the lazy mapper cache.
				// Reloading the cache will read v2 again and since it's not available anymore, it should invalidate the cache.
				// 	#1: GET https://host/apis/inventory.example.com/v1alpha1
				// 	#2: GET https://host/apis/inventory.example.com/v1
				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: "Limo"})
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(2))
				crt.Reset()

				// Now we request v1alpha1 again and it should return an error since the cache was invalidated.
				// 	#1: GET https://host/apis/inventory.example.com/v1alpha1
				_, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, "v1alpha1")
				g.Expect(err).To(beNoMatchError())
				g.Expect(crt.GetRequestCount()).To(gmg.Equal(1))

				// Verify that when requesting the mapping without a version, it doesn't error
				// and it returns v1.
				mapping, err = lazyRestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
				g.Expect(err).NotTo(gmg.HaveOccurred())
				g.Expect(mapping.Resource.Version).To(gmg.Equal("v1"))
			})

			t.Run("Restmapper should consistently return the preferred version", func(t *testing.T) {
				g := gmg.NewWithT(t)

				wg := sync.WaitGroup{}
				wg.Add(50)
				for i := 0; i < 50; i++ {
					go func() {
						defer wg.Done()
						httpClient, err := rest.HTTPClientFor(restCfg)
						g.Expect(err).NotTo(gmg.HaveOccurred())

						mapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
						g.Expect(err).NotTo(gmg.HaveOccurred())

						mapping, err := mapper.RESTMapping(schema.GroupKind{
							Group: "crew.example.com",
							Kind:  "Driver",
						})
						g.Expect(err).NotTo(gmg.HaveOccurred())
						// APIServer seems to have a heuristic to prefer the higher
						// version number.
						g.Expect(mapping.GroupVersionKind.Version).To(gmg.Equal("v2"))
					}()
				}
				wg.Wait()
			})
		})
	}
}

// createNewCRD creates a new CRD with the given group, kind, and plural and returns it.
func createNewCRD(ctx context.Context, g gmg.Gomega, c client.Client, group, kind, plural string) *apiextensionsv1.CustomResourceDefinition {
	newCRD := newCRD(ctx, g, c, group, kind, plural)
	g.Expect(c.Create(ctx, newCRD)).To(gmg.Succeed())

	return newCRD
}

// newCRD returns a new CRD with the given group, kind, and plural.
func newCRD(ctx context.Context, g gmg.Gomega, c client.Client, group, kind, plural string) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
	g.Expect(err).NotTo(gmg.HaveOccurred())
	g.Expect(crd.Spec.Names.Kind).To(gmg.Equal("Driver"))

	newCRD := &apiextensionsv1.CustomResourceDefinition{}
	crd.DeepCopyInto(newCRD)
	newCRD.Spec.Group = group
	newCRD.Name = plural + "." + group
	newCRD.Spec.Names = apiextensionsv1.CustomResourceDefinitionNames{
		Kind:   kind,
		Plural: plural,
	}
	newCRD.ResourceVersion = ""

	return newCRD
}

func beNoMatchError() gomegatypes.GomegaMatcher {
	return &errorMatcher{
		checkFunc: meta.IsNoMatchError,
		message:   "NoMatch",
	}
}

type errorMatcher struct {
	checkFunc func(error) bool
	message   string
}

func (e *errorMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil {
		return false, nil
	}

	actualErr, actualOk := actual.(error)
	if !actualOk {
		return false, fmt.Errorf("expected an error-type. got:\n%s", format.Object(actual, 1))
	}

	return e.checkFunc(actualErr), nil
}

func (e *errorMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("to be %s error", e.message))
}

func (e *errorMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("not to be %s error", e.message))
}
