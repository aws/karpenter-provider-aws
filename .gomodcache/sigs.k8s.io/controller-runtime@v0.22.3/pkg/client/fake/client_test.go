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

package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientgoapplyconfigurations "k8s.io/client-go/applyconfigurations"
	corev1applyconfigurations "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	machineIDFromStatusUpdate = "machine-id-from-status-update"
	cidrFromStatusUpdate      = "cidr-from-status-update"
)

var _ = Describe("Fake client", func() {
	var dep *appsv1.Deployment
	var dep2 *appsv1.Deployment
	var cm *corev1.ConfigMap
	var cl client.WithWatch

	BeforeEach(func() {
		replicas := int32(1)
		dep = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-deployment",
				Namespace:       "ns1",
				ResourceVersion: trackerAddResourceVersion,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RecreateDeploymentStrategyType,
				},
			},
		}
		dep2 = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-2",
				Namespace: "ns1",
				Labels: map[string]string{
					"test-label": "label-value",
				},
				ResourceVersion: trackerAddResourceVersion,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-cm",
				Namespace:       "ns2",
				ResourceVersion: trackerAddResourceVersion,
			},
			Data: map[string]string{
				"test-key": "test-value",
			},
		}
	})

	AssertClientWithoutIndexBehavior := func() {
		It("should be able to Get", func(ctx SpecContext) {
			By("Getting a deployment")
			namespacedName := types.NamespacedName{
				Name:      "test-deployment",
				Namespace: "ns1",
			}
			obj := &appsv1.Deployment{}
			err := cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(Equal(dep))
		})

		It("should be able to Get using unstructured", func(ctx SpecContext) {
			By("Getting a deployment")
			namespacedName := types.NamespacedName{
				Name:      "test-deployment",
				Namespace: "ns1",
			}
			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion("apps/v1")
			obj.SetKind("Deployment")
			err := cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to List", func(ctx SpecContext) {
			By("Listing all deployments in a namespace")
			list := &appsv1.DeploymentList{}
			err := cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(2))
			Expect(list.Items).To(ConsistOf(*dep, *dep2))
		})

		It("should be able to List using unstructured list", func(ctx SpecContext) {
			By("Listing all deployments in a namespace")
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion("apps/v1")
			list.SetKind("DeploymentList")
			err := cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("apps/v1"))
			Expect(list.GetKind()).To(Equal("DeploymentList"))
			Expect(list.Items).To(HaveLen(2))
		})

		It("should be able to List using unstructured list when setting a non-list kind", func(ctx SpecContext) {
			By("Listing all deployments in a namespace")
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion("apps/v1")
			list.SetKind("Deployment")
			err := cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("apps/v1"))
			Expect(list.GetKind()).To(Equal("Deployment"))
			Expect(list.Items).To(HaveLen(2))
		})

		It("should be able to retrieve registered objects that got manipulated as unstructured", func(ctx SpecContext) {
			list := func() {
				By("Listing all endpoints in a namespace")
				list := &unstructured.UnstructuredList{}
				list.SetAPIVersion("v1")
				list.SetKind("EndpointsList")
				err := cl.List(ctx, list, client.InNamespace("ns1"))
				Expect(err).ToNot(HaveOccurred())
				Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("v1"))
				Expect(list.GetKind()).To(Equal("EndpointsList"))
				Expect(list.Items).To(HaveLen(1))
			}

			unstructuredEndpoint := func() *unstructured.Unstructured {
				item := &unstructured.Unstructured{}
				item.SetAPIVersion("v1")
				item.SetKind("Endpoints")
				item.SetName("test-endpoint")
				item.SetNamespace("ns1")
				return item
			}

			By("Adding the object during client initialization")
			cl = NewClientBuilder().WithRuntimeObjects(unstructuredEndpoint()).Build()
			list()
			Expect(cl.Delete(ctx, unstructuredEndpoint())).To(Succeed())

			By("Creating an object")
			item := unstructuredEndpoint()
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())
			list()

			By("Updating the object")
			item.SetAnnotations(map[string]string{"foo": "bar"})
			err = cl.Update(ctx, item)
			Expect(err).ToNot(HaveOccurred())
			list()

			By("Patching the object")
			old := item.DeepCopy()
			item.SetAnnotations(map[string]string{"bar": "baz"})
			err = cl.Patch(ctx, item, client.MergeFrom(old))
			Expect(err).ToNot(HaveOccurred())
			list()
		})

		It("should be able to Create an unregistered type using unstructured", func(ctx SpecContext) {
			item := &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v1")
			item.SetKind("Image")
			item.SetName("my-item")
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to Get an unregisted type using unstructured", func(ctx SpecContext) {
			By("Creating an object of an unregistered type")
			item := &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v2")
			item.SetKind("Image")
			item.SetName("my-item")
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Getting and the object")
			item = &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v2")
			item.SetKind("Image")
			item.SetName("my-item")
			err = cl.Get(ctx, client.ObjectKeyFromObject(item), item)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to List an unregistered type using unstructured with ListKind", func(ctx SpecContext) {
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion("custom/v3")
			list.SetKind("ImageList")
			err := cl.List(ctx, list)
			Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("custom/v3"))
			Expect(list.GetKind()).To(Equal("ImageList"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to List an unregistered type using unstructured with Kind", func(ctx SpecContext) {
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion("custom/v4")
			list.SetKind("Image")
			err := cl.List(ctx, list)
			Expect(err).ToNot(HaveOccurred())
			Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("custom/v4"))
			Expect(list.GetKind()).To(Equal("Image"))
		})

		It("should be able to Update an unregistered type using unstructured", func(ctx SpecContext) {
			By("Creating an object of an unregistered type")
			item := &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v5")
			item.SetKind("Image")
			item.SetName("my-item")
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Updating the object")
			err = unstructured.SetNestedField(item.Object, int64(2), "spec", "replicas")
			Expect(err).ToNot(HaveOccurred())
			err = cl.Update(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			item = &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v5")
			item.SetKind("Image")
			item.SetName("my-item")
			err = cl.Get(ctx, client.ObjectKeyFromObject(item), item)
			Expect(err).ToNot(HaveOccurred())

			By("Inspecting the object")
			value, found, err := unstructured.NestedInt64(item.Object, "spec", "replicas")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(int64(2)))
		})

		It("should be able to Patch an unregistered type using unstructured", func(ctx SpecContext) {
			By("Creating an object of an unregistered type")
			item := &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v6")
			item.SetKind("Image")
			item.SetName("my-item")
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Updating the object")
			original := item.DeepCopy()
			err = unstructured.SetNestedField(item.Object, int64(2), "spec", "replicas")
			Expect(err).ToNot(HaveOccurred())
			err = cl.Patch(ctx, item, client.MergeFrom(original))
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			item = &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v6")
			item.SetKind("Image")
			item.SetName("my-item")
			err = cl.Get(ctx, client.ObjectKeyFromObject(item), item)
			Expect(err).ToNot(HaveOccurred())

			By("Inspecting the object")
			value, found, err := unstructured.NestedInt64(item.Object, "spec", "replicas")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(int64(2)))
		})

		It("should be able to Delete an unregistered type using unstructured", func(ctx SpecContext) {
			By("Creating an object of an unregistered type")
			item := &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v7")
			item.SetKind("Image")
			item.SetName("my-item")
			err := cl.Create(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the object")
			err = cl.Delete(ctx, item)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			item = &unstructured.Unstructured{}
			item.SetAPIVersion("custom/v7")
			item.SetKind("Image")
			item.SetName("my-item")
			err = cl.Get(ctx, client.ObjectKeyFromObject(item), item)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should be able to retrieve objects by PartialObjectMetadata", func(ctx SpecContext) {
			By("Creating a Resource")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			err := cl.Create(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			By("Fetching the resource using a PartialObjectMeta")
			partialObjMeta := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			partialObjMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

			err = cl.Get(ctx, client.ObjectKeyFromObject(partialObjMeta), partialObjMeta)
			Expect(err).ToNot(HaveOccurred())

			Expect(partialObjMeta.Kind).To(Equal("Secret"))
			Expect(partialObjMeta.APIVersion).To(Equal("v1"))
		})

		It("should support filtering by labels and their values", func(ctx SpecContext) {
			By("Listing deployments with a particular label and value")
			list := &appsv1.DeploymentList{}
			err := cl.List(ctx, list, client.InNamespace("ns1"),
				client.MatchingLabels(map[string]string{
					"test-label": "label-value",
				}))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items).To(ConsistOf(*dep2))
		})

		It("should support filtering by label existence", func(ctx SpecContext) {
			By("Listing deployments with a particular label")
			list := &appsv1.DeploymentList{}
			err := cl.List(ctx, list, client.InNamespace("ns1"),
				client.HasLabels{"test-label"})
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items).To(ConsistOf(*dep2))
		})

		It("should be able to Create", func(ctx SpecContext) {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-test-cm",
					Namespace: "ns2",
				},
			}
			err := cl.Create(ctx, newcm)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the new configmap")
			namespacedName := types.NamespacedName{
				Name:      "new-test-cm",
				Namespace: "ns2",
			}
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(Equal(newcm))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1"))
		})

		It("should error on create with set resourceVersion", func(ctx SpecContext) {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "new-test-cm",
					Namespace:       "ns2",
					ResourceVersion: "1",
				},
			}
			err := cl.Create(ctx, newcm)
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should not change the submitted object if Create failed", func(ctx SpecContext) {
			By("Trying to create an existing configmap")
			submitted := cm.DeepCopy()
			submitted.ResourceVersion = ""
			submittedReference := submitted.DeepCopy()
			err := cl.Create(ctx, submitted)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
			Expect(submitted).To(BeComparableTo(submittedReference))
		})

		It("should error on Create with empty Name", func(ctx SpecContext) {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns2",
				},
			}
			err := cl.Create(ctx, newcm)
			Expect(err.Error()).To(Equal("ConfigMap \"\" is invalid: metadata.name: Required value: name is required"))
		})

		It("should error on Update with empty Name", func(ctx SpecContext) {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns2",
				},
			}
			err := cl.Update(ctx, newcm)
			Expect(err.Error()).To(Equal("ConfigMap \"\" is invalid: metadata.name: Required value: name is required"))
		})

		It("should be able to Create with GenerateName", func(ctx SpecContext) {
			By("Creating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "new-test-cm",
					Namespace:    "ns2",
					Labels: map[string]string{
						"test-label": "label-value",
					},
				},
			}
			err := cl.Create(ctx, newcm)
			Expect(err).ToNot(HaveOccurred())

			By("Listing configmaps with a particular label")
			list := &corev1.ConfigMapList{}
			err = cl.List(ctx, list, client.InNamespace("ns2"),
				client.MatchingLabels(map[string]string{
					"test-label": "label-value",
				}))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Name).NotTo(BeEmpty())
		})

		It("should be able to Update", func(ctx SpecContext) {
			By("Updating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-cm",
					Namespace:       "ns2",
					ResourceVersion: "",
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Update(ctx, newcm)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the new configmap")
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "ns2",
			}
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(Equal(newcm))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1000"))
		})

		It("should allow updates with non-set ResourceVersion for a resource that allows unconditional updates", func(ctx SpecContext) {
			By("Updating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "ns2",
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Update(ctx, newcm)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the configmap")
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "ns2",
			}
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(Equal(newcm))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1000"))
		})

		It("should allow patch when the patch sets RV to 'null'", func(ctx SpecContext) {
			cl := NewClientBuilder().Build()
			original := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj",
					Namespace: "ns2",
				}}

			err := cl.Create(ctx, original)
			Expect(err).ToNot(HaveOccurred())

			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      original.Name,
					Namespace: original.Namespace,
					Annotations: map[string]string{
						"foo": "bar",
					},
				}}

			Expect(cl.Patch(ctx, newObj, client.MergeFrom(original))).To(Succeed())

			patched := &corev1.ConfigMap{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(original), patched)).To(Succeed())
			Expect(patched.Annotations).To(Equal(map[string]string{"foo": "bar"}))
		})

		It("should reject updates with non-set ResourceVersion for a resource that doesn't allow unconditional updates", func(ctx SpecContext) {
			By("Creating a new binding")
			binding := &corev1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "ns2",
				},
				Target: corev1.ObjectReference{
					Kind:       "ConfigMap",
					APIVersion: "v1",
					Namespace:  cm.Namespace,
					Name:       cm.Name,
				},
			}
			Expect(cl.Create(ctx, binding)).To(Succeed())

			By("Updating the binding with a new resource lacking resource version")
			newBinding := &corev1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      binding.Name,
					Namespace: binding.Namespace,
				},
				Target: corev1.ObjectReference{
					Namespace: binding.Namespace,
					Name:      "blue",
				},
			}
			Expect(cl.Update(ctx, newBinding)).NotTo(Succeed())
		})

		It("should allow create on update for a resource that allows create on update", func(ctx SpecContext) {
			By("Creating a new lease with update")
			lease := &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-lease",
					Namespace: "ns2",
				},
				Spec: coordinationv1.LeaseSpec{},
			}
			Expect(cl.Create(ctx, lease)).To(Succeed())

			By("Getting the lease")
			namespacedName := types.NamespacedName{
				Name:      lease.Name,
				Namespace: lease.Namespace,
			}
			obj := &coordinationv1.Lease{}
			Expect(cl.Get(ctx, namespacedName, obj)).To(Succeed())
			Expect(obj).To(Equal(lease))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1"))
		})

		It("should reject create on update for a resource that does not allow create on update", func(ctx SpecContext) {
			By("Attemping to create a new configmap with update")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "different-test-cm",
					Namespace: "ns2",
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			Expect(cl.Update(ctx, newcm)).NotTo(Succeed())
		})

		It("should reject updates with non-matching ResourceVersion", func(ctx SpecContext) {
			By("Updating a new configmap")
			newcm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-cm",
					Namespace:       "ns2",
					ResourceVersion: "1",
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Update(ctx, newcm)
			Expect(apierrors.IsConflict(err)).To(BeTrue())

			By("Getting the configmap")
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "ns2",
			}
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).To(Equal(cm))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal(trackerAddResourceVersion))
		})

		It("should reject Delete with a mismatched ResourceVersion", func(ctx SpecContext) {
			bogusRV := "bogus"
			By("Deleting with a mismatched ResourceVersion Precondition")
			err := cl.Delete(ctx, dep, client.Preconditions{ResourceVersion: &bogusRV})
			Expect(apierrors.IsConflict(err)).To(BeTrue())

			list := &appsv1.DeploymentList{}
			err = cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(2))
			Expect(list.Items).To(ConsistOf(*dep, *dep2))
		})

		It("should successfully Delete with a matching ResourceVersion", func(ctx SpecContext) {
			goodRV := trackerAddResourceVersion
			By("Deleting with a matching ResourceVersion Precondition")
			err := cl.Delete(ctx, dep, client.Preconditions{ResourceVersion: &goodRV})
			Expect(err).ToNot(HaveOccurred())

			list := &appsv1.DeploymentList{}
			err = cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items).To(ConsistOf(*dep2))
		})

		It("should be able to Delete with no ResourceVersion Precondition", func(ctx SpecContext) {
			By("Deleting a deployment")
			err := cl.Delete(ctx, dep)
			Expect(err).ToNot(HaveOccurred())

			By("Listing all deployments in the namespace")
			list := &appsv1.DeploymentList{}
			err = cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items).To(ConsistOf(*dep2))
		})

		It("should be able to Delete with no opts even if object's ResourceVersion doesn't match server", func(ctx SpecContext) {
			By("Deleting a deployment")
			depCopy := dep.DeepCopy()
			depCopy.ResourceVersion = "bogus"
			err := cl.Delete(ctx, depCopy)
			Expect(err).ToNot(HaveOccurred())

			By("Listing all deployments in the namespace")
			list := &appsv1.DeploymentList{}
			err = cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items).To(ConsistOf(*dep2))
		})

		It("should handle finalizers on Update", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "delete-with-finalizers",
			}
			By("Updating a new object")
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespacedName.Name,
					Namespace:  namespacedName.Namespace,
					Finalizers: []string{"finalizers.sigs.k8s.io/test"},
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Create(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the object")
			err = cl.Delete(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj.DeletionTimestamp).NotTo(BeNil())

			By("Removing the finalizer")
			obj.Finalizers = []string{}
			err = cl.Update(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj = &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should reject changes to deletionTimestamp on Update", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "reject-with-deletiontimestamp",
			}
			By("Updating a new object")
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Create(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj.DeletionTimestamp).To(BeNil())

			By("Adding deletionTimestamp")
			now := metav1.Now()
			obj.DeletionTimestamp = &now
			err = cl.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			By("Deleting the object")
			err = cl.Delete(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Changing the deletionTimestamp to new value")
			obj = &corev1.ConfigMap{}
			t := metav1.NewTime(time.Now().Add(time.Second))
			obj.DeletionTimestamp = &t
			err = cl.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			By("Removing deletionTimestamp")
			obj.DeletionTimestamp = nil
			err = cl.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

		})

		It("should be able to Delete a Collection", func(ctx SpecContext) {
			By("Deleting a deploymentList")
			err := cl.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())

			By("Listing all deployments in the namespace")
			list := &appsv1.DeploymentList{}
			err = cl.List(ctx, list, client.InNamespace("ns1"))
			Expect(err).ToNot(HaveOccurred())
			Expect(list.Items).To(BeEmpty())
		})

		It("should handle finalizers deleting a collection", func(ctx SpecContext) {
			for i := 0; i < 5; i++ {
				namespacedName := types.NamespacedName{
					Name:      fmt.Sprintf("test-cm-%d", i),
					Namespace: "delete-collection-with-finalizers",
				}
				By("Creating a new object")
				newObj := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:       namespacedName.Name,
						Namespace:  namespacedName.Namespace,
						Finalizers: []string{"finalizers.sigs.k8s.io/test"},
					},
					Data: map[string]string{
						"test-key": "new-value",
					},
				}
				err := cl.Create(ctx, newObj)
				Expect(err).ToNot(HaveOccurred())
			}

			By("Deleting the object")
			err := cl.DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace("delete-collection-with-finalizers"))
			Expect(err).ToNot(HaveOccurred())

			configmaps := corev1.ConfigMapList{}
			err = cl.List(ctx, &configmaps, client.InNamespace("delete-collection-with-finalizers"))
			Expect(err).ToNot(HaveOccurred())

			Expect(configmaps.Items).To(HaveLen(5))
			for _, cm := range configmaps.Items {
				Expect(cm.DeletionTimestamp).NotTo(BeNil())
			}
		})

		It("should be able to watch", func(ctx SpecContext) {
			By("Creating a watch")
			objWatch, err := cl.Watch(ctx, &corev1.ServiceList{})
			Expect(err).NotTo(HaveOccurred())

			defer objWatch.Stop()

			go func() {
				defer GinkgoRecover()
				// It is likely starting a new goroutine is slower than progressing
				// in the outer routine, sleep to make sure this is always true
				time.Sleep(100 * time.Millisecond)

				err := cl.Create(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "for-watch"}})
				Expect(err).ToNot(HaveOccurred())
			}()

			event, ok := <-objWatch.ResultChan()
			Expect(ok).To(BeTrue())
			Expect(event.Type).To(Equal(watch.Added))

			service, ok := event.Object.(*corev1.Service)
			Expect(ok).To(BeTrue())
			Expect(service.Name).To(Equal("for-watch"))
		})

		Context("with the DryRun option", func() {
			It("should not create a new object", func(ctx SpecContext) {
				By("Creating a new configmap with DryRun")
				newcm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "new-test-cm",
						Namespace: "ns2",
					},
				}
				err := cl.Create(ctx, newcm, client.DryRunAll)
				Expect(err).ToNot(HaveOccurred())

				By("Getting the new configmap")
				namespacedName := types.NamespacedName{
					Name:      "new-test-cm",
					Namespace: "ns2",
				}
				obj := &corev1.ConfigMap{}
				err = cl.Get(ctx, namespacedName, obj)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(obj).NotTo(Equal(newcm))
			})

			It("should not Update the object", func(ctx SpecContext) {
				By("Updating a new configmap with DryRun")
				newcm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-cm",
						Namespace:       "ns2",
						ResourceVersion: "1",
					},
					Data: map[string]string{
						"test-key": "new-value",
					},
				}
				err := cl.Update(ctx, newcm, client.DryRunAll)
				Expect(err).ToNot(HaveOccurred())

				By("Getting the new configmap")
				namespacedName := types.NamespacedName{
					Name:      "test-cm",
					Namespace: "ns2",
				}
				obj := &corev1.ConfigMap{}
				err = cl.Get(ctx, namespacedName, obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj).To(Equal(cm))
				Expect(obj.ObjectMeta.ResourceVersion).To(Equal(trackerAddResourceVersion))
			})

			It("Should not Delete the object", func(ctx SpecContext) {
				By("Deleting a configmap with DryRun with Delete()")
				err := cl.Delete(ctx, cm, client.DryRunAll)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting a configmap with DryRun with DeleteAllOf()")
				err = cl.DeleteAllOf(ctx, cm, client.DryRunAll)
				Expect(err).ToNot(HaveOccurred())

				By("Getting the configmap")
				namespacedName := types.NamespacedName{
					Name:      "test-cm",
					Namespace: "ns2",
				}
				obj := &corev1.ConfigMap{}
				err = cl.Get(ctx, namespacedName, obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj).To(Equal(cm))
				Expect(obj.ObjectMeta.ResourceVersion).To(Equal(trackerAddResourceVersion))
			})
		})

		It("should be able to Patch", func(ctx SpecContext) {
			By("Patching a deployment")
			mergePatch, err := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"foo": "bar",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			err = cl.Patch(ctx, dep, client.RawPatch(types.StrategicMergePatchType, mergePatch))
			Expect(err).NotTo(HaveOccurred())

			By("Getting the patched deployment")
			namespacedName := types.NamespacedName{
				Name:      "test-deployment",
				Namespace: "ns1",
			}
			obj := &appsv1.Deployment{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Annotations["foo"]).To(Equal("bar"))
			Expect(obj.ObjectMeta.ResourceVersion).To(Equal("1000"))
		})

		It("should ignore deletionTimestamp without finalizer on Create", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "ignore-deletiontimestamp",
			}
			By("Creating a new object")
			now := metav1.Now()
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:              namespacedName.Name,
					Namespace:         namespacedName.Namespace,
					Finalizers:        []string{"finalizers.sigs.k8s.io/test"},
					DeletionTimestamp: &now,
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}

			err := cl.Create(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj.DeletionTimestamp).To(BeNil())

		})

		It("should reject deletionTimestamp without finalizers on Build", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "reject-deletiontimestamp-no-finalizers",
			}
			By("Build with a new object without finalizer")
			now := metav1.Now()
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:              namespacedName.Name,
					Namespace:         namespacedName.Namespace,
					DeletionTimestamp: &now,
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}

			Expect(func() { NewClientBuilder().WithObjects(obj).Build() }).To(Panic())

			By("Build with a new object with finalizer")
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:              namespacedName.Name,
					Namespace:         namespacedName.Namespace,
					Finalizers:        []string{"finalizers.sigs.k8s.io/test"},
					DeletionTimestamp: &now,
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}

			cl := NewClientBuilder().WithObjects(newObj).Build()

			By("Getting the object")
			obj = &corev1.ConfigMap{}
			err := cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should reject changes to deletionTimestamp on Patch", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "reject-deletiontimestamp",
			}
			By("Creating a new object")
			now := metav1.Now()
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespacedName.Name,
					Namespace:  namespacedName.Namespace,
					Finalizers: []string{"finalizers.sigs.k8s.io/test"},
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Create(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Add a deletionTimestamp")
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:              namespacedName.Name,
					Namespace:         namespacedName.Namespace,
					Finalizers:        []string{},
					DeletionTimestamp: &now,
				},
			}
			err = cl.Patch(ctx, obj, client.MergeFrom(newObj))
			Expect(err).To(HaveOccurred())

			By("Deleting the object")
			err = cl.Delete(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj = &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj.DeletionTimestamp).NotTo(BeNil())

			By("Changing the deletionTimestamp to new value")
			newObj = &corev1.ConfigMap{}
			t := metav1.NewTime(time.Now().Add(time.Second))
			newObj.DeletionTimestamp = &t
			err = cl.Patch(ctx, newObj, client.MergeFrom(obj))
			Expect(err).To(HaveOccurred())

			By("Removing deletionTimestamp")
			newObj = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:              namespacedName.Name,
					Namespace:         namespacedName.Namespace,
					DeletionTimestamp: nil,
				},
			}
			err = cl.Patch(ctx, newObj, client.MergeFrom(obj))
			Expect(err).To(HaveOccurred())

		})

		It("should handle finalizers on Patch", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "delete-with-finalizers",
			}
			By("Creating a new object")
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespacedName.Name,
					Namespace:  namespacedName.Namespace,
					Finalizers: []string{"finalizers.sigs.k8s.io/test"},
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Create(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the object")
			err = cl.Delete(ctx, newObj)
			Expect(err).ToNot(HaveOccurred())

			By("Removing the finalizer")
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespacedName.Name,
					Namespace:  namespacedName.Namespace,
					Finalizers: []string{},
				},
			}
			err = cl.Patch(ctx, obj, client.MergeFrom(newObj))
			Expect(err).ToNot(HaveOccurred())

			By("Getting the object")
			obj = &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, obj)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should remove finalizers of the object on Patch", func(ctx SpecContext) {
			namespacedName := types.NamespacedName{
				Name:      "test-cm",
				Namespace: "patch-finalizers-in-obj",
			}
			By("Creating a new object")
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespacedName.Name,
					Namespace:  namespacedName.Namespace,
					Finalizers: []string{"finalizers.sigs.k8s.io/test"},
				},
				Data: map[string]string{
					"test-key": "new-value",
				},
			}
			err := cl.Create(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("Removing the finalizer")
			mergePatch, err := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"$deleteFromPrimitiveList/finalizers": []string{
						"finalizers.sigs.k8s.io/test",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			err = cl.Patch(ctx, obj, client.RawPatch(types.StrategicMergePatchType, mergePatch))
			Expect(err).ToNot(HaveOccurred())

			By("Check the finalizer has been removed in the object")
			Expect(obj.Finalizers).To(BeEmpty())

			By("Check the finalizer has been removed in client")
			newObj := &corev1.ConfigMap{}
			err = cl.Get(ctx, namespacedName, newObj)
			Expect(err).ToNot(HaveOccurred())
			Expect(newObj.Finalizers).To(BeEmpty())
		})

	}

	Context("with default scheme.Scheme", func() {
		BeforeEach(func() {
			cl = NewClientBuilder().
				WithObjects(dep, dep2, cm).
				Build()
		})
		AssertClientWithoutIndexBehavior()
	})

	Context("with given scheme", func() {
		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(appsv1.AddToScheme(scheme)).To(Succeed())
			Expect(coordinationv1.AddToScheme(scheme)).To(Succeed())
			cl = NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cm).
				WithLists(&appsv1.DeploymentList{Items: []appsv1.Deployment{*dep, *dep2}}).
				Build()
		})
		AssertClientWithoutIndexBehavior()
	})

	Context("with Indexes", func() {
		depReplicasIndexer := func(obj client.Object) []string {
			dep, ok := obj.(*appsv1.Deployment)
			if !ok {
				panic(fmt.Errorf("indexer function for type %T's spec.replicas field received"+
					" object of type %T, this should never happen", appsv1.Deployment{}, obj))
			}
			indexVal := ""
			if dep.Spec.Replicas != nil {
				indexVal = strconv.Itoa(int(*dep.Spec.Replicas))
			}
			return []string{indexVal}
		}

		depStrategyTypeIndexer := func(obj client.Object) []string {
			dep, ok := obj.(*appsv1.Deployment)
			if !ok {
				panic(fmt.Errorf("indexer function for type %T's spec.strategy.type field received"+
					" object of type %T, this should never happen", appsv1.Deployment{}, obj))
			}
			return []string{string(dep.Spec.Strategy.Type)}
		}

		var cb *ClientBuilder
		BeforeEach(func() {
			cb = NewClientBuilder().
				WithObjects(dep, dep2, cm).
				WithIndex(&appsv1.Deployment{}, "spec.replicas", depReplicasIndexer)
		})

		Context("client has just one Index", func() {
			BeforeEach(func() { cl = cb.Build() })

			Context("behavior that doesn't use an Index", func() {
				AssertClientWithoutIndexBehavior()
			})

			Context("filtered List using field selector", func() {
				It("errors when there's no Index for the GroupVersionResource", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("key", "val"),
					}
					list := &corev1.ConfigMapList{}
					err := cl.List(ctx, list, listOpts)
					Expect(err).To(HaveOccurred())
					Expect(list.Items).To(BeEmpty())
				})

				It("errors when there's no Index for the GroupVersionResource with UnstructuredList", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("key", "val"),
					}
					list := &unstructured.UnstructuredList{}
					list.SetAPIVersion("v1")
					list.SetKind("ConfigMapList")
					err := cl.List(ctx, list, listOpts)
					Expect(err).To(HaveOccurred())
					Expect(list.GroupVersionKind().GroupVersion().String()).To(Equal("v1"))
					Expect(list.GetKind()).To(Equal("ConfigMapList"))
					Expect(list.Items).To(BeEmpty())
				})

				It("errors when there's no Index matching the field name", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.paused", "false"),
					}
					list := &appsv1.DeploymentList{}
					err := cl.List(ctx, list, listOpts)
					Expect(err).To(HaveOccurred())
					Expect(list.Items).To(BeEmpty())
				})

				It("errors when field selector uses two requirements", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.AndSelectors(
							fields.OneTermEqualSelector("spec.replicas", "1"),
							fields.OneTermEqualSelector("spec.strategy.type", string(appsv1.RecreateDeploymentStrategyType)),
						)}
					list := &appsv1.DeploymentList{}
					err := cl.List(ctx, list, listOpts)
					Expect(err).To(HaveOccurred())
					Expect(list.Items).To(BeEmpty())
				})

				It("returns two deployments that match the only field selector requirement", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "1"),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(ConsistOf(*dep, *dep2))
				})

				It("returns no object because no object matches the only field selector requirement", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "2"),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(BeEmpty())
				})

				It("returns deployment that matches both the field and label selectors", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "1"),
						LabelSelector: labels.SelectorFromSet(dep2.Labels),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(ConsistOf(*dep2))
				})

				It("returns no object even if field selector matches because label selector doesn't", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "1"),
						LabelSelector: labels.Nothing(),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(BeEmpty())
				})

				It("returns no object even if label selector matches because field selector doesn't", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "2"),
						LabelSelector: labels.Everything(),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(BeEmpty())
				})

				It("supports adding an index at runtime", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("metadata.name", "test-deployment-2"),
					}
					list := &appsv1.DeploymentList{}
					err := cl.List(ctx, list, listOpts)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("no index with name metadata.name has been registered"))

					err = AddIndex(cl, &appsv1.Deployment{}, "metadata.name", func(obj client.Object) []string {
						return []string{obj.GetName()}
					})
					Expect(err).To(Succeed())

					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(ConsistOf(*dep2))
				})

				It("Is not a datarace to add and use indexes in parallel", func(ctx SpecContext) {
					wg := sync.WaitGroup{}
					wg.Add(2)

					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.replicas", "2"),
					}
					go func() {
						defer wg.Done()
						defer GinkgoRecover()
						Expect(cl.List(ctx, &appsv1.DeploymentList{}, listOpts)).To(Succeed())
					}()
					go func() {
						defer wg.Done()
						defer GinkgoRecover()
						err := AddIndex(cl, &appsv1.Deployment{}, "metadata.name", func(obj client.Object) []string {
							return []string{obj.GetName()}
						})
						Expect(err).To(Succeed())
					}()
					wg.Wait()
				})
			})
		})

		Context("client has two Indexes", func() {
			BeforeEach(func() {
				cl = cb.WithIndex(&appsv1.Deployment{}, "spec.strategy.type", depStrategyTypeIndexer).Build()
			})

			Context("behavior that doesn't use an Index", func() {
				AssertClientWithoutIndexBehavior()
			})

			Context("filtered List using field selector", func() {
				It("uses the second index to retrieve the indexed objects when there are matches", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.strategy.type", string(appsv1.RecreateDeploymentStrategyType)),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(ConsistOf(*dep))
				})

				It("uses the second index to retrieve the indexed objects when there are no matches", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.OneTermEqualSelector("spec.strategy.type", string(appsv1.RollingUpdateDeploymentStrategyType)),
					}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(BeEmpty())
				})

				It("no error when field selector uses two requirements", func(ctx SpecContext) {
					listOpts := &client.ListOptions{
						FieldSelector: fields.AndSelectors(
							fields.OneTermEqualSelector("spec.replicas", "1"),
							fields.OneTermEqualSelector("spec.strategy.type", string(appsv1.RecreateDeploymentStrategyType)),
						)}
					list := &appsv1.DeploymentList{}
					Expect(cl.List(ctx, list, listOpts)).To(Succeed())
					Expect(list.Items).To(ConsistOf(*dep))
				})
			})
		})
	})

	It("should set the ResourceVersion to 999 when adding an object to the tracker", func(ctx SpecContext) {
		cl := NewClientBuilder().WithObjects(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}).Build()

		retrieved := &corev1.Secret{}
		Expect(cl.Get(ctx, types.NamespacedName{Name: "cm"}, retrieved)).To(Succeed())

		reference := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				ResourceVersion: "999",
			},
		}
		Expect(retrieved).To(Equal(reference))
	})

	It("should be able to build with given tracker and get resource", func(ctx SpecContext) {
		clientSet := fake.NewSimpleClientset(dep)
		cl := NewClientBuilder().WithRuntimeObjects(dep2).WithObjectTracker(clientSet.Tracker()).Build()

		By("Getting a deployment")
		namespacedName := types.NamespacedName{
			Name:      "test-deployment",
			Namespace: "ns1",
		}
		obj := &appsv1.Deployment{}
		err := cl.Get(ctx, namespacedName, obj)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj).To(BeComparableTo(dep))

		By("Getting a deployment from clientSet")
		csDep2, err := clientSet.AppsV1().Deployments("ns1").Get(ctx, "test-deployment-2", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(csDep2).To(Equal(dep2))

		By("Getting a new deployment")
		namespacedName3 := types.NamespacedName{
			Name:      "test-deployment-3",
			Namespace: "ns1",
		}

		dep3 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-3",
				Namespace: "ns1",
				Labels: map[string]string{
					"test-label": "label-value",
				},
				ResourceVersion: trackerAddResourceVersion,
			},
		}

		_, err = clientSet.AppsV1().Deployments("ns1").Create(ctx, dep3, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		obj = &appsv1.Deployment{}
		err = cl.Get(ctx, namespacedName3, obj)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj).To(BeComparableTo(dep3))
	})

	It("should not change the status of typed objects that have a status subresource on update", func(ctx SpecContext) {
		obj := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		obj.Status.Phase = "Running"
		Expect(cl.Update(ctx, obj)).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Status).To(BeEquivalentTo(corev1.PodStatus{}))
	})

	It("should return a conflict error when an incorrect RV is used on status update", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "node",
				ResourceVersion: trackerAddResourceVersion,
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		obj.Status.Phase = corev1.NodeRunning
		obj.ResourceVersion = "invalid"
		err := cl.Status().Update(ctx, obj)
		Expect(apierrors.IsConflict(err)).To(BeTrue())
	})

	It("should not change non-status field of typed objects that have a status subresource on status update", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{
				PodCIDR: "old-cidr",
			},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					MachineID: "machine-id",
				},
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		objOriginal := obj.DeepCopy()

		obj.Spec.PodCIDR = cidrFromStatusUpdate
		obj.Annotations = map[string]string{
			"some-annotation-key": "some-annotation-value",
		}
		obj.Labels = map[string]string{
			"some-label-key": "some-label-value",
		}

		obj.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cl.Status().Update(ctx, obj)).NotTo(HaveOccurred())

		actual := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).NotTo(HaveOccurred())

		objOriginal.APIVersion = actual.APIVersion
		objOriginal.Kind = actual.Kind
		objOriginal.ResourceVersion = actual.ResourceVersion
		objOriginal.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cmp.Diff(objOriginal, actual)).To(BeEmpty())
	})

	It("should be able to update an object after updating an object's status", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{
				PodCIDR: "old-cidr",
			},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					MachineID: "machine-id",
				},
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		expectedObj := obj.DeepCopy()

		obj.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cl.Status().Update(ctx, obj)).NotTo(HaveOccurred())

		obj.Annotations = map[string]string{
			"some-annotation-key": "some",
		}
		expectedObj.Annotations = map[string]string{
			"some-annotation-key": "some",
		}
		Expect(cl.Update(ctx, obj)).NotTo(HaveOccurred())

		actual := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).NotTo(HaveOccurred())

		expectedObj.APIVersion = actual.APIVersion
		expectedObj.Kind = actual.Kind
		expectedObj.ResourceVersion = actual.ResourceVersion
		expectedObj.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cmp.Diff(expectedObj, actual)).To(BeEmpty())
	})

	It("should be able to update an object's status after updating an object", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{
				PodCIDR: "old-cidr",
			},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					MachineID: "machine-id",
				},
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		expectedObj := obj.DeepCopy()

		obj.Annotations = map[string]string{
			"some-annotation-key": "some",
		}
		expectedObj.Annotations = map[string]string{
			"some-annotation-key": "some",
		}
		Expect(cl.Update(ctx, obj)).NotTo(HaveOccurred())

		obj.Spec.PodCIDR = cidrFromStatusUpdate
		obj.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cl.Status().Update(ctx, obj)).NotTo(HaveOccurred())

		actual := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).NotTo(HaveOccurred())

		expectedObj.APIVersion = actual.APIVersion
		expectedObj.Kind = actual.Kind
		expectedObj.ResourceVersion = actual.ResourceVersion
		expectedObj.Status.NodeInfo.MachineID = machineIDFromStatusUpdate
		Expect(cmp.Diff(expectedObj, actual)).To(BeEmpty())
	})

	It("Should only override status fields of typed objects that have a status subresource on status update", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{
				PodCIDR: "old-cidr",
			},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					MachineID: "machine-id",
				},
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		objOriginal := obj.DeepCopy()

		obj.Status.Phase = corev1.NodeRunning
		Expect(cl.Status().Update(ctx, obj)).NotTo(HaveOccurred())

		actual := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).NotTo(HaveOccurred())

		objOriginal.APIVersion = actual.APIVersion
		objOriginal.Kind = actual.Kind
		objOriginal.ResourceVersion = actual.ResourceVersion
		Expect(cmp.Diff(objOriginal, actual)).ToNot(BeEmpty())
		Expect(objOriginal.Status.NodeInfo.MachineID).To(Equal(actual.Status.NodeInfo.MachineID))
		Expect(objOriginal.Status.Phase).ToNot(Equal(actual.Status.Phase))
	})

	It("should be able to change typed objects that have a scale subresource on patch", func(ctx SpecContext) {
		obj := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "deploy",
			},
		}
		cl := NewClientBuilder().WithObjects(obj).Build()
		objOriginal := obj.DeepCopy()

		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, 2))
		Expect(cl.SubResource("scale").Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch))).NotTo(HaveOccurred())

		actual := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).To(Succeed())

		objOriginal.APIVersion = actual.APIVersion
		objOriginal.Kind = actual.Kind
		objOriginal.ResourceVersion = actual.ResourceVersion
		objOriginal.Spec.Replicas = ptr.To(int32(2))
		Expect(cmp.Diff(objOriginal, actual)).To(BeEmpty())
	})

	It("should not change the status of typed objects that have a status subresource on patch", func(ctx SpecContext) {
		obj := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
		}
		Expect(cl.Create(ctx, obj)).To(Succeed())
		original := obj.DeepCopy()

		obj.Status.Phase = "Running"
		Expect(cl.Patch(ctx, obj, client.MergeFrom(original))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Status).To(BeEquivalentTo(corev1.PodStatus{}))
	})

	It("should not change non-status field of typed objects that have a status subresource on status patch", func(ctx SpecContext) {
		obj := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Spec: corev1.NodeSpec{
				PodCIDR: "old-cidr",
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		objOriginal := obj.DeepCopy()

		obj.Spec.PodCIDR = cidrFromStatusUpdate
		obj.Status.NodeInfo.MachineID = "machine-id"
		Expect(cl.Status().Patch(ctx, obj, client.MergeFrom(objOriginal))).NotTo(HaveOccurred())

		actual := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: obj.Name}}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(actual), actual)).NotTo(HaveOccurred())

		objOriginal.APIVersion = actual.APIVersion
		objOriginal.Kind = actual.Kind
		objOriginal.ResourceVersion = actual.ResourceVersion
		objOriginal.Status.NodeInfo.MachineID = "machine-id"
		Expect(cmp.Diff(objOriginal, actual)).To(BeEmpty())
	})

	It("should Unmarshal the schemaless object with int64 to preserve ints", func(ctx SpecContext) {
		schemeBuilder := &scheme.Builder{GroupVersion: schema.GroupVersion{Group: "test", Version: "v1"}}
		schemeBuilder.Register(&WithSchemalessSpec{})

		scheme := runtime.NewScheme()
		Expect(schemeBuilder.AddToScheme(scheme)).NotTo(HaveOccurred())

		spec := Schemaless{
			"key": int64(1),
		}

		obj := &WithSchemalessSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-foo",
			},
			Spec: spec,
		}
		cl := NewClientBuilder().WithScheme(scheme).WithStatusSubresource(obj).WithObjects(obj).Build()

		Expect(cl.Update(ctx, obj)).To(Succeed())
		Expect(obj.Spec).To(BeEquivalentTo(spec))
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		Expect(obj.Spec).To(BeEquivalentTo(spec))
	})

	It("should Unmarshal the schemaless object with float64 to preserve ints", func(ctx SpecContext) {
		schemeBuilder := &scheme.Builder{GroupVersion: schema.GroupVersion{Group: "test", Version: "v1"}}
		schemeBuilder.Register(&WithSchemalessSpec{})

		scheme := runtime.NewScheme()
		Expect(schemeBuilder.AddToScheme(scheme)).NotTo(HaveOccurred())

		spec := Schemaless{
			"key": 1.1,
		}

		obj := &WithSchemalessSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-foo",
			},
			Spec: spec,
		}
		cl := NewClientBuilder().WithScheme(scheme).WithStatusSubresource(obj).WithObjects(obj).Build()

		Expect(cl.Update(ctx, obj)).To(Succeed())
		Expect(obj.Spec).To(BeEquivalentTo(spec))
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		Expect(obj.Spec).To(BeEquivalentTo(spec))
	})

	It("should not change the status of unstructured objects that are configured to have a status subresource on update", func(ctx SpecContext) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("foo/v1")
		obj.SetKind("Foo")
		obj.SetName("a-foo")

		err := unstructured.SetNestedField(obj.Object, map[string]any{"state": "old"}, "status")
		Expect(err).NotTo(HaveOccurred())

		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		err = unstructured.SetNestedField(obj.Object, map[string]any{"state": "new"}, "status")
		Expect(err).ToNot(HaveOccurred())

		Expect(cl.Update(ctx, obj)).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Object["status"]).To(BeEquivalentTo(map[string]any{"state": "old"}))
	})

	It("should not change non-status fields of unstructured objects that are configured to have a status subresource on status update", func(ctx SpecContext) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("foo/v1")
		obj.SetKind("Foo")
		obj.SetName("a-foo")

		err := unstructured.SetNestedField(obj.Object, "original", "spec")
		Expect(err).NotTo(HaveOccurred())

		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		err = unstructured.SetNestedField(obj.Object, "from-status-update", "spec")
		Expect(err).NotTo(HaveOccurred())
		err = unstructured.SetNestedField(obj.Object, map[string]any{"state": "new"}, "status")
		Expect(err).ToNot(HaveOccurred())

		Expect(cl.Status().Update(ctx, obj)).To(Succeed())
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Object["status"]).To(BeEquivalentTo(map[string]any{"state": "new"}))
		Expect(obj.Object["spec"]).To(BeEquivalentTo("original"))
	})

	It("should not change the status of known unstructured objects that have a status subresource on update", func(ctx SpecContext) {
		obj := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyAlways,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		// update using unstructured
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Pod")
		u.SetName(obj.Name)
		err := cl.Get(ctx, client.ObjectKeyFromObject(u), u)
		Expect(err).NotTo(HaveOccurred())

		err = unstructured.SetNestedField(u.Object, string(corev1.RestartPolicyNever), "spec", "restartPolicy")
		Expect(err).NotTo(HaveOccurred())
		err = unstructured.SetNestedField(u.Object, string(corev1.PodRunning), "status", "phase")
		Expect(err).NotTo(HaveOccurred())

		Expect(cl.Update(ctx, u)).To(Succeed())

		actual := &corev1.Pod{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), actual)).To(Succeed())
		obj.ResourceVersion = actual.ResourceVersion
		// only the spec mutation should persist
		obj.Spec.RestartPolicy = corev1.RestartPolicyNever
		Expect(cmp.Diff(obj, actual)).To(BeEmpty())
	})

	It("should not change non-status field of known unstructured objects that have a status subresource on status update", func(ctx SpecContext) {
		obj := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod",
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyAlways,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}
		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()

		// status update using unstructured
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Pod")
		u.SetName(obj.Name)
		err := cl.Get(ctx, client.ObjectKeyFromObject(u), u)
		Expect(err).NotTo(HaveOccurred())

		err = unstructured.SetNestedField(u.Object, string(corev1.RestartPolicyNever), "spec", "restartPolicy")
		Expect(err).NotTo(HaveOccurred())
		err = unstructured.SetNestedField(u.Object, string(corev1.PodRunning), "status", "phase")
		Expect(err).NotTo(HaveOccurred())

		Expect(cl.Status().Update(ctx, u)).To(Succeed())

		actual := &corev1.Pod{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), actual)).To(Succeed())
		obj.ResourceVersion = actual.ResourceVersion
		// only the status mutation should persist
		obj.Status.Phase = corev1.PodRunning
		Expect(cmp.Diff(obj, actual)).To(BeEmpty())
	})

	It("should not change the status of unstructured objects that are configured to have a status subresource on patch", func(ctx SpecContext) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("foo/v1")
		obj.SetKind("Foo")
		obj.SetName("a-foo")
		cl := NewClientBuilder().WithStatusSubresource(obj).Build()

		Expect(cl.Create(ctx, obj)).To(Succeed())
		original := obj.DeepCopy()

		err := unstructured.SetNestedField(obj.Object, map[string]interface{}{"count": int64(2)}, "status")
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Patch(ctx, obj, client.MergeFrom(original))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Object["status"]).To(BeNil())

	})

	It("should not change non-status fields of unstructured objects that are configured to have a status subresource on status patch", func(ctx SpecContext) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("foo/v1")
		obj.SetKind("Foo")
		obj.SetName("a-foo")

		err := unstructured.SetNestedField(obj.Object, "original", "spec")
		Expect(err).NotTo(HaveOccurred())

		cl := NewClientBuilder().WithStatusSubresource(obj).WithObjects(obj).Build()
		original := obj.DeepCopy()

		err = unstructured.SetNestedField(obj.Object, "from-status-update", "spec")
		Expect(err).NotTo(HaveOccurred())
		err = unstructured.SetNestedField(obj.Object, map[string]any{"state": "new"}, "status")
		Expect(err).ToNot(HaveOccurred())

		Expect(cl.Status().Patch(ctx, obj, client.MergeFrom(original))).To(Succeed())
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

		Expect(obj.Object["status"]).To(BeEquivalentTo(map[string]any{"state": "new"}))
		Expect(obj.Object["spec"]).To(BeEquivalentTo("original"))
	})

	It("should return not found on status update of resources that don't have a status subresource", func(ctx SpecContext) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("foo/v1")
		obj.SetKind("Foo")
		obj.SetName("a-foo")

		cl := NewClientBuilder().WithObjects(obj).Build()

		err := cl.Status().Update(ctx, obj)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	evictionTypes := []client.Object{
		&policyv1beta1.Eviction{},
		&policyv1.Eviction{},
	}
	for _, tp := range evictionTypes {
		It("should delete a pod through the eviction subresource", func(ctx SpecContext) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

			cl := NewClientBuilder().WithObjects(pod).Build()

			err := cl.SubResource("eviction").Create(ctx, pod, tp)
			Expect(err).NotTo(HaveOccurred())

			err = cl.Get(ctx, client.ObjectKeyFromObject(pod), pod)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return not found when attempting to evict a pod that doesn't exist", func(ctx SpecContext) {
			cl := NewClientBuilder().Build()

			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
			err := cl.SubResource("eviction").Create(ctx, pod, tp)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return not found when attempting to evict something other than a pod", func(ctx SpecContext) {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
			cl := NewClientBuilder().WithObjects(ns).Build()

			err := cl.SubResource("eviction").Create(ctx, ns, tp)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("should return an error when using the wrong subresource", func(ctx SpecContext) {
			cl := NewClientBuilder().Build()

			err := cl.SubResource("eviction-subresource").Create(ctx, &corev1.Namespace{}, tp)
			Expect(err).To(HaveOccurred())
		})
	}

	It("should error when creating an eviction with the wrong type", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		err := cl.SubResource("eviction").Create(ctx, &corev1.Pod{}, &corev1.Namespace{})
		Expect(apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("should create a ServiceAccount token through the token subresource", func(ctx SpecContext) {
		sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		cl := NewClientBuilder().WithObjects(sa).Build()

		tokenRequest := &authenticationv1.TokenRequest{}
		err := cl.SubResource("token").Create(ctx, sa, tokenRequest)
		Expect(err).NotTo(HaveOccurred())

		Expect(tokenRequest.Status.Token).NotTo(Equal(""))
		Expect(tokenRequest.Status.ExpirationTimestamp).NotTo(Equal(metav1.Time{}))
	})

	It("should return not found when creating a token for a ServiceAccount that doesn't exist", func(ctx SpecContext) {
		sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		cl := NewClientBuilder().Build()

		err := cl.SubResource("token").Create(ctx, sa, &authenticationv1.TokenRequest{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should error when creating a token with the wrong subresource type", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		err := cl.SubResource("token").Create(ctx, &corev1.ServiceAccount{}, &corev1.Namespace{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("should error when creating a token with the wrong type", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		err := cl.SubResource("token").Create(ctx, &corev1.Secret{}, &authenticationv1.TokenRequest{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should leave typemeta empty on typed get", func(ctx SpecContext) {
		cl := NewClientBuilder().WithObjects(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		}}).Build()

		var pod corev1.Pod
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "foo"}, &pod)).NotTo(HaveOccurred())

		Expect(pod.TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("should leave typemeta empty on typed list", func(ctx SpecContext) {
		cl := NewClientBuilder().WithObjects(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		}}).Build()

		var podList corev1.PodList
		Expect(cl.List(ctx, &podList)).NotTo(HaveOccurred())
		Expect(podList.ListMeta).To(Equal(metav1.ListMeta{}))
		Expect(podList.Items[0].TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("should allow concurrent patches to a configMap", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "0",
			},
		}
		cl := NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

		const tries = 50
		wg := sync.WaitGroup{}
		wg.Add(tries)

		for i := range tries {
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				newObj := obj.DeepCopy()
				newObj.Data = map[string]string{"foo": strconv.Itoa(i)}
				Expect(cl.Patch(ctx, newObj, client.MergeFrom(obj))).To(Succeed())
			}()
		}
		wg.Wait()

		// While the order is not deterministic, there must be $tries distinct updates
		// that each increment the resource version by one
		Expect(cl.Get(ctx, client.ObjectKey{Name: "foo"}, obj)).To(Succeed())
		Expect(obj.ResourceVersion).To(Equal(strconv.Itoa(tries)))
	})

	It("should not allow concurrent patches to a configMap if the patch contains a ResourceVersion", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "0",
			},
		}
		cl := NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
		wg := sync.WaitGroup{}
		wg.Add(5)

		for i := range 5 {
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				newObj := obj.DeepCopy()
				newObj.ResourceVersion = "1" // include an invalid RV to cause a conflict
				newObj.Data = map[string]string{"foo": strconv.Itoa(i)}
				Expect(apierrors.IsConflict(cl.Patch(ctx, newObj, client.MergeFrom(obj)))).To(BeTrue())
			}()
		}
		wg.Wait()
	})

	It("should allow concurrent updates to an object that allows unconditionalUpdate if the incoming request has no RV", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				ResourceVersion: "0",
			},
		}
		cl := NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

		const tries = 50
		wg := sync.WaitGroup{}
		wg.Add(tries)

		for i := range tries {
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				newObj := obj.DeepCopy()
				newObj.Data = map[string]string{"foo": strconv.Itoa(i)}
				newObj.ResourceVersion = ""
				Expect(cl.Update(ctx, newObj)).To(Succeed())
			}()
		}
		wg.Wait()

		// While the order is not deterministic, there must be $tries distinct updates
		// that each increment the resource version by one
		Expect(cl.Get(ctx, client.ObjectKey{Name: "foo"}, obj)).To(Succeed())
		Expect(obj.ResourceVersion).To(Equal(strconv.Itoa(tries)))
	})

	It("If a create races with an update for an object that allows createOnUpdate, the update should always succeed", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		cl := NewClientBuilder().WithScheme(scheme).Build()

		const tries = 50
		wg := sync.WaitGroup{}
		wg.Add(tries * 2)

		for i := range tries {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.Itoa(i),
				},
			}
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				// this may or may not succeed depending on if we win the race. Either is acceptable,
				// but if it fails, it must fail due to an AlreadyExists.
				err := cl.Create(ctx, obj.DeepCopy())
				if err != nil {
					Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
				}
			}()

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				// This must always succeed, regardless of the outcome of the create.
				Expect(cl.Update(ctx, obj.DeepCopy())).To(Succeed())
			}()
		}

		wg.Wait()
	})

	It("If a delete races with an update for an object that allows createOnUpdate, the update should always succeed", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		cl := NewClientBuilder().WithScheme(scheme).Build()

		const tries = 50
		wg := sync.WaitGroup{}
		wg.Add(tries * 2)

		for i := range tries {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.Itoa(i),
				},
			}
			Expect(cl.Create(ctx, obj.DeepCopy())).To(Succeed())

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				Expect(cl.Delete(ctx, obj.DeepCopy())).To(Succeed())
			}()

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				// This must always succeed, regardless of if the delete came before or
				// after us.
				Expect(cl.Update(ctx, obj.DeepCopy())).To(Succeed())
			}()
		}

		wg.Wait()
	})

	It("If a DeleteAllOf races with a delete, the DeleteAllOf should always succeed", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		cl := NewClientBuilder().WithScheme(scheme).Build()

		const objects = 50
		wg := sync.WaitGroup{}
		wg.Add(objects)

		for i := range objects {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.Itoa(i),
				},
			}
			Expect(cl.Create(ctx, obj.DeepCopy())).To(Succeed())
		}

		for i := range objects {
			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.Itoa(i),
				},
			}

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				// This may or may not succeed depending on if the DeleteAllOf is faster,
				// but if it fails, it should be a not found.
				err := cl.Delete(ctx, obj)
				if err != nil {
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				}
			}()
		}
		Expect(cl.DeleteAllOf(ctx, &corev1.Service{})).To(Succeed())

		wg.Wait()
	})

	It("If an update races with a scale update, only one of them succeeds", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(appsv1.AddToScheme(scheme)).To(Succeed())

		cl := NewClientBuilder().WithScheme(scheme).Build()

		const tries = 5000
		for i := range tries {
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.Itoa(i),
				},
			}
			Expect(cl.Create(ctx, dep)).To(Succeed())

			wg := sync.WaitGroup{}
			wg.Add(2)
			var updateSucceeded bool
			var scaleSucceeded bool

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				dep := dep.DeepCopy()
				dep.Annotations = map[string]string{"foo": "bar"}

				// This may or may not fail. If it does fail, it must be a conflict.
				err := cl.Update(ctx, dep)
				if err != nil {
					Expect(apierrors.IsConflict(err)).To(BeTrue())
				} else {
					updateSucceeded = true
				}
			}()

			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				// This may or may not fail. If it does fail, it must be a conflict.
				scale := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: 10}}
				err := cl.SubResource("scale").Update(ctx, dep.DeepCopy(), client.WithSubResourceBody(scale))
				if err != nil {
					Expect(apierrors.IsConflict(err)).To(BeTrue())
				} else {
					scaleSucceeded = true
				}
			}()

			wg.Wait()
			Expect(updateSucceeded).ToNot(Equal(scaleSucceeded))
		}

	})

	It("disallows scale subresources on unsupported built-in types", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(scheme)).To(Succeed())

		obj := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		cl := NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()

		scale := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: 2}}
		expectedErr := "unimplemented scale subresource for resource *v1.Pod"
		Expect(cl.SubResource(subResourceScale).Get(ctx, obj, scale).Error()).To(Equal(expectedErr))
		Expect(cl.SubResource(subResourceScale).Update(ctx, obj, client.WithSubResourceBody(scale)).Error()).To(Equal(expectedErr))
	})

	It("disallows scale subresources on non-existing objects", func(ctx SpecContext) {
		obj := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](2),
			},
		}
		cl := NewClientBuilder().Build()

		scale := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: 2}}
		expectedErr := "deployments.apps \"foo\" not found"
		Expect(cl.SubResource(subResourceScale).Get(ctx, obj, scale).Error()).To(Equal(expectedErr))
		Expect(cl.SubResource(subResourceScale).Update(ctx, obj, client.WithSubResourceBody(scale)).Error()).To(Equal(expectedErr))
	})

	It("clears typemeta from structured objects on create", func(ctx SpecContext) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}
		cl := NewClientBuilder().Build()
		Expect(cl.Create(ctx, obj)).To(Succeed())
		Expect(obj.TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("clears typemeta from structured objects on update", func(ctx SpecContext) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}
		cl := NewClientBuilder().WithObjects(obj).Build()
		Expect(cl.Update(ctx, obj)).To(Succeed())
		Expect(obj.TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("clears typemeta from structured objects on patch", func(ctx SpecContext) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		cl := NewClientBuilder().WithObjects(obj).Build()
		original := obj.DeepCopy()
		obj.TypeMeta = metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		}
		Expect(cl.Patch(ctx, obj, client.MergeFrom(original))).To(Succeed())
		Expect(obj.TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("clears typemeta from structured objects on get", func(ctx SpecContext) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}
		cl := NewClientBuilder().WithObjects(obj).Build()
		target := &corev1.ConfigMap{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), target)).To(Succeed())
		Expect(target.TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("clears typemeta from structured objects on list", func(ctx SpecContext) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}
		cl := NewClientBuilder().WithObjects(obj).Build()
		target := &corev1.ConfigMapList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		}
		Expect(cl.List(ctx, target)).To(Succeed())
		Expect(target.TypeMeta).To(Equal(metav1.TypeMeta{}))
		Expect(target.Items[0].TypeMeta).To(Equal(metav1.TypeMeta{}))
	})

	It("is threadsafe", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()

		u := func() *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetAPIVersion("custom/v1")
			u.SetKind("Version")
			u.SetName("foo")
			return u
		}

		uList := func() *unstructured.UnstructuredList {
			u := &unstructured.UnstructuredList{}
			u.SetAPIVersion("custom/v1")
			u.SetKind("Version")

			return u
		}

		meta := func() *metav1.PartialObjectMetadata {
			return &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "custom/v1",
					Kind:       "Version",
				},
			}
		}
		metaList := func() *metav1.PartialObjectMetadataList {
			return &metav1.PartialObjectMetadataList{
				TypeMeta: metav1.TypeMeta{

					APIVersion: "custom/v1",
					Kind:       "Version",
				},
			}
		}

		pod := func() *corev1.Pod {
			return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			}}
		}

		ops := []func(){
			func() { _ = cl.Create(ctx, u()) },
			func() { _ = cl.Get(ctx, client.ObjectKeyFromObject(u()), u()) },
			func() { _ = cl.Update(ctx, u()) },
			func() { _ = cl.Patch(ctx, u(), client.RawPatch(types.StrategicMergePatchType, []byte("foo"))) },
			func() { _ = cl.Delete(ctx, u()) },
			func() { _ = cl.DeleteAllOf(ctx, u(), client.HasLabels{"foo"}) },
			func() { _ = cl.List(ctx, uList()) },

			func() { _ = cl.Create(ctx, meta()) },
			func() { _ = cl.Get(ctx, client.ObjectKeyFromObject(meta()), meta()) },
			func() { _ = cl.Update(ctx, meta()) },
			func() { _ = cl.Patch(ctx, meta(), client.RawPatch(types.StrategicMergePatchType, []byte("foo"))) },
			func() { _ = cl.Delete(ctx, meta()) },
			func() { _ = cl.DeleteAllOf(ctx, meta(), client.HasLabels{"foo"}) },
			func() { _ = cl.List(ctx, metaList()) },

			func() { _ = cl.Create(ctx, pod()) },
			func() { _ = cl.Get(ctx, client.ObjectKeyFromObject(pod()), pod()) },
			func() { _ = cl.Update(ctx, pod()) },
			func() { _ = cl.Patch(ctx, pod(), client.RawPatch(types.StrategicMergePatchType, []byte("foo"))) },
			func() { _ = cl.Delete(ctx, pod()) },
			func() { _ = cl.DeleteAllOf(ctx, pod(), client.HasLabels{"foo"}) },
			func() { _ = cl.List(ctx, &corev1.PodList{}) },
		}

		wg := sync.WaitGroup{}
		wg.Add(len(ops))
		for _, op := range ops {
			go func() {
				defer wg.Done()
				op()
			}()
		}

		wg.Wait()
	})

	DescribeTable("mutating operations return the updated object",
		func(ctx SpecContext, mutate func(ctx SpecContext) (*corev1.ConfigMap, error)) {
			mutated, err := mutate(ctx)
			Expect(err).NotTo(HaveOccurred())

			var retrieved corev1.ConfigMap
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(mutated), &retrieved)).To(Succeed())

			Expect(&retrieved).To(BeComparableTo(mutated))
		},

		Entry("create", func(ctx SpecContext) (*corev1.ConfigMap, error) {
			cl = NewClientBuilder().Build()
			cm.ResourceVersion = ""
			return cm, cl.Create(ctx, cm)
		}),
		Entry("update", func(ctx SpecContext) (*corev1.ConfigMap, error) {
			cl = NewClientBuilder().WithObjects(cm).Build()
			cm.Labels = map[string]string{"updated-label": "update-test"}
			cm.Data["new-key"] = "new-value"
			return cm, cl.Update(ctx, cm)
		}),
		Entry("patch", func(ctx SpecContext) (*corev1.ConfigMap, error) {
			cl = NewClientBuilder().WithObjects(cm).Build()
			original := cm.DeepCopy()

			cm.Labels = map[string]string{"updated-label": "update-test"}
			cm.Data["new-key"] = "new-value"
			return cm, cl.Patch(ctx, cm, client.MergeFrom(original))
		}),
		Entry("Create through Apply", func(ctx SpecContext) (*corev1.ConfigMap, error) {
			ac := corev1applyconfigurations.ConfigMap(cm.Name, cm.Namespace).WithData(cm.Data)

			cl = NewClientBuilder().Build()
			Expect(cl.Apply(ctx, ac, client.FieldOwner("foo"))).To(Succeed())

			serialized, err := json.Marshal(ac)
			Expect(err).NotTo(HaveOccurred())

			var cm corev1.ConfigMap
			Expect(json.Unmarshal(serialized, &cm)).To(Succeed())

			// ApplyConfigurations always have TypeMeta set as they do not support using the scheme
			// to retrieve gvk.
			cm.TypeMeta = metav1.TypeMeta{}
			return &cm, nil
		}),
		Entry("Update through Apply", func(ctx SpecContext) (*corev1.ConfigMap, error) {
			ac := corev1applyconfigurations.ConfigMap(cm.Name, cm.Namespace).
				WithLabels(map[string]string{"updated-label": "update-test"}).
				WithData(map[string]string{"new-key": "new-value"})

			cl = NewClientBuilder().WithObjects(cm).Build()
			Expect(cl.Apply(ctx, ac, client.FieldOwner("foo"))).To(Succeed())

			serialized, err := json.Marshal(ac)
			Expect(err).NotTo(HaveOccurred())

			var cm corev1.ConfigMap
			Expect(json.Unmarshal(serialized, &cm)).To(Succeed())

			// ApplyConfigurations always have TypeMeta set as they do not support using the scheme
			// to retrieve gvk.
			cm.TypeMeta = metav1.TypeMeta{}
			return &cm, nil
		}),
	)

	It("supports server-side apply of a client-go resource", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("ConfigMap")
		obj.SetName("foo")
		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"some": "data"}, "data")).To(Succeed())

		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(Equal(map[string]string{"some": "data"}))

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"other": "data"}, "data")).To(Succeed())
		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(Equal(map[string]string{"other": "data"}))
	})

	It("supports server-side apply of a custom resource", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("custom/v1")
		obj.SetKind("FakeResource")
		obj.SetName("foo")
		result := obj.DeepCopy()

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"some": "data"}, "spec")).To(Succeed())

		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(result), result)).To(Succeed())
		Expect(result.Object["spec"]).To(Equal(map[string]any{"some": "data"}))

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"other": "data"}, "spec")).To(Succeed())
		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(result), result)).To(Succeed())
		Expect(result.Object["spec"]).To(Equal(map[string]any{"other": "data"}))
	})

	It("errors out when doing SSA with managedFields set", func(ctx SpecContext) {
		cl := NewClientBuilder().WithReturnManagedFields().Build()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("ConfigMap")
		obj.SetName("foo")
		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"some": "data"}, "data")).To(Succeed())

		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		err := cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("metadata.managedFields must be nil"))
	})

	It("supports server-side apply using a custom type converter", func(ctx SpecContext) {
		cl := NewClientBuilder().
			WithTypeConverters(clientgoapplyconfigurations.NewTypeConverter(clientgoscheme.Scheme)).
			Build()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("ConfigMap")
		obj.SetName("foo")

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"some": "data"}, "data")).To(Succeed())

		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(Equal(map[string]string{"some": "data"}))

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"other": "data"}, "data")).To(Succeed())
		Expect(cl.Patch(ctx, obj, client.Apply, client.FieldOwner("foo"))).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(Equal(map[string]string{"other": "data"}))
	})

	It("returns managedFields if configured to do so", func(ctx SpecContext) {
		cl := NewClientBuilder().WithReturnManagedFields().Build()
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"initial": "data",
			},
		}
		Expect(cl.Create(ctx, cm)).NotTo(HaveOccurred())
		Expect(cm.ManagedFields).NotTo(BeNil())

		retrieved := &corev1.ConfigMap{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), retrieved)).NotTo(HaveOccurred())
		Expect(retrieved.ManagedFields).NotTo(BeNil())

		cm.Data["another"] = "value"
		cm.SetManagedFields(nil)
		Expect(cl.Update(ctx, cm)).NotTo(HaveOccurred())
		Expect(cm.ManagedFields).NotTo(BeNil())

		cm.SetManagedFields(nil)
		beforePatch := cm.DeepCopy()
		cm.Data["a-third"] = "value"
		Expect(cl.Patch(ctx, cm, client.MergeFrom(beforePatch))).NotTo(HaveOccurred())
		Expect(cm.ManagedFields).NotTo(BeNil())

		u := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      cm.Name,
				"namespace": cm.Namespace,
			},
			"data": map[string]any{
				"ssa": "value",
			},
		}}
		Expect(cl.Patch(ctx, u, client.Apply, client.FieldOwner("foo"))).NotTo(HaveOccurred())
		_, exists, err := unstructured.NestedFieldNoCopy(u.Object, "metadata", "managedFields")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())

		var list corev1.ConfigMapList
		Expect(cl.List(ctx, &list)).NotTo(HaveOccurred())
		for _, item := range list.Items {
			Expect(item.ManagedFields).NotTo(BeNil())
		}
	})

	It("clears managedFields from objects in a list", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithData(map[string]string{"some": "data"})

		Expect(cl.Apply(ctx, obj, &client.ApplyOptions{FieldManager: "test-manager"})).To(Succeed())

		var list corev1.ConfigMapList
		Expect(cl.List(ctx, &list)).NotTo(HaveOccurred())
		for _, item := range list.Items {
			Expect(item.ManagedFields).To(BeNil())
		}
	})

	It("supports server-side apply of a client-go resource via Apply method", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithData(map[string]string{"some": "data"})

		Expect(cl.Apply(ctx, obj, &client.ApplyOptions{FieldManager: "test-manager"})).To(Succeed())

		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(BeComparableTo(map[string]string{"some": "data"}))

		obj.Data = map[string]string{"other": "data"}
		Expect(cl.Apply(ctx, obj, &client.ApplyOptions{FieldManager: "test-manager"})).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data).To(BeComparableTo(map[string]string{"other": "data"}))
	})

	It("errors when trying to server-side apply an object without configuring a FieldManager", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithData(map[string]string{"some": "data"})

		err := cl.Apply(ctx, obj)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "Expected error to be an invalid error")
	})

	It("errors when trying to server-side apply an object with an invalid FieldManager", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithData(map[string]string{"some": "data"})

		err := cl.Apply(ctx, obj, client.FieldOwner("\x00"))
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "Expected error to be an invalid error")
	})

	It("supports server-side apply of a custom resource via Apply method", func(ctx SpecContext) {
		cl := NewClientBuilder().Build()
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("custom/v1")
		obj.SetKind("FakeResource")
		obj.SetName("foo")
		result := obj.DeepCopy()

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"some": "data"}, "spec")).To(Succeed())

		applyConfig := client.ApplyConfigurationFromUnstructured(obj)
		Expect(cl.Apply(ctx, applyConfig, &client.ApplyOptions{FieldManager: "test-manager"})).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(result), result)).To(Succeed())
		Expect(result.Object["spec"]).To(Equal(map[string]any{"some": "data"}))

		Expect(unstructured.SetNestedField(obj.Object, map[string]any{"other": "data"}, "spec")).To(Succeed())
		applyConfig2 := client.ApplyConfigurationFromUnstructured(obj)
		Expect(cl.Apply(ctx, applyConfig2, &client.ApplyOptions{FieldManager: "test-manager"})).To(Succeed())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(result), result)).To(Succeed())
		Expect(result.Object["spec"]).To(Equal(map[string]any{"other": "data"}))
	})

	It("sets managed fields through all methods", func(ctx SpecContext) {
		owner := "test-owner"
		cl := client.WithFieldOwner(
			NewClientBuilder().WithReturnManagedFields().Build(),
			owner,
		)

		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "foo"},
			Data:       map[string]string{"method": "create"},
		}
		Expect(cl.Create(ctx, obj)).NotTo(HaveOccurred())

		Expect(obj.ManagedFields).NotTo(BeEmpty())
		for _, f := range obj.ManagedFields {
			Expect(f.Manager).To(BeEquivalentTo(owner))
		}

		originalObj := obj.DeepCopy()
		obj.Data["method"] = "patch"
		Expect(cl.Patch(ctx, obj, client.MergeFrom(originalObj))).NotTo(HaveOccurred())
		Expect(obj.ManagedFields).NotTo(BeEmpty())
		for _, f := range obj.ManagedFields {
			Expect(f.Manager).To(BeEquivalentTo(owner))
		}

		obj.Data["method"] = "update"
		Expect(cl.Update(ctx, obj)).NotTo(HaveOccurred())
		Expect(obj.ManagedFields).NotTo(BeEmpty())
		for _, f := range obj.ManagedFields {
			Expect(f.Manager).To(BeEquivalentTo(owner))
		}
	})

	// GH-3267
	It("Doesn't leave stale data when updating an object through SSA", func(ctx SpecContext) {
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithData(map[string]string{"some": "data"})

		cl := NewClientBuilder().Build()
		Expect(cl.Apply(ctx, obj, client.FieldOwner("foo"))).NotTo(HaveOccurred())

		obj.WithData(map[string]string{"bar": "baz"})
		Expect(cl.Apply(ctx, obj, client.FieldOwner("foo"))).NotTo(HaveOccurred())
		var cms corev1.ConfigMapList
		Expect(cl.List(ctx, &cms)).NotTo(HaveOccurred())
		Expect(len(cms.Items)).To(BeEquivalentTo(1))
	})

	It("allows to set deletionTimestamp on an object during SSA create", func(ctx SpecContext) {
		now := metav1.Time{Time: time.Now().Round(time.Second)}
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithDeletionTimestamp(now).
			WithData(map[string]string{"some": "data"})

		cl := NewClientBuilder().Build()
		Expect(cl.Apply(ctx, obj, client.FieldOwner("foo"))).NotTo(HaveOccurred())

		Expect(obj.DeletionTimestamp).To(BeEquivalentTo(&now))
	})

	It("will silently ignore a deletionTimestamp update through SSA", func(ctx SpecContext) {
		now := metav1.Time{Time: time.Now().Round(time.Second)}
		obj := corev1applyconfigurations.
			ConfigMap("foo", "default").
			WithDeletionTimestamp(now).
			WithFinalizers("foo.bar").
			WithData(map[string]string{"some": "data"})

		cl := NewClientBuilder().Build()
		Expect(cl.Apply(ctx, obj, client.FieldOwner("foo"))).NotTo(HaveOccurred())
		Expect(obj.DeletionTimestamp).To(BeEquivalentTo(&now))

		later := metav1.Time{Time: now.Add(time.Second)}
		obj.DeletionTimestamp = &later
		Expect(cl.Apply(ctx, obj, client.FieldOwner("foo"))).NotTo(HaveOccurred())
		Expect(*obj.DeletionTimestamp).To(BeEquivalentTo(now))
	})

	It("will error out if an object with invalid managedFields is added", func(ctx SpecContext) {
		fieldV1Map := map[string]interface{}{
			"f:metadata": map[string]interface{}{
				"f:name":        map[string]interface{}{},
				"f:labels":      map[string]interface{}{},
				"f:annotations": map[string]interface{}{},
				"f:finalizers":  map[string]interface{}{},
			},
		}
		fieldV1, err := json.Marshal(fieldV1Map)
		Expect(err).NotTo(HaveOccurred())

		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-1",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "my-manager",
				Operation:  metav1.ManagedFieldsOperationUpdate,
				FieldsType: "FieldsV1",
				FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
			}},
		}}

		Expect(func() {
			NewClientBuilder().WithObjects(obj).Build()
		}).To(PanicWith(MatchError(ContainSubstring("invalid managedFields"))))
	})

	It("allows adding an object with managedFields", func(ctx SpecContext) {
		fieldV1Map := map[string]interface{}{
			"f:metadata": map[string]interface{}{
				"f:name":        map[string]interface{}{},
				"f:labels":      map[string]interface{}{},
				"f:annotations": map[string]interface{}{},
				"f:finalizers":  map[string]interface{}{},
			},
		}
		fieldV1, err := json.Marshal(fieldV1Map)
		Expect(err).NotTo(HaveOccurred())

		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-1",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "my-manager",
				Operation:  metav1.ManagedFieldsOperationUpdate,
				FieldsType: "FieldsV1",
				FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
				APIVersion: "v1",
			}},
		}}

		NewClientBuilder().WithObjects(obj).Build()
	})

	It("allows adding an object with invalid managedFields when not using the FieldManagedObjectTracker", func(ctx SpecContext) {
		fieldV1Map := map[string]interface{}{
			"f:metadata": map[string]interface{}{
				"f:name":        map[string]interface{}{},
				"f:labels":      map[string]interface{}{},
				"f:annotations": map[string]interface{}{},
				"f:finalizers":  map[string]interface{}{},
			},
		}
		fieldV1, err := json.Marshal(fieldV1Map)
		Expect(err).NotTo(HaveOccurred())

		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-1",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "my-manager",
				Operation:  metav1.ManagedFieldsOperationUpdate,
				FieldsType: "FieldsV1",
				FieldsV1:   &metav1.FieldsV1{Raw: fieldV1},
			}},
		}}

		NewClientBuilder().
			WithObjectTracker(testing.NewObjectTracker(
				clientgoscheme.Scheme,
				serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDecoder(),
			)).
			WithObjects(obj).
			Build()
	})

	scalableObjs := []client.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](2),
			},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: appsv1.ReplicaSetSpec{
				Replicas: ptr.To[int32](2),
			},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: corev1.ReplicationControllerSpec{
				Replicas: ptr.To[int32](2),
			},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: ptr.To[int32](2),
			},
		},
	}
	for _, obj := range scalableObjs {
		It(fmt.Sprintf("should be able to Get scale subresources for resource %T", obj), func(ctx SpecContext) {
			cl := NewClientBuilder().WithObjects(obj).Build()

			scaleActual := &autoscalingv1.Scale{}
			Expect(cl.SubResource(subResourceScale).Get(ctx, obj, scaleActual)).NotTo(HaveOccurred())

			scaleExpected := &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{
					Name:            obj.GetName(),
					UID:             obj.GetUID(),
					ResourceVersion: obj.GetResourceVersion(),
				},
				Spec: autoscalingv1.ScaleSpec{
					Replicas: 2,
				},
			}
			Expect(cmp.Diff(scaleExpected, scaleActual)).To(BeEmpty())
		})

		It(fmt.Sprintf("should be able to Update scale subresources for resource %T", obj), func(ctx SpecContext) {
			cl := NewClientBuilder().WithObjects(obj).Build()

			scaleExpected := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: 3}}
			Expect(cl.SubResource(subResourceScale).Update(ctx, obj, client.WithSubResourceBody(scaleExpected))).NotTo(HaveOccurred())

			objActual := obj.DeepCopyObject().(client.Object)
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(objActual), objActual)).To(Succeed())

			objExpected := obj.DeepCopyObject().(client.Object)
			switch expected := objExpected.(type) {
			case *appsv1.Deployment:
				expected.ResourceVersion = objActual.GetResourceVersion()
				expected.Spec.Replicas = ptr.To(int32(3))
			case *appsv1.ReplicaSet:
				expected.ResourceVersion = objActual.GetResourceVersion()
				expected.Spec.Replicas = ptr.To(int32(3))
			case *corev1.ReplicationController:
				expected.ResourceVersion = objActual.GetResourceVersion()
				expected.Spec.Replicas = ptr.To(int32(3))
			case *appsv1.StatefulSet:
				expected.ResourceVersion = objActual.GetResourceVersion()
				expected.Spec.Replicas = ptr.To(int32(3))
			}
			Expect(cmp.Diff(objExpected, objActual)).To(BeEmpty())

			scaleActual := &autoscalingv1.Scale{}
			Expect(cl.SubResource(subResourceScale).Get(ctx, obj, scaleActual)).NotTo(HaveOccurred())

			// When we called Update, these were derived but we need them now to compare.
			scaleExpected.Name = scaleActual.Name
			scaleExpected.ResourceVersion = scaleActual.ResourceVersion
			Expect(cmp.Diff(scaleExpected, scaleActual)).To(BeEmpty())
		})

	}
})

