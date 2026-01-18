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

package controllerutil_test

import (
	"context"
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Controllerutil", func() {
	Describe("SetOwnerReference", func() {
		It("should set ownerRef on an empty list", func() {
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:       "foo",
				Kind:       "Deployment",
				APIVersion: "extensions/v1beta1",
				UID:        "foo-uid",
			}))
		})

		It("should set the BlockOwnerDeletion if it is specified as an option", func() {
			t := true
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}

			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme, controllerutil.WithBlockOwnerDeletion(true))).ToNot(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				BlockOwnerDeletion: &t,
			}))
		})

		It("should not duplicate owner references", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "foo",
							Kind:       "Deployment",
							APIVersion: "extensions/v1beta1",
							UID:        "foo-uid",
						},
					},
				},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}

			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:       "foo",
				Kind:       "Deployment",
				APIVersion: "extensions/v1beta1",
				UID:        "foo-uid",
			}))
		})

		It("should update the reference", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "foo",
							Kind:       "Deployment",
							APIVersion: "extensions/v1alpha1",
							UID:        "foo-uid-1",
						},
					},
				},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}

			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:       "foo",
				Kind:       "Deployment",
				APIVersion: "extensions/v1beta1",
				UID:        "foo-uid-2",
			}))
		})
		It("should remove the owner reference", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "foo",
							Kind:       "Deployment",
							APIVersion: "extensions/v1alpha1",
							UID:        "foo-uid-1",
						},
					},
				},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}

			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:       "foo",
				Kind:       "Deployment",
				APIVersion: "extensions/v1beta1",
				UID:        "foo-uid-2",
			}))
			Expect(controllerutil.RemoveOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(BeEmpty())
		})
		It("should remove the owner reference established by the SetControllerReference function", func() {
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			t := true
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &t,
			}))
			Expect(controllerutil.RemoveOwnerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(BeEmpty())
		})
		It("should error when trying to remove the reference that doesn't exist", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.RemoveOwnerReference(dep, rs, scheme.Scheme)).To(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(BeEmpty())
		})
		It("should error when trying to remove the reference that doesn't abide by the scheme", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveOwnerReference(dep, rs, runtime.NewScheme())).To(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(HaveLen(1))
		})
		It("should error when trying to remove the owner when setting the owner as a non runtime.Object", func() {
			var obj metav1.Object
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveOwnerReference(obj, rs, scheme.Scheme)).To(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(HaveLen(1))
		})

		It("should error when trying to remove an owner that doesn't exist", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			dep2 := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "bar", UID: "bar-uid-3"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveOwnerReference(dep2, rs, scheme.Scheme)).To(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(HaveLen(1))
		})

		It("should return true when HasControllerReference evaluates owner reference set by SetControllerReference", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.HasControllerReference(rs)).To(BeTrue())
		})

		It("should return false when HasControllerReference evaluates owner reference set by SetOwnerReference", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.HasControllerReference(rs)).To(BeFalse())
		})

		It("should error when RemoveControllerReference owner's controller is set to false", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveControllerReference(dep, rs, scheme.Scheme)).To(HaveOccurred())
		})

		It("should error when RemoveControllerReference passed in owner is not the owner", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			dep2 := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo-2", UID: "foo-uid-42"},
			}
			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.SetOwnerReference(dep2, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveControllerReference(dep2, rs, scheme.Scheme)).To(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(HaveLen(2))
		})

		It("should not error when RemoveControllerReference owner's controller is set to true", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{},
			}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid-2"},
			}
			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(controllerutil.RemoveControllerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
			Expect(rs.GetOwnerReferences()).To(BeEmpty())
		})
	})

	Describe("SetControllerReference", func() {
		It("should set the OwnerReference if it can find the group version kind", func() {
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			t := true
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &t,
			}))
		})

		It("should return an error if it can't find the group version kind of the owner", func() {
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			}
			Expect(controllerutil.SetControllerReference(dep, rs, runtime.NewScheme())).To(HaveOccurred())
		})

		It("should return an error if the owner isn't a runtime.Object", func() {
			rs := &appsv1.ReplicaSet{}
			Expect(controllerutil.SetControllerReference(&errMetaObj{}, rs, scheme.Scheme)).To(HaveOccurred())
		})

		It("should return an error if object is already owned by another controller", func() {
			t := true
			rsOwners := []metav1.OwnerReference{
				{
					Name:               "bar",
					Kind:               "Deployment",
					APIVersion:         "extensions/v1beta1",
					UID:                "bar-uid",
					Controller:         &t,
					BlockOwnerDeletion: &t,
				},
			}
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", OwnerReferences: rsOwners}}
			dep := &extensionsv1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "foo-uid"}}

			err := controllerutil.SetControllerReference(dep, rs, scheme.Scheme)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&controllerutil.AlreadyOwnedError{}))
		})

		It("should not duplicate existing owner reference", func() {
			f := false
			t := true
			rsOwners := []metav1.OwnerReference{
				{
					Name:               "foo",
					Kind:               "Deployment",
					APIVersion:         "extensions/v1beta1",
					UID:                "foo-uid",
					Controller:         &f,
					BlockOwnerDeletion: &t,
				},
			}
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", OwnerReferences: rsOwners}}
			dep := &extensionsv1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "foo-uid"}}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &t,
			}))
		})

		It("should replace the owner reference if it's already present", func() {
			t := true
			rsOwners := []metav1.OwnerReference{
				{
					Name:               "foo",
					Kind:               "Deployment",
					APIVersion:         "extensions/v1alpha1",
					UID:                "foo-uid",
					Controller:         &t,
					BlockOwnerDeletion: &t,
				},
			}
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", OwnerReferences: rsOwners}}
			dep := &extensionsv1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "foo-uid"}}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &t,
			}))
		})

		It("should return an error if it's setting a cross-namespace owner reference", func() {
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "namespace1"}}
			dep := &extensionsv1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "namespace2", UID: "foo-uid"}}

			err := controllerutil.SetControllerReference(dep, rs, scheme.Scheme)

			Expect(err).To(HaveOccurred())
		})

		It("should return an error if it's owner is namespaced resource but dependant is cluster-scoped resource", func() {
			pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "foo-uid"}}

			err := controllerutil.SetControllerReference(pod, pv, scheme.Scheme)

			Expect(err).To(HaveOccurred())
		})

		It("should not return any error if the existing owner has a different version", func() {
			f := false
			t := true
			rsOwners := []metav1.OwnerReference{
				{
					Name:               "foo",
					Kind:               "Deployment",
					APIVersion:         "extensions/v1alpha1",
					UID:                "foo-uid",
					Controller:         &f,
					BlockOwnerDeletion: &t,
				},
			}
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", OwnerReferences: rsOwners}}
			dep := &extensionsv1beta1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "foo-uid"}}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme)).NotTo(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name: "foo",
				Kind: "Deployment",
				// APIVersion is the new owner's one
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &t,
			}))
		})

		It("should set the BlockOwnerDeletion if it is specified as an option", func() {
			f := false
			t := true
			rs := &appsv1.ReplicaSet{}
			dep := &extensionsv1beta1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
			}

			Expect(controllerutil.SetControllerReference(dep, rs, scheme.Scheme, controllerutil.WithBlockOwnerDeletion(false))).NotTo(HaveOccurred())
			Expect(rs.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				Name:               "foo",
				Kind:               "Deployment",
				APIVersion:         "extensions/v1beta1",
				UID:                "foo-uid",
				Controller:         &t,
				BlockOwnerDeletion: &f,
			}))
		})
	})

	Describe("CreateOrUpdate", func() {
		var deploy *appsv1.Deployment
		var deplSpec appsv1.DeploymentSpec
		var deplKey types.NamespacedName
		var specr controllerutil.MutateFn

		BeforeEach(func() {
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("deploy-%d", rand.Int31()),
					Namespace: "default",
				},
			}

			deplSpec = appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
							},
						},
					},
				},
			}

			deplKey = types.NamespacedName{
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
			}

			specr = deploymentSpecr(deploy, deplSpec)
		})

		It("creates a new object if one doesn't exists", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, specr)

			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultCreated")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))

			By("actually having the deployment created")
			fetched := &appsv1.Deployment{}
			Expect(c.Get(ctx, deplKey, fetched)).To(Succeed())

			By("being mutated by MutateFn")
			Expect(fetched.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(fetched.Spec.Template.Spec.Containers[0].Name).To(Equal(deplSpec.Template.Spec.Containers[0].Name))
			Expect(fetched.Spec.Template.Spec.Containers[0].Image).To(Equal(deplSpec.Template.Spec.Containers[0].Image))
		})

		It("updates existing object", func(ctx SpecContext) {
			var scale int32 = 2
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, specr)
			Expect(err).NotTo(HaveOccurred())
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))

			op, err = controllerutil.CreateOrUpdate(ctx, c, deploy, deploymentScaler(deploy, scale))
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdated")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdated))

			By("actually having the deployment scaled")
			fetched := &appsv1.Deployment{}
			Expect(c.Get(ctx, deplKey, fetched)).To(Succeed())
			Expect(*fetched.Spec.Replicas).To(Equal(scale))
		})

		It("updates only changed objects", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrUpdate(ctx, c, deploy, deploymentIdentity)
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("errors when MutateFn changes object name on creation", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, func() error {
				Expect(specr()).To(Succeed())
				return deploymentRenamer(deploy)()
			})

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("errors when MutateFn renames an object", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrUpdate(ctx, c, deploy, deploymentRenamer(deploy))

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("errors when object namespace changes", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrUpdate(ctx, c, deploy, deploymentNamespaceChanger(deploy))

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("aborts immediately if there was an error initially retrieving the object", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrUpdate(ctx, errorReader{c}, deploy, func() error {
				Fail("Mutation method should not run")
				return nil
			})

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CreateOrPatch", func() {
		var deploy *appsv1.Deployment
		var deplSpec appsv1.DeploymentSpec
		var deplKey types.NamespacedName
		var specr controllerutil.MutateFn

		BeforeEach(func() {
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("deploy-%d", rand.Int31()),
					Namespace: "default",
				},
			}

			deplSpec = appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
							},
						},
					},
				},
			}

			deplKey = types.NamespacedName{
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
			}

			specr = deploymentSpecr(deploy, deplSpec)
		})

		assertLocalDeployWasUpdated := func(ctx context.Context, fetched *appsv1.Deployment) {
			By("local deploy object was updated during patch & has same spec, status, resource version as fetched")
			if fetched == nil {
				fetched = &appsv1.Deployment{}
				ExpectWithOffset(1, c.Get(ctx, deplKey, fetched)).To(Succeed())
			}
			ExpectWithOffset(1, fetched.ResourceVersion).To(Equal(deploy.ResourceVersion))
			ExpectWithOffset(1, fetched.Spec).To(BeEquivalentTo(deploy.Spec))
			ExpectWithOffset(1, fetched.Status).To(BeEquivalentTo(deploy.Status))
		}

		assertLocalDeployStatusWasUpdated := func(ctx context.Context, fetched *appsv1.Deployment) {
			By("local deploy object was updated during patch & has same spec, status, resource version as fetched")
			if fetched == nil {
				fetched = &appsv1.Deployment{}
				ExpectWithOffset(1, c.Get(ctx, deplKey, fetched)).To(Succeed())
			}
			ExpectWithOffset(1, fetched.ResourceVersion).To(Equal(deploy.ResourceVersion))
			ExpectWithOffset(1, *fetched.Spec.Replicas).To(BeEquivalentTo(int32(5)))
			ExpectWithOffset(1, fetched.Status).To(BeEquivalentTo(deploy.Status))
			ExpectWithOffset(1, len(fetched.Status.Conditions)).To(BeEquivalentTo(1))
		}

		It("creates a new object if one doesn't exists", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultCreated")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))

			By("actually having the deployment created")
			fetched := &appsv1.Deployment{}
			Expect(c.Get(ctx, deplKey, fetched)).To(Succeed())

			By("being mutated by MutateFn")
			Expect(fetched.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(fetched.Spec.Template.Spec.Containers[0].Name).To(Equal(deplSpec.Template.Spec.Containers[0].Name))
			Expect(fetched.Spec.Template.Spec.Containers[0].Image).To(Equal(deplSpec.Template.Spec.Containers[0].Image))
		})

		It("patches existing object", func(ctx SpecContext) {
			var scale int32 = 2
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)
			Expect(err).NotTo(HaveOccurred())
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))

			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, deploymentScaler(deploy, scale))
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdated")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdated))

			By("actually having the deployment scaled")
			fetched := &appsv1.Deployment{}
			Expect(c.Get(ctx, deplKey, fetched)).To(Succeed())
			Expect(*fetched.Spec.Replicas).To(Equal(scale))
			assertLocalDeployWasUpdated(ctx, fetched)
		})

		It("patches only changed objects", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, deploymentIdentity)
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))

			assertLocalDeployWasUpdated(ctx, nil)
		})

		It("patches only changed status", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			deployStatus := appsv1.DeploymentStatus{
				ReadyReplicas: 1,
				Replicas:      3,
			}
			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, deploymentStatusr(deploy, deployStatus))
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdatedStatusOnly")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdatedStatusOnly))

			assertLocalDeployWasUpdated(ctx, nil)
		})

		It("patches resource and status", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			replicas := int32(3)
			deployStatus := appsv1.DeploymentStatus{
				ReadyReplicas: 1,
				Replicas:      replicas,
			}
			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, func() error {
				Expect(deploymentScaler(deploy, replicas)()).To(Succeed())
				return deploymentStatusr(deploy, deployStatus)()
			})
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdatedStatus")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdatedStatus))

			assertLocalDeployWasUpdated(ctx, nil)
		})

		It("patches resource and not empty status", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			replicas := int32(3)
			deployStatus := appsv1.DeploymentStatus{
				ReadyReplicas: 1,
				Replicas:      replicas,
			}
			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, func() error {
				Expect(deploymentScaler(deploy, replicas)()).To(Succeed())
				return deploymentStatusr(deploy, deployStatus)()
			})
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdatedStatus")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdatedStatus))

			assertLocalDeployWasUpdated(ctx, nil)

			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, func() error {
				deploy.Spec.Replicas = ptr.To(int32(5))
				deploy.Status.Conditions = []appsv1.DeploymentCondition{{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
				}}
				return nil
			})
			By("returning no error")
			Expect(err).NotTo(HaveOccurred())

			By("returning OperationResultUpdatedStatus")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultUpdatedStatus))

			assertLocalDeployStatusWasUpdated(ctx, nil)
		})

		It("errors when MutateFn changes object name on creation", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, func() error {
				Expect(specr()).To(Succeed())
				return deploymentRenamer(deploy)()
			})

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("errors when MutateFn renames an object", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, deploymentRenamer(deploy))

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("errors when object namespace changes", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, c, deploy, specr)

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultCreated))
			Expect(err).NotTo(HaveOccurred())

			op, err = controllerutil.CreateOrPatch(ctx, c, deploy, deploymentNamespaceChanger(deploy))

			By("returning error")
			Expect(err).To(HaveOccurred())

			By("returning OperationResultNone")
			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
		})

		It("aborts immediately if there was an error initially retrieving the object", func(ctx SpecContext) {
			op, err := controllerutil.CreateOrPatch(ctx, errorReader{c}, deploy, func() error {
				Fail("Mutation method should not run")
				return nil
			})

			Expect(op).To(BeEquivalentTo(controllerutil.OperationResultNone))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Finalizers", func() {
		var deploy *appsv1.Deployment

		Describe("AddFinalizer", func() {
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{},
				},
			}

			It("should add the finalizer when not present", func() {
				controllerutil.AddFinalizer(deploy, testFinalizer)
				Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{testFinalizer}))
			})

			It("should not add the finalizer when already present", func() {
				controllerutil.AddFinalizer(deploy, testFinalizer)
				Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{testFinalizer}))
			})
		})

		Describe("RemoveFinalizer", func() {
			It("should remove finalizer if present", func() {
				controllerutil.RemoveFinalizer(deploy, testFinalizer)
				Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{}))
			})

			It("should remove all equal finalizers if present", func() {
				deploy.SetFinalizers(append(deploy.Finalizers, testFinalizer, testFinalizer))
				controllerutil.RemoveFinalizer(deploy, testFinalizer)
				Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{}))
			})
		})

		Describe("AddFinalizer, which returns an indication of whether it modified the object's list of finalizers,", func() {
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{},
				},
			}

			When("the object's list of finalizers has no instances of the input finalizer", func() {
				It("should return true", func() {
					Expect(controllerutil.AddFinalizer(deploy, testFinalizer)).To(BeTrue())
				})
				It("should add the input finalizer to the object's list of finalizers", func() {
					Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{testFinalizer}))
				})
			})

			When("the object's list of finalizers has an instance of the input finalizer", func() {
				It("should return false", func() {
					Expect(controllerutil.AddFinalizer(deploy, testFinalizer)).To(BeFalse())
				})
				It("should not modify the object's list of finalizers", func() {
					Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{testFinalizer}))
				})
			})
		})

		Describe("RemoveFinalizer, which returns an indication of whether it modified the object's list of finalizers,", func() {
			When("the object's list of finalizers has no instances of the input finalizer", func() {
				It("should return false", func() {
					Expect(controllerutil.RemoveFinalizer(deploy, testFinalizer1)).To(BeFalse())
				})
				It("should not modify the object's list of finalizers", func() {
					Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{testFinalizer}))
				})
			})

			When("the object's list of finalizers has one instance of the input finalizer", func() {
				It("should return true", func() {
					Expect(controllerutil.RemoveFinalizer(deploy, testFinalizer)).To(BeTrue())
				})
				It("should remove the instance of the input finalizer from the object's list of finalizers", func() {
					Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{}))
				})
			})

			When("the object's list of finalizers has multiple instances of the input finalizer", func() {
				It("should return true", func() {
					deploy.SetFinalizers(append(deploy.Finalizers, testFinalizer, testFinalizer))
					Expect(controllerutil.RemoveFinalizer(deploy, testFinalizer)).To(BeTrue())
				})
				It("should remove each instance of the input finalizer from the object's list of finalizers", func() {
					Expect(deploy.ObjectMeta.GetFinalizers()).To(Equal([]string{}))
				})
			})
		})

		Describe("ContainsFinalizer", func() {
			It("should check that finalizer is present", func() {
				controllerutil.AddFinalizer(deploy, testFinalizer)
				Expect(controllerutil.ContainsFinalizer(deploy, testFinalizer)).To(BeTrue())
			})

			It("should check that finalizer is not present after RemoveFinalizer call", func() {
				controllerutil.RemoveFinalizer(deploy, testFinalizer)
				Expect(controllerutil.ContainsFinalizer(deploy, testFinalizer)).To(BeFalse())
			})
		})

		Describe("HasOwnerReference", func() {
			It("should return true if the object has the owner reference", func() {
				rs := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
				}
				dep := &extensionsv1beta1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
				}
				Expect(controllerutil.SetOwnerReference(dep, rs, scheme.Scheme)).ToNot(HaveOccurred())
				b, err := controllerutil.HasOwnerReference(rs.GetOwnerReferences(), dep, scheme.Scheme)
				Expect(err).NotTo(HaveOccurred())
				Expect(b).To(BeTrue())
			})

			It("should return false if the object does not have the owner reference", func() {
				rs := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
				}
				dep := &extensionsv1beta1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "foo-uid"},
				}
				b, err := controllerutil.HasOwnerReference(rs.GetOwnerReferences(), dep, scheme.Scheme)
				Expect(err).NotTo(HaveOccurred())
				Expect(b).To(BeFalse())
			})
		})
	})
})

