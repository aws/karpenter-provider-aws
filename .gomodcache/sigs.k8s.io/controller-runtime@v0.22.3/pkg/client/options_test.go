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

package client_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ListOptions", func() {
	It("Should set LabelSelector", func() {
		labelSelector, err := labels.Parse("a=b")
		Expect(err).NotTo(HaveOccurred())
		o := &client.ListOptions{LabelSelector: labelSelector}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set LabelSelector with MatchingLabelsSelector", func() {
		labelSelector, err := labels.Parse("a=b")
		Expect(err).NotTo(HaveOccurred())
		newListOpts := &client.ListOptions{}
		newListOpts.ApplyOptions([]client.ListOption{client.MatchingLabelsSelector{Selector: labelSelector}})
		expectedListOpts := &client.ListOptions{LabelSelector: client.MatchingLabelsSelector{Selector: labelSelector}}
		Expect(newListOpts).To(Equal(expectedListOpts))
	})
	It("Should set LabelSelector to nothing with empty MatchingLabelsSelector", func() {
		newListOpts := &client.ListOptions{}
		newListOpts.ApplyOptions([]client.ListOption{client.MatchingLabelsSelector{}})
		expectedListOpts := &client.ListOptions{LabelSelector: client.MatchingLabelsSelector{Selector: labels.Nothing()}}
		Expect(newListOpts).To(Equal(expectedListOpts))
	})
	It("Should set FieldSelector", func() {
		o := &client.ListOptions{FieldSelector: fields.Nothing()}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set FieldSelector with MatchingFieldsSelector", func() {
		newListOpts := &client.ListOptions{}
		newListOpts.ApplyOptions([]client.ListOption{client.MatchingFieldsSelector{Selector: fields.Nothing()}})
		expectedListOpts := &client.ListOptions{FieldSelector: client.MatchingFieldsSelector{Selector: fields.Nothing()}}
		Expect(newListOpts).To(Equal(expectedListOpts))
	})
	It("Should set FieldSelector to nothing with empty MatchingFieldsSelector", func() {
		newListOpts := &client.ListOptions{}
		newListOpts.ApplyOptions([]client.ListOption{client.MatchingFieldsSelector{}})
		expectedListOpts := &client.ListOptions{FieldSelector: client.MatchingFieldsSelector{Selector: fields.Nothing()}}
		Expect(newListOpts).To(Equal(expectedListOpts))
	})
	It("Should set Namespace", func() {
		o := &client.ListOptions{Namespace: "my-ns"}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set Raw", func() {
		o := &client.ListOptions{Raw: &metav1.ListOptions{FieldSelector: "Hans"}}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set Limit", func() {
		o := &client.ListOptions{Limit: int64(1)}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set Continue", func() {
		o := &client.ListOptions{Continue: "foo"}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set UnsafeDisableDeepCopy", func() {
		definitelyTrue := true
		o := &client.ListOptions{UnsafeDisableDeepCopy: &definitelyTrue}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
	It("Should set UnsafeDisableDeepCopy through option", func() {
		listOpts := &client.ListOptions{}
		client.UnsafeDisableDeepCopy.ApplyToList(listOpts)
		Expect(listOpts.UnsafeDisableDeepCopy).ToNot(BeNil())
		Expect(*listOpts.UnsafeDisableDeepCopy).To(BeTrue())
	})
	It("Should not set anything", func() {
		o := &client.ListOptions{}
		newListOpts := &client.ListOptions{}
		o.ApplyToList(newListOpts)
		Expect(newListOpts).To(Equal(o))
	})
})

var _ = Describe("GetOptions", func() {
	It("Should set Raw", func() {
		o := &client.GetOptions{Raw: &metav1.GetOptions{ResourceVersion: "RV0"}}
		newGetOpts := &client.GetOptions{}
		o.ApplyToGet(newGetOpts)
		Expect(newGetOpts).To(Equal(o))
	})
	It("Should set UnsafeDisableDeepCopy", func() {
		definitelyTrue := true
		o := &client.GetOptions{UnsafeDisableDeepCopy: &definitelyTrue}
		newGetOpts := &client.GetOptions{}
		o.ApplyToGet(newGetOpts)
		Expect(newGetOpts).To(Equal(o))
	})
	It("Should set UnsafeDisableDeepCopy through option", func() {
		getOpts := &client.GetOptions{}
		client.UnsafeDisableDeepCopy.ApplyToGet(getOpts)
		Expect(getOpts.UnsafeDisableDeepCopy).ToNot(BeNil())
		Expect(*getOpts.UnsafeDisableDeepCopy).To(BeTrue())
	})
})

