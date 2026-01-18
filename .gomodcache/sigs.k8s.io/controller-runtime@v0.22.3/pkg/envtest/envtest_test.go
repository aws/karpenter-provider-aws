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

package envtest

import (
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Test", func() {
	var crds []*apiextensionsv1.CustomResourceDefinition
	var err error
	var s *runtime.Scheme
	var c client.Client

	var validDirectory = filepath.Join(".", "testdata")
	var invalidDirectory = "fake"

	var teardownTimeoutSeconds float64 = 10

	// Initialize the client
	BeforeEach(func() {
		crds = []*apiextensionsv1.CustomResourceDefinition{}
		s = scheme.Scheme
		err = apiextensionsv1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())

		c, err = client.New(env.Config, client.Options{Scheme: s})
		Expect(err).NotTo(HaveOccurred())
	})

	// Cleanup CRDs
	AfterEach(func(ctx SpecContext) {
		for _, crd := range crds {
			// Delete only if CRD exists.
			crdObjectKey := client.ObjectKey{
				Name: crd.GetName(),
			}
			var placeholder apiextensionsv1.CustomResourceDefinition
			if err = c.Get(ctx, crdObjectKey, &placeholder); err != nil &&
				apierrors.IsNotFound(err) {
				// CRD doesn't need to be deleted.
				continue
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(c.Delete(ctx, crd)).To(Succeed())
			Eventually(func() bool {
				err := c.Get(ctx, crdObjectKey, &placeholder)
				return apierrors.IsNotFound(err)
			}, 5*time.Second).Should(BeTrue())
		}
	}, teardownTimeoutSeconds)

	Describe("InstallCRDs", func() {
		It("should install the unserved CRDs into the cluster", func(ctx SpecContext) {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{filepath.Join(".", "testdata", "crds", "examplecrd_unserved.yaml")},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to find the CRDs

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "frigates.ship.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Frigate"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "ship.example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "frigates",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  false,
							},
							{
								Name:    "v1beta1",
								Storage: false,
								Served:  false,
							},
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should install the CRDs into the cluster using directory", func(ctx SpecContext) {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{validDirectory},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to find the CRDs

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "foos.bar.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Foo"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "bazs.qux.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Baz"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "captains.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Captain"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "firstmates.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("FirstMate"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Driver"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "foos",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "qux.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "bazs",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "captains",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "firstmates",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "drivers",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
							},
							{
								Name:    "v2",
								Storage: false,
								Served:  true,
							},
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should install the CRDs into the cluster using file", func(ctx SpecContext) {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{filepath.Join(".", "testdata", "crds", "examplecrd3.yaml")},
			})
			Expect(err).NotTo(HaveOccurred())

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "configs.foo.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Config"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "foo.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "configs",
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be able to install CRDs using multiple files", func() {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{
					filepath.Join(".", "testdata", "examplecrd.yaml"),
					filepath.Join(".", "testdata", "examplecrd_v1.yaml"),
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(crds).To(HaveLen(2))
		})

		It("should filter out already existent CRD", func(ctx SpecContext) {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{
					filepath.Join(".", "testdata"),
					filepath.Join(".", "testdata", "examplecrd1.yaml"),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "foos.bar.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Foo"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "foos",
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return an not error if the directory doesn't exist", func() {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{Paths: []string{invalidDirectory}})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if the directory doesn't exist", func() {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{invalidDirectory}, ErrorIfPathMissing: true,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the file doesn't exist", func() {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{Paths: []string{
				filepath.Join(".", "testdata", "fake.yaml")}, ErrorIfPathMissing: true,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the resource group version isn't found", func() {
			// Wait for a CRD where the Group and Version don't exist
			err := WaitForCRDs(env.Config,
				[]*apiextensionsv1.CustomResourceDefinition{
					{
						Spec: apiextensionsv1.CustomResourceDefinitionSpec{
							Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Storage: true,
									Served:  true,
									Schema: &apiextensionsv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
									},
								},
							},
							Names: apiextensionsv1.CustomResourceDefinitionNames{
								Plural: "notfound",
							}},
					},
				},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the resource isn't found in the group version", func() {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{"."},
			})
			Expect(err).NotTo(HaveOccurred())

			// Wait for a CRD that doesn't exist, but the Group and Version do
			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "qux.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "bazs",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "fake",
						}},
				}},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).To(HaveOccurred())
		})

		It("should reinstall the CRDs if already present in the cluster", func(ctx SpecContext) {

			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{filepath.Join(".", "testdata")},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to find the CRDs

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "foos.bar.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Foo"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "bazs.qux.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Baz"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "captains.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Captain"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "firstmates.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("FirstMate"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Driver"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "foos",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "qux.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "bazs",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "captains",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "firstmates",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "drivers",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
							},
							{
								Name:    "v2",
								Storage: false,
								Served:  true,
							},
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())

			// Try to re-install the CRDs

			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{filepath.Join(".", "testdata")},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to find the CRDs

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "foos.bar.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Foo"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "bazs.qux.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Baz"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "captains.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Captain"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "firstmates.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("FirstMate"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Driver"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "foos",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "qux.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "bazs",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "captains",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "firstmates",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "drivers",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
							},
							{
								Name:    "v2",
								Storage: false,
								Served:  true,
							},
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should set a working KubeConfig", func(ctx SpecContext) {
		kubeconfigRESTConfig, err := clientcmd.RESTConfigFromKubeConfig(env.KubeConfig)
		Expect(err).ToNot(HaveOccurred())
		kubeconfigClient, err := client.New(kubeconfigRESTConfig, client.Options{Scheme: s})
		Expect(err).NotTo(HaveOccurred())
		Expect(kubeconfigClient.List(ctx, &apiextensionsv1.CustomResourceDefinitionList{})).To(Succeed())
	})

	It("should update CRDs if already present in the cluster", func(ctx SpecContext) {

		// Install only the CRDv1 multi-version example
		crds, err = InstallCRDs(env.Config, CRDInstallOptions{
			Paths: []string{filepath.Join(".", "testdata")},
		})
		Expect(err).NotTo(HaveOccurred())

		// Expect to find the CRDs

		crd := &apiextensionsv1.CustomResourceDefinition{}
		err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
		Expect(err).NotTo(HaveOccurred())
		Expect(crd.Spec.Names.Kind).To(Equal("Driver"))
		Expect(len(crd.Spec.Versions)).To(BeEquivalentTo(2))

		// Store resource version for comparison later on
		firstRV := crd.ResourceVersion

		err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
			{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "crew.example.com",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural: "drivers",
					},
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Storage: true,
							Served:  true,
						},
						{
							Name:    "v2",
							Storage: false,
							Served:  true,
						},
					}},
			},
		},
			CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
		)
		Expect(err).NotTo(HaveOccurred())

		// Add one more version and update
		_, err = InstallCRDs(env.Config, CRDInstallOptions{
			Paths: []string{filepath.Join(".", "testdata", "crdv1_updated")},
		})
		Expect(err).NotTo(HaveOccurred())

		// Expect to find updated CRD

		crd = &apiextensionsv1.CustomResourceDefinition{}
		err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
		Expect(err).NotTo(HaveOccurred())
		Expect(crd.Spec.Names.Kind).To(Equal("Driver"))
		Expect(len(crd.Spec.Versions)).To(BeEquivalentTo(3))
		Expect(crd.ResourceVersion).NotTo(BeEquivalentTo(firstRV))

		err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
			{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: "crew.example.com",
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural: "drivers",
					},
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Storage: true,
							Served:  true,
						},
						{
							Name:    "v2",
							Storage: false,
							Served:  true,
						},
						{
							Name:    "v3",
							Storage: false,
							Served:  true,
						},
					}},
			},
		},
			CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("UninstallCRDs", func() {
		It("should uninstall the CRDs from the cluster", func(ctx SpecContext) {
			crds, err = InstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{validDirectory},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to find the CRDs

			crd := &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "foos.bar.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Foo"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "bazs.qux.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Baz"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "captains.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Captain"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "firstmates.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("FirstMate"))

			crd = &apiextensionsv1.CustomResourceDefinition{}
			err = c.Get(ctx, types.NamespacedName{Name: "drivers.crew.example.com"}, crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Names.Kind).To(Equal("Driver"))

			err = WaitForCRDs(env.Config, []*apiextensionsv1.CustomResourceDefinition{
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "bar.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "foos",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "qux.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "bazs",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "captains",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Storage: true,
								Served:  true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
								},
							},
						},
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "firstmates",
						}},
				},
				{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "crew.example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Plural: "drivers",
						},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Storage: true,
								Served:  true,
							},
							{
								Name:    "v2",
								Storage: false,
								Served:  true,
							},
						}},
				},
			},
				CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
			)
			Expect(err).NotTo(HaveOccurred())

			err = UninstallCRDs(env.Config, CRDInstallOptions{
				Paths: []string{validDirectory},
			})
			Expect(err).NotTo(HaveOccurred())

			// Expect to NOT find the CRDs

			crds := []string{
				"foos.bar.example.com",
				"bazs.qux.example.com",
				"captains.crew.example.com",
				"firstmates.crew.example.com",
				"drivers.crew.example.com",
			}
			placeholder := &apiextensionsv1.CustomResourceDefinition{}
			Eventually(func() bool {
				for _, crd := range crds {
					err = c.Get(ctx, types.NamespacedName{Name: crd}, placeholder)
					notFound := err != nil && apierrors.IsNotFound(err)
					if !notFound {
						return false
					}
				}
				return true
			}, 20).Should(BeTrue())
		})
	})

	Describe("Start", func() {
		It("should raise an error on invalid dir when flag is enabled", func() {
			env := &Environment{ErrorIfCRDPathMissing: true, CRDDirectoryPaths: []string{invalidDirectory}}
			_, err := env.Start()
			Expect(err).To(HaveOccurred())
			Expect(env.Stop()).To(Succeed())
		})

		It("should not raise an error on invalid dir when flag is disabled", func() {
			env := &Environment{ErrorIfCRDPathMissing: false, CRDDirectoryPaths: []string{invalidDirectory}}
			_, err := env.Start()
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Stop()).To(Succeed())
		})
	})

	Describe("Stop", func() {
		It("should cleanup webhook /tmp folder with no error when using existing cluster", func() {
			env := &Environment{}
			_, err := env.Start()
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Stop()).To(Succeed())

			// check if the /tmp/envtest-serving-certs-* dir doesnt exists any more
			Expect(env.WebhookInstallOptions.LocalServingCertDir).ShouldNot(BeADirectory())
		})
	})
})