const (
	testFinalizer  = "foo.bar.baz"
	testFinalizer1 = testFinalizer + "1"
)

var (
	_ runtime.Object = &errRuntimeObj{}
	_ metav1.Object  = &errMetaObj{}
)

type errRuntimeObj struct {
	runtime.TypeMeta
}

func (o *errRuntimeObj) DeepCopyObject() runtime.Object {
	return &errRuntimeObj{}
}

type errMetaObj struct {
	metav1.ObjectMeta
}

func deploymentSpecr(deploy *appsv1.Deployment, spec appsv1.DeploymentSpec) controllerutil.MutateFn {
	return func() error {
		deploy.Spec = spec
		return nil
	}
}

func deploymentStatusr(deploy *appsv1.Deployment, status appsv1.DeploymentStatus) controllerutil.MutateFn {
	return func() error {
		deploy.Status = status
		return nil
	}
}

var deploymentIdentity controllerutil.MutateFn = func() error {
	return nil
}

func deploymentRenamer(deploy *appsv1.Deployment) controllerutil.MutateFn {
	return func() error {
		deploy.Name = fmt.Sprintf("%s-1", deploy.Name)
		return nil
	}
}

func deploymentNamespaceChanger(deploy *appsv1.Deployment) controllerutil.MutateFn {
	return func() error {
		deploy.Namespace = fmt.Sprintf("%s-1", deploy.Namespace)
		return nil
	}
}

func deploymentScaler(deploy *appsv1.Deployment, replicas int32) controllerutil.MutateFn {
	fn := func() error {
		deploy.Spec.Replicas = &replicas
		return nil
	}
	return fn
}

type errorReader struct {
	client.Client
}

func (e errorReader) Get(ctx context.Context, key client.ObjectKey, into client.Object, opts ...client.GetOption) error {
	return fmt.Errorf("unexpected error")
}