var _ = Describe("ApplyOptions", func() {
	It("Should set DryRun", func() {
		o := &client.ApplyOptions{DryRun: []string{"Hello", "Theodore"}}
		newApplyOpts := &client.ApplyOptions{}
		o.ApplyToApply(newApplyOpts)
		Expect(newApplyOpts).To(Equal(o))
	})
	It("Should set Force", func() {
		o := &client.ApplyOptions{Force: ptr.To(true)}
		newApplyOpts := &client.ApplyOptions{}
		o.ApplyToApply(newApplyOpts)
		Expect(newApplyOpts).To(Equal(o))
	})
	It("Should set FieldManager", func() {
		o := &client.ApplyOptions{FieldManager: "field-manager"}
		newApplyOpts := &client.ApplyOptions{}
		o.ApplyToApply(newApplyOpts)
		Expect(newApplyOpts).To(Equal(o))
	})
})

var _ = Describe("CreateOptions", func() {
	It("Should set DryRun", func() {
		o := &client.CreateOptions{DryRun: []string{"Hello", "Theodore"}}
		newCreateOpts := &client.CreateOptions{}
		o.ApplyToCreate(newCreateOpts)
		Expect(newCreateOpts).To(Equal(o))
	})
	It("Should set FieldManager", func() {
		o := &client.CreateOptions{FieldManager: "FieldManager"}
		newCreateOpts := &client.CreateOptions{}
		o.ApplyToCreate(newCreateOpts)
		Expect(newCreateOpts).To(Equal(o))
	})
	It("Should set Raw", func() {
		o := &client.CreateOptions{Raw: &metav1.CreateOptions{DryRun: []string{"Bye", "Theodore"}}}
		newCreateOpts := &client.CreateOptions{}
		o.ApplyToCreate(newCreateOpts)
		Expect(newCreateOpts).To(Equal(o))
	})
	It("Should not set anything", func() {
		o := &client.CreateOptions{}
		newCreateOpts := &client.CreateOptions{}
		o.ApplyToCreate(newCreateOpts)
		Expect(newCreateOpts).To(Equal(o))
	})
})