type Schemaless map[string]interface{}

type WithSchemalessSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Schemaless `json:"spec,omitempty"`
}

func (t *WithSchemalessSpec) DeepCopy() *WithSchemalessSpec {
	w := &WithSchemalessSpec{
		ObjectMeta: *t.ObjectMeta.DeepCopy(),
	}
	w.TypeMeta = metav1.TypeMeta{
		APIVersion: t.APIVersion,
		Kind:       t.Kind,
	}
	t.Spec.DeepCopyInto(&w.Spec)

	return w
}

func (t *WithSchemalessSpec) DeepCopyObject() runtime.Object {
	return t.DeepCopy()
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Schemaless) DeepCopyInto(out *Schemaless) {
	if *in != nil {
		*out = make(Schemaless, len(*in))
		for key := range *in {
			(*out)[key] = (*in)[key]
		}
	}
}

// DeepCopy copies the receiver, creating a new Schemaless.
func (in *Schemaless) DeepCopy() *Schemaless {
	if in == nil {
		return nil
	}
	out := new(Schemaless)
	in.DeepCopyInto(out)
	return out
}

var _ = Describe("Fake client builder", func() {
	It("panics when an index with the same name and GroupVersionKind is registered twice", func(ctx SpecContext) {
		// We need any realistic GroupVersionKind, the choice of apps/v1 Deployment is arbitrary.
		cb := NewClientBuilder().WithIndex(&appsv1.Deployment{},
			"test-name",
			func(client.Object) []string { return nil })

		Expect(func() {
			cb.WithIndex(&appsv1.Deployment{},
				"test-name",
				func(client.Object) []string { return []string{"foo"} })
		}).To(Panic())
	})

	It("should wrap the fake client with an interceptor when WithInterceptorFuncs is called", func(ctx SpecContext) {
		var called bool
		cli := NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				called = true
				return nil
			},
		}).Build()
		err := cli.Get(ctx, client.ObjectKey{}, &corev1.Pod{})
		Expect(err).NotTo(HaveOccurred())
		Expect(called).To(BeTrue())
	})

	It("should panic when calling build more than once", func() {
		cb := NewClientBuilder()
		anotherCb := cb
		cb.Build()
		Expect(func() {
			anotherCb.Build()
		}).To(Panic())
	})
})
