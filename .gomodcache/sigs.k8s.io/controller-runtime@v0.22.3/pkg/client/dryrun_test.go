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
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DryRunClient", func() {
	var dep *appsv1.Deployment
	var count uint64 = 0
	var replicaCount int32 = 2
	var ns = "default"

	getClient := func() client.Client {
		cl, err := client.New(cfg, client.Options{DryRun: ptr.To(true)})
		Expect(err).NotTo(HaveOccurred())
		Expect(cl).NotTo(BeNil())
		return cl
	}

	BeforeEach(func(ctx SpecContext) {
		atomic.AddUint64(&count, 1)
		dep = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("dry-run-deployment-%v", count),
				Namespace: ns,
				Labels:    map[string]string{"name": fmt.Sprintf("dry-run-deployment-%v", count)},
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

		var err error
		dep, err = clientset.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		deleteDeployment(ctx, dep, ns)
	})

	It("should successfully Get an object", func(ctx SpecContext) {
		name := types.NamespacedName{Namespace: ns, Name: dep.Name}
		result := &appsv1.Deployment{}

		Expect(getClient().Get(ctx, name, result)).NotTo(HaveOccurred())
		Expect(result).To(BeEquivalentTo(dep))
	})

	It("should successfully List objects", func(ctx SpecContext) {
		result := &appsv1.DeploymentList{}
		opts := client.MatchingLabels(dep.Labels)

		Expect(getClient().List(ctx, result, opts)).NotTo(HaveOccurred())

		Expect(len(result.Items)).To(BeEquivalentTo(1))
		Expect(result.Items[0]).To(BeEquivalentTo(*dep))
	})

	It("should not create an object", func(ctx SpecContext) {
		newDep := dep.DeepCopy()
		newDep.Name = "new-deployment"

		Expect(getClient().Create(ctx, newDep)).ToNot(HaveOccurred())

		_, err := clientset.AppsV1().Deployments(ns).Get(ctx, newDep.Name, metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should not create an object with opts", func(ctx SpecContext) {
		newDep := dep.DeepCopy()
		newDep.Name = "new-deployment"
		opts := &client.CreateOptions{DryRun: []string{"Bye", "Pippa"}}

		Expect(getClient().Create(ctx, newDep, opts)).ToNot(HaveOccurred())

		_, err := clientset.AppsV1().Deployments(ns).Get(ctx, newDep.Name, metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should refuse a create request for an invalid object", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Spec.Template.Spec.Containers = nil

		err := getClient().Create(ctx, changedDep)
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("should not change objects via update", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		*changedDep.Spec.Replicas = 2

		Expect(getClient().Update(ctx, changedDep)).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via update with opts", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		*changedDep.Spec.Replicas = 2
		opts := &client.UpdateOptions{DryRun: []string{"Bye", "Pippa"}}

		Expect(getClient().Update(ctx, changedDep, opts)).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should refuse an update request for an invalid change", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Spec.Template.Spec.Containers = nil

		err := getClient().Update(ctx, changedDep)
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("should not change objects via patch", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		*changedDep.Spec.Replicas = 2

		Expect(getClient().Patch(ctx, changedDep, client.MergeFrom(dep))).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via patch with opts", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		*changedDep.Spec.Replicas = 2
		opts := &client.PatchOptions{DryRun: []string{"Bye", "Pippa"}}

		Expect(getClient().Patch(ctx, changedDep, client.MergeFrom(dep), opts)).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not delete objects", func(ctx SpecContext) {
		Expect(getClient().Delete(ctx, dep)).NotTo(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not delete objects with opts", func(ctx SpecContext) {
		opts := &client.DeleteOptions{DryRun: []string{"Bye", "Pippa"}}

		Expect(getClient().Delete(ctx, dep, opts)).NotTo(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not delete objects via deleteAllOf", func(ctx SpecContext) {
		opts := []client.DeleteAllOfOption{client.InNamespace(ns), client.MatchingLabels(dep.Labels)}

		Expect(getClient().DeleteAllOf(ctx, dep, opts...)).NotTo(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via update status", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Status.Replicas = 99

		Expect(getClient().Status().Update(ctx, changedDep)).NotTo(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via update status with opts", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Status.Replicas = 99
		opts := &client.SubResourceUpdateOptions{UpdateOptions: client.UpdateOptions{DryRun: []string{"Bye", "Pippa"}}}

		Expect(getClient().Status().Update(ctx, changedDep, opts)).NotTo(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via status patch", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Status.Replicas = 99

		Expect(getClient().Status().Patch(ctx, changedDep, client.MergeFrom(dep))).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})

	It("should not change objects via status patch with opts", func(ctx SpecContext) {
		changedDep := dep.DeepCopy()
		changedDep.Status.Replicas = 99

		opts := &client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{DryRun: []string{"Bye", "Pippa"}}}

		Expect(getClient().Status().Patch(ctx, changedDep, client.MergeFrom(dep), opts)).ToNot(HaveOccurred())

		actual, err := clientset.AppsV1().Deployments(ns).Get(ctx, dep.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).NotTo(BeNil())
		Expect(actual).To(BeEquivalentTo(dep))
	})
})