var _ = Describe("DeleteOptions", func() {
	It("Should set GracePeriodSeconds", func() {
		o := &client.DeleteOptions{GracePeriodSeconds: ptr.To(int64(42))}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
	It("Should set Preconditions", func() {
		o := &client.DeleteOptions{Preconditions: &metav1.Preconditions{}}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
	It("Should set PropagationPolicy", func() {
		policy := metav1.DeletePropagationBackground
		o := &client.DeleteOptions{PropagationPolicy: &policy}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
	It("Should set Raw", func() {
		o := &client.DeleteOptions{Raw: &metav1.DeleteOptions{}}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
	It("Should set DryRun", func() {
		o := &client.DeleteOptions{DryRun: []string{"Hello", "Pippa"}}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
	It("Should not set anything", func() {
		o := &client.DeleteOptions{}
		newDeleteOpts := &client.DeleteOptions{}
		o.ApplyToDelete(newDeleteOpts)
		Expect(newDeleteOpts).To(Equal(o))
	})
})

var _ = Describe("UpdateOptions", func() {
	It("Should set DryRun", func() {
		o := &client.UpdateOptions{DryRun: []string{"Bye", "Pippa"}}
		newUpdateOpts := &client.UpdateOptions{}
		o.ApplyToUpdate(newUpdateOpts)
		Expect(newUpdateOpts).To(Equal(o))
	})
	It("Should set FieldManager", func() {
		o := &client.UpdateOptions{FieldManager: "Hello Boris"}
		newUpdateOpts := &client.UpdateOptions{}
		o.ApplyToUpdate(newUpdateOpts)
		Expect(newUpdateOpts).To(Equal(o))
	})
	It("Should set Raw", func() {
		o := &client.UpdateOptions{Raw: &metav1.UpdateOptions{}}
		newUpdateOpts := &client.UpdateOptions{}
		o.ApplyToUpdate(newUpdateOpts)
		Expect(newUpdateOpts).To(Equal(o))
	})
	It("Should not set anything", func() {
		o := &client.UpdateOptions{}
		newUpdateOpts := &client.UpdateOptions{}
		o.ApplyToUpdate(newUpdateOpts)
		Expect(newUpdateOpts).To(Equal(o))
	})
})

var _ = Describe("PatchOptions", func() {
	It("Should set DryRun", func() {
		o := &client.PatchOptions{DryRun: []string{"Bye", "Boris"}}
		newPatchOpts := &client.PatchOptions{}
		o.ApplyToPatch(newPatchOpts)
		Expect(newPatchOpts).To(Equal(o))
	})
	It("Should set Force", func() {
		o := &client.PatchOptions{Force: ptr.To(true)}
		newPatchOpts := &client.PatchOptions{}
		o.ApplyToPatch(newPatchOpts)
		Expect(newPatchOpts).To(Equal(o))
	})
	It("Should set FieldManager", func() {
		o := &client.PatchOptions{FieldManager: "Hello Julian"}
		newPatchOpts := &client.PatchOptions{}
		o.ApplyToPatch(newPatchOpts)
		Expect(newPatchOpts).To(Equal(o))
	})
	It("Should set Raw", func() {
		o := &client.PatchOptions{Raw: &metav1.PatchOptions{}}
		newPatchOpts := &client.PatchOptions{}
		o.ApplyToPatch(newPatchOpts)
		Expect(newPatchOpts).To(Equal(o))
	})
	It("Should not set anything", func() {
		o := &client.PatchOptions{}
		newPatchOpts := &client.PatchOptions{}
		o.ApplyToPatch(newPatchOpts)
		Expect(newPatchOpts).To(Equal(o))
	})
})

var _ = Describe("DeleteAllOfOptions", func() {
	It("Should set ListOptions", func() {
		o := &client.DeleteAllOfOptions{ListOptions: client.ListOptions{Raw: &metav1.ListOptions{}}}
		newDeleteAllOfOpts := &client.DeleteAllOfOptions{}
		o.ApplyToDeleteAllOf(newDeleteAllOfOpts)
		Expect(newDeleteAllOfOpts).To(Equal(o))
	})
	It("Should set DeleleteOptions", func() {
		o := &client.DeleteAllOfOptions{DeleteOptions: client.DeleteOptions{GracePeriodSeconds: ptr.To(int64(44))}}
		newDeleteAllOfOpts := &client.DeleteAllOfOptions{}
		o.ApplyToDeleteAllOf(newDeleteAllOfOpts)
		Expect(newDeleteAllOfOpts).To(Equal(o))
	})
})

var _ = Describe("MatchingLabels", func() {
	It("Should produce an invalid selector when given invalid input", func() {
		matchingLabels := client.MatchingLabels(map[string]string{"k": "axahm2EJ8Phiephe2eixohbee9eGeiyees1thuozi1xoh0GiuH3diewi8iem7Nui"})
		listOpts := &client.ListOptions{}
		matchingLabels.ApplyToList(listOpts)

		r, _ := listOpts.LabelSelector.Requirements()
		_, err := labels.NewRequirement(r[0].Key(), r[0].Operator(), r[0].Values().List())
		Expect(err).To(HaveOccurred())
		expectedErrMsg := `values[0][k]: Invalid value: "axahm2EJ8Phiephe2eixohbee9eGeiyees1thuozi1xoh0GiuH3diewi8iem7Nui": must be no more than 63 characters`
		Expect(err.Error()).To(Equal(expectedErrMsg))
	})

	It("Should add matchingLabels to existing selector", func() {
		listOpts := &client.ListOptions{}

		matchingLabels := client.MatchingLabels(map[string]string{"k": "v"})
		matchingLabels2 := client.MatchingLabels(map[string]string{"k2": "v2"})

		matchingLabels.ApplyToList(listOpts)
		Expect(listOpts.LabelSelector.String()).To(Equal("k=v"))

		matchingLabels2.ApplyToList(listOpts)
		Expect(listOpts.LabelSelector.String()).To(Equal("k=v,k2=v2"))
	})
})

var _ = Describe("DryRunAll", func() {
	It("Should apply to ApplyOptions", func() {
		o := &client.ApplyOptions{DryRun: []string{"server"}}
		t := client.DryRunAll
		t.ApplyToApply(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to CreateOptions", func() {
		o := &client.CreateOptions{DryRun: []string{"server"}}
		t := client.DryRunAll
		t.ApplyToCreate(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to UpdateOptions", func() {
		o := &client.UpdateOptions{DryRun: []string{"server"}}
		t := client.DryRunAll
		t.ApplyToUpdate(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to PatchOptions", func() {
		o := &client.PatchOptions{DryRun: []string{"server"}}
		t := client.DryRunAll
		t.ApplyToPatch(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to DeleteOptions", func() {
		o := &client.DeleteOptions{DryRun: []string{"server"}}
		t := client.DryRunAll
		t.ApplyToDelete(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to SubResourcePatchOptions", func() {
		o := &client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{DryRun: []string{"server"}}}
		t := client.DryRunAll
		t.ApplyToSubResourcePatch(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to SubResourceCreateOptions", func() {
		o := &client.SubResourceCreateOptions{CreateOptions: client.CreateOptions{DryRun: []string{"server"}}}
		t := client.DryRunAll
		t.ApplyToSubResourceCreate(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
	It("Should apply to SubResourceUpdateOptions", func() {
		o := &client.SubResourceUpdateOptions{UpdateOptions: client.UpdateOptions{DryRun: []string{"server"}}}
		t := client.DryRunAll
		t.ApplyToSubResourceUpdate(o)
		Expect(o.DryRun).To(Equal([]string{metav1.DryRunAll}))
	})
})

var _ = Describe("FieldOwner", func() {
	It("Should apply to PatchOptions", func() {
		o := &client.PatchOptions{FieldManager: "bar"}
		t := client.FieldOwner("foo")
		t.ApplyToPatch(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to ApplyOptions", func() {
		o := &client.ApplyOptions{FieldManager: "bar"}
		t := client.FieldOwner("foo")
		t.ApplyToApply(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to CreateOptions", func() {
		o := &client.CreateOptions{FieldManager: "bar"}
		t := client.FieldOwner("foo")
		t.ApplyToCreate(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to UpdateOptions", func() {
		o := &client.UpdateOptions{FieldManager: "bar"}
		t := client.FieldOwner("foo")
		t.ApplyToUpdate(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to SubResourcePatchOptions", func() {
		o := &client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: "bar"}}
		t := client.FieldOwner("foo")
		t.ApplyToSubResourcePatch(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to SubResourceCreateOptions", func() {
		o := &client.SubResourceCreateOptions{CreateOptions: client.CreateOptions{FieldManager: "bar"}}
		t := client.FieldOwner("foo")
		t.ApplyToSubResourceCreate(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
	It("Should apply to SubResourceUpdateOptions", func() {
		o := &client.SubResourceUpdateOptions{UpdateOptions: client.UpdateOptions{FieldManager: "bar"}}
		t := client.FieldOwner("foo")
		t.ApplyToSubResourceUpdate(o)
		Expect(o.FieldManager).To(Equal("foo"))
	})
})

var _ = Describe("ForceOwnership", func() {
	It("Should apply to PatchOptions", func() {
		o := &client.PatchOptions{}
		t := client.ForceOwnership
		t.ApplyToPatch(o)
		Expect(*o.Force).To(BeTrue())
	})
	It("Should apply to SubResourcePatchOptions", func() {
		o := &client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{}}
		t := client.ForceOwnership
		t.ApplyToSubResourcePatch(o)
		Expect(*o.Force).To(BeTrue())
	})
	It("Should apply to ApplyOptions", func() {
		o := &client.ApplyOptions{}
		t := client.ForceOwnership
		t.ApplyToApply(o)
		Expect(*o.Force).To(BeTrue())
	})
})

var _ = Describe("HasLabels", func() {
	It("Should produce hasLabels in given order", func() {
		listOpts := &client.ListOptions{}

		hasLabels := client.HasLabels([]string{"labelApe", "labelFox"})
		hasLabels.ApplyToList(listOpts)
		Expect(listOpts.LabelSelector.String()).To(Equal("labelApe,labelFox"))
	})

	It("Should add hasLabels to existing hasLabels selector", func() {
		listOpts := &client.ListOptions{}

		hasLabel := client.HasLabels([]string{"labelApe"})
		hasLabel.ApplyToList(listOpts)

		hasOtherLabel := client.HasLabels([]string{"labelFox"})
		hasOtherLabel.ApplyToList(listOpts)
		Expect(listOpts.LabelSelector.String()).To(Equal("labelApe,labelFox"))
	})

	It("Should add hasLabels to existing matchingLabels", func() {
		listOpts := &client.ListOptions{}

		matchingLabels := client.MatchingLabels(map[string]string{"k": "v"})
		matchingLabels.ApplyToList(listOpts)

		hasLabel := client.HasLabels([]string{"labelApe"})
		hasLabel.ApplyToList(listOpts)
		Expect(listOpts.LabelSelector.String()).To(Equal("k=v,labelApe"))
	})
})
