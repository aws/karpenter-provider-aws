/*
Copyright 2020 The Kubernetes Authors.

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

package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	appsv1applyconfigurations "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1applyconfigurations "k8s.io/client-go/applyconfigurations/core/v1"
	metav1applyconfigurations "k8s.io/client-go/applyconfigurations/meta/v1"
	rbacv1applyconfigurations "k8s.io/client-go/applyconfigurations/rbac/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NamespacedClient", func() {
	var dep *appsv1.Deployment
	var acDep *appsv1applyconfigurations.DeploymentApplyConfiguration
	var ns = "default"
	var count uint64 = 0
	var replicaCount int32 = 2

	getClient := func() client.Client {
		var sch = runtime.NewScheme()

		err := rbacv1.AddToScheme(sch)
		Expect(err).ToNot(HaveOccurred())
		err = appsv1.AddToScheme(sch)
		Expect(err).ToNot(HaveOccurred())

		nonNamespacedClient, err := client.New(cfg, client.Options{Scheme: sch})
		Expect(err).NotTo(HaveOccurred())
		Expect(nonNamespacedClient).NotTo(BeNil())
		return client.NewNamespacedClient(nonNamespacedClient, ns)
	}

	BeforeEach(func() {
		atomic.AddUint64(&count, 1)
		dep = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("namespaced-deployment-%v", count),
				Labels: map[string]string{"name": fmt.Sprintf("namespaced-deployment-%v", count)},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicaCount,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
				},
			},
		}
		acDep = appsv1applyconfigurations.Deployment(dep.Name, "").
			WithLabels(dep.Labels).
			WithSpec(appsv1applyconfigurations.DeploymentSpec().
				WithReplicas(*dep.Spec.Replicas).
				WithSelector(metav1applyconfigurations.LabelSelector().WithMatchLabels(dep.Spec.Selector.MatchLabels)).
				WithTemplate(corev1applyconfigurations.PodTemplateSpec().
					WithLabels(dep.Spec.Template.Labels).
					WithSpec(corev1applyconfigurations.PodSpec().
						WithContainers(corev1applyconfigurations.Container().
							WithName(dep.Spec.Template.Spec.Containers[0].Name).
							WithImage(dep.Spec.Template.Spec.Containers[0].Image),
						),
					),
				),
			)

	})

	Describe("Get", func() {
		BeforeEach(func(ctx SpecContext) {
			var err error
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully Get a namespace-scoped object", func(ctx SpecContext) {
			name := types.NamespacedName{Name: dep.Name}
			result := &appsv1.Deployment{}

			Expect(getClient().Get(ctx, name, result)).NotTo(HaveOccurred())
			Expect(result).To(BeEquivalentTo(dep))
		})

		It("should error when namespace provided in the object is different than the one "+
			"specified in client", func(ctx SpecContext) {
			name := types.NamespacedName{Name: dep.Name, Namespace: "non-default"}
			result := &appsv1.Deployment{}

			Expect(getClient().Get(ctx, name, result)).To(HaveOccurred())
		})
	})

	Describe("List", func() {
		BeforeEach(func(ctx SpecContext) {
			var err error
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully List objects when namespace is not specified with the object", func(ctx SpecContext) {
			result := &appsv1.DeploymentList{}
			opts := client.MatchingLabels(dep.Labels)

			Expect(getClient().List(ctx, result, opts)).NotTo(HaveOccurred())
			Expect(len(result.Items)).To(BeEquivalentTo(1))
			Expect(result.Items[0]).To(BeEquivalentTo(*dep))
		})

		It("should List objects from the namespace specified in the client", func(ctx SpecContext) {
			result := &appsv1.DeploymentList{}
			opts := client.InNamespace("non-default")

			Expect(getClient().List(ctx, result, opts)).NotTo(HaveOccurred())
			Expect(len(result.Items)).To(BeEquivalentTo(1))
			Expect(result.Items[0]).To(BeEquivalentTo(*dep))
		})
	})

	Describe("Apply", func() {
		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully apply an object in the right namespace", func(ctx SpecContext) {
			err := getClient().Apply(ctx, acDep, client.FieldOwner("test"))
			Expect(err).NotTo(HaveOccurred())

			res, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.GetNamespace()).To(BeEquivalentTo(ns))
		})

		It("should successfully apply an object in the right namespace through unstructured", func(ctx SpecContext) {
			serialized, err := json.Marshal(acDep)
			Expect(err).NotTo(HaveOccurred())
			u := &unstructured.Unstructured{}
			Expect(json.Unmarshal(serialized, &u.Object)).To(Succeed())
			err = getClient().Apply(ctx, client.ApplyConfigurationFromUnstructured(u), client.FieldOwner("test"))
			Expect(err).NotTo(HaveOccurred())

			res, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.GetNamespace()).To(BeEquivalentTo(ns))
		})

		It("should not create an object if the namespace of the object is different", func(ctx SpecContext) {
			acDep.WithNamespace("non-default")
			err := getClient().Apply(ctx, acDep, client.FieldOwner("test"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not match the namespace"))
		})

		It("should not create an object through unstructured if the namespace of the object is different", func(ctx SpecContext) {
			acDep.WithNamespace("non-default")
			serialized, err := json.Marshal(acDep)
			Expect(err).NotTo(HaveOccurred())
			u := &unstructured.Unstructured{}
			Expect(json.Unmarshal(serialized, &u.Object)).To(Succeed())
			err = getClient().Apply(ctx, client.ApplyConfigurationFromUnstructured(u), client.FieldOwner("test"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not match the namespace"))
		})

		It("should create a cluster scoped object", func(ctx SpecContext) {
			cr := rbacv1applyconfigurations.ClusterRole(fmt.Sprintf("clusterRole-%v", count))

			err := getClient().Apply(ctx, cr, client.FieldOwner("test"))
			Expect(err).NotTo(HaveOccurred())

			By("checking if the object was created")
			res, err := clientset.RbacV1().ClusterRoles().Get(ctx, *cr.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())

			deleteClusterRole(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: *cr.Name}})
		})
	})

	Describe("Create", func() {
		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully create object in the right namespace", func(ctx SpecContext) {
			By("creating the object initially")
			err := getClient().Create(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("checking if the object was created in the right namespace")
			res, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.GetNamespace()).To(BeEquivalentTo(ns))
		})

		It("should not create object if the namespace of the object is different", func(ctx SpecContext) {
			By("creating the object initially")
			dep.SetNamespace("non-default")
			err := getClient().Create(ctx, dep)
			Expect(err).To(HaveOccurred())
		})
		It("should create a cluster scoped object", func(ctx SpecContext) {
			cr := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("clusterRole-%v", count),
					Labels: map[string]string{"name": fmt.Sprintf("clusterRole-%v", count)},
				},
			}
			cr.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			})

			By("creating the object initially")
			err := getClient().Create(ctx, cr)
			Expect(err).NotTo(HaveOccurred())

			By("checking if the object was created")
			res, err := clientset.RbacV1().ClusterRoles().Get(ctx, cr.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())

			// Delete the clusterRole Resource
			deleteClusterRole(ctx, cr)
		})
	})

	Describe("Update", func() {
		var err error
		BeforeEach(func(ctx SpecContext) {
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			dep.Annotations = map[string]string{"foo": "bar"}
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully update the provided object", func(ctx SpecContext) {
			By("updating the Deployment")
			err = getClient().Update(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("validating if the updated Deployment has new annotation")
			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).NotTo(BeNil())
			Expect(actual.GetNamespace()).To(Equal(ns))
			Expect(actual.Annotations["foo"]).To(Equal("bar"))
		})

		It("should successfully update the provided object when namespace is not provided", func(ctx SpecContext) {
			By("updating the Deployment")
			dep.SetNamespace("")
			err = getClient().Update(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("validating if the updated Deployment has new annotation")
			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).NotTo(BeNil())
			Expect(actual.GetNamespace()).To(Equal(ns))
			Expect(actual.Annotations["foo"]).To(Equal("bar"))
		})

		It("should not update when object namespace is different", func(ctx SpecContext) {
			By("updating the Deployment")
			dep.SetNamespace("non-default")
			err = getClient().Update(ctx, dep)
			Expect(err).To(HaveOccurred())
		})

		It("should not update any object from other namespace", func(ctx SpecContext) {
			By("creating a new namespace")
			tns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "non-default-1"}}
			_, err := clientset.CoreV1().Namespaces().Create(ctx, tns, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			changedDep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "changed-dep",
					Namespace: tns.Name,
					Labels:    map[string]string{"name": "changed-dep"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
					},
				},
			}
			changedDep.Annotations = map[string]string{"foo": "bar"}

			By("creating the object initially")
			_, err = clientset.AppsV1().Deployments(tns.Name).Create(ctx, changedDep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("updating the object")
			err = getClient().Update(ctx, changedDep)
			Expect(err).To(HaveOccurred())

			deleteDeployment(ctx, changedDep, tns.Name)
			deleteNamespace(ctx, tns)
		})

		It("should update a cluster scoped resource", func(ctx SpecContext) {
			changedCR := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("clusterRole-%v", count),
					Labels: map[string]string{"name": fmt.Sprintf("clusterRole-%v", count)},
				},
			}

			changedCR.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			})

			By("Setting annotations and creating the resource")
			changedCR.Annotations = map[string]string{"foo": "bar"}
			changedCR, err = clientset.RbacV1().ClusterRoles().Create(ctx, changedCR, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("updating the deployment")
			err = getClient().Update(ctx, changedCR)

			By("validating if the cluster role was update")
			actual, err := clientset.RbacV1().ClusterRoles().Get(ctx, changedCR.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).NotTo(BeNil())
			Expect(actual.Annotations["foo"]).To(Equal("bar"))

			// delete cluster role resource
			deleteClusterRole(ctx, changedCR)
		})

	})

	Describe("Patch", func() {
		var err error
		BeforeEach(func(ctx SpecContext) {
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should successfully modify the object using Patch", func(ctx SpecContext) {
			By("Applying Patch")
			err = getClient().Patch(ctx, dep, client.RawPatch(types.MergePatchType, generatePatch()))
			Expect(err).NotTo(HaveOccurred())

			By("validating patched Deployment has new annotations")
			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Annotations["foo"]).To(Equal("bar"))
			Expect(actual.GetNamespace()).To(Equal(ns))
		})

		It("should successfully modify the object using Patch when namespace is not provided", func(ctx SpecContext) {
			By("Applying Patch")
			dep.SetNamespace("")
			err = getClient().Patch(ctx, dep, client.RawPatch(types.MergePatchType, generatePatch()))
			Expect(err).NotTo(HaveOccurred())

			By("validating patched Deployment has new annotations")
			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Annotations["foo"]).To(Equal("bar"))
			Expect(actual.GetNamespace()).To(Equal(ns))
		})

		It("should not modify the object when namespace of the object is different", func(ctx SpecContext) {
			dep.SetNamespace("non-default")
			err = getClient().Patch(ctx, dep, client.RawPatch(types.MergePatchType, generatePatch()))
			Expect(err).To(HaveOccurred())
		})

		It("should not modify an object from a different namespace", func(ctx SpecContext) {
			By("creating a new namespace")
			tns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "non-default-2"}}
			_, err := clientset.CoreV1().Namespaces().Create(ctx, tns, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			changedDep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "changed-dep",
					Namespace: tns.Name,
					Labels:    map[string]string{"name": "changed-dep"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
					},
				},
			}

			By("creating the object initially")
			changedDep, err = clientset.AppsV1().Deployments(tns.Name).Create(ctx, changedDep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = getClient().Patch(ctx, changedDep, client.RawPatch(types.MergePatchType, generatePatch()))
			Expect(err).To(HaveOccurred())

			deleteDeployment(ctx, changedDep, tns.Name)
			deleteNamespace(ctx, tns)
		})

		It("should successfully modify cluster scoped resource", func(ctx SpecContext) {
			cr := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("clusterRole-%v", count),
					Labels: map[string]string{"name": fmt.Sprintf("clusterRole-%v", count)},
				},
			}

			cr.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			})

			By("creating the resource")
			cr, err = clientset.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Applying Patch")
			err = getClient().Patch(ctx, cr, client.RawPatch(types.MergePatchType, generatePatch()))
			Expect(err).NotTo(HaveOccurred())

			By("Validating the patch")
			actual, err := clientset.RbacV1().ClusterRoles().Get(ctx, cr.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Annotations["foo"]).To(Equal("bar"))

			// delete the resource
			deleteClusterRole(ctx, cr)
		})
	})

	Describe("Delete and DeleteAllOf", func() {
		var err error
		BeforeEach(func(ctx SpecContext) {
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})
		It("should successfully delete an object when namespace is not specified", func(ctx SpecContext) {
			By("deleting the object")
			dep.SetNamespace("")
			err = getClient().Delete(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("validating the Deployment no longer exists")
			_, err = clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
		})

		It("should successfully delete all of the deployments in the given namespace", func(ctx SpecContext) {
			By("Deleting all objects in the namespace")
			err = getClient().DeleteAllOf(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("validating the Deployment no longer exists")
			_, err = clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
		})

		It("should not delete deployments in other namespaces", func(ctx SpecContext) {
			tns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "non-default-3"}}
			_, err = clientset.CoreV1().Namespaces().Create(ctx, tns, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			changedDep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "changed-dep",
					Namespace: tns.Name,
					Labels:    map[string]string{"name": "changed-dep"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
					},
				},
			}

			By("creating the object initially in other namespace")
			changedDep, err = clientset.AppsV1().Deployments(tns.Name).Create(ctx, changedDep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = getClient().DeleteAllOf(ctx, dep)
			Expect(err).NotTo(HaveOccurred())

			By("validating the Deployment exists")
			actual, err := clientset.AppsV1().Deployments(tns.Name).Get(ctx, changedDep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(BeEquivalentTo(changedDep))

			deleteDeployment(ctx, changedDep, tns.Name)
			deleteNamespace(ctx, tns)
		})
	})

	Describe("SubResourceWriter", func() {
		var err error
		BeforeEach(func(ctx SpecContext) {
			dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			deleteDeployment(ctx, dep, ns)
		})

		It("should change objects via update status", func(ctx SpecContext) {
			changedDep := dep.DeepCopy()
			changedDep.Status.Replicas = 99

			Expect(getClient().SubResource("status").Update(ctx, changedDep)).NotTo(HaveOccurred())

			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).NotTo(BeNil())
			Expect(actual.GetNamespace()).To(BeEquivalentTo(ns))
			Expect(actual.Status.Replicas).To(BeEquivalentTo(99))
		})

		It("should not change objects via update status when object namespace is different", func(ctx SpecContext) {
			changedDep := dep.DeepCopy()
			changedDep.SetNamespace("test")
			changedDep.Status.Replicas = 99

			Expect(getClient().SubResource("status").Update(ctx, changedDep)).To(HaveOccurred())
		})

		It("should change objects via status patch", func(ctx SpecContext) {
			changedDep := dep.DeepCopy()
			changedDep.Status.Replicas = 99

			Expect(getClient().SubResource("status").Patch(ctx, changedDep, client.MergeFrom(dep))).NotTo(HaveOccurred())

			actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).NotTo(BeNil())
			Expect(actual.GetNamespace()).To(BeEquivalentTo(ns))
			Expect(actual.Status.Replicas).To(BeEquivalentTo(99))
		})

		It("should not change objects via status patch when object namespace is different", func(ctx SpecContext) {
			changedDep := dep.DeepCopy()
			changedDep.Status.Replicas = 99
			changedDep.SetNamespace("test")

			Expect(getClient().SubResource("status").Patch(ctx, changedDep, client.MergeFrom(dep))).To(HaveOccurred())
		})
	})

	Describe("Test on invalid objects", func() {
		It("should refuse to perform operations on invalid object", func(ctx SpecContext) {
			err := getClient().Create(ctx, nil)
			Expect(err).To(HaveOccurred())

			err = getClient().List(ctx, nil)
			Expect(err).To(HaveOccurred())

			err = getClient().Patch(ctx, nil, client.MergeFrom(dep))
			Expect(err).To(HaveOccurred())

			err = getClient().Update(ctx, nil)
			Expect(err).To(HaveOccurred())

			err = getClient().Delete(ctx, nil)
			Expect(err).To(HaveOccurred())

			err = getClient().Status().Patch(ctx, nil, client.MergeFrom(dep))
			Expect(err).To(HaveOccurred())

			err = getClient().Status().Update(ctx, nil)
			Expect(err).To(HaveOccurred())

		})

	})
})

func generatePatch() []byte {
	mergePatch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"foo": "bar",
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return mergePatch
}

func deleteClusterRole(ctx context.Context, cr *rbacv1.ClusterRole) {
	_, err := clientset.RbacV1().ClusterRoles().Get(ctx, cr.Name, metav1.GetOptions{})
	if err == nil {
		err = clientset.RbacV1().ClusterRoles().Delete(ctx, cr.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}
