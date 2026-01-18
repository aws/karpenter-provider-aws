package status_test

import (
	"time"

	"github.com/awslabs/operatorpkg/status"
	"github.com/awslabs/operatorpkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Conditions", func() {
	It("should correctly toggle conditions", func() {
		testObject := test.Object(&test.CustomObject{})
		// Conditions should be initialized
		conditions := testObject.StatusConditions()
		Expect(conditions.Get(test.ConditionTypeFoo).GetStatus()).To(Equal(metav1.ConditionUnknown))
		Expect(conditions.Get(test.ConditionTypeBar).GetStatus()).To(Equal(metav1.ConditionUnknown))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionUnknown))
		// Update the condition to unknown with reason
		Expect(conditions.SetUnknownWithReason(test.ConditionTypeFoo, "reason", "message")).To(BeTrue())
		fooCondition := conditions.Get(test.ConditionTypeFoo)
		Expect(fooCondition.Type).To(Equal(test.ConditionTypeFoo))
		Expect(fooCondition.Status).To(Equal(metav1.ConditionUnknown))
		Expect(fooCondition.Reason).To(Equal("reason"))   // default to type
		Expect(fooCondition.Message).To(Equal("message")) // default to type
		Expect(fooCondition.LastTransitionTime.UnixNano()).To(BeNumerically(">", 0))
		Expect(fooCondition.ObservedGeneration).To(Equal(int64(1)))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionUnknown))
		time.Sleep(1 * time.Nanosecond)
		// Update the condition to true
		Expect(conditions.SetTrue(test.ConditionTypeFoo)).To(BeTrue())
		fooCondition = conditions.Get(test.ConditionTypeFoo)
		Expect(fooCondition.Type).To(Equal(test.ConditionTypeFoo))
		Expect(fooCondition.Status).To(Equal(metav1.ConditionTrue))
		Expect(fooCondition.Reason).To(Equal(test.ConditionTypeFoo)) // default to type
		Expect(fooCondition.Message).To(Equal(""))                   // default to type
		Expect(fooCondition.LastTransitionTime.UnixNano()).To(BeNumerically(">", 0))
		Expect(fooCondition.ObservedGeneration).To(Equal(int64(1)))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionUnknown))
		time.Sleep(1 * time.Nanosecond)
		// Update the other condition to false
		Expect(conditions.SetFalse(test.ConditionTypeBar, "reason", "message")).To(BeTrue())
		fooCondition2 := conditions.Get(test.ConditionTypeBar)
		Expect(fooCondition2.Type).To(Equal(test.ConditionTypeBar))
		Expect(fooCondition2.Status).To(Equal(metav1.ConditionFalse))
		Expect(fooCondition2.Reason).To(Equal("reason"))
		Expect(fooCondition2.Message).To(Equal("message"))
		Expect(fooCondition2.LastTransitionTime.UnixNano()).To(BeNumerically(">", 0))
		Expect(fooCondition2.ObservedGeneration).To(Equal(int64(1)))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionFalse))
		time.Sleep(1 * time.Nanosecond)
		// transition the root condition to true
		Expect(conditions.SetTrueWithReason(test.ConditionTypeBar, "reason", "message")).To(BeTrue())
		updatedFooCondition := conditions.Get(test.ConditionTypeBar)
		Expect(updatedFooCondition.Type).To(Equal(test.ConditionTypeBar))
		Expect(updatedFooCondition.Status).To(Equal(metav1.ConditionTrue))
		Expect(updatedFooCondition.Reason).To(Equal("reason"))
		Expect(updatedFooCondition.Message).To(Equal("message"))
		Expect(updatedFooCondition.LastTransitionTime.UnixNano()).To(BeNumerically(">", fooCondition.LastTransitionTime.UnixNano()))
		Expect(updatedFooCondition.ObservedGeneration).To(Equal(int64(1)))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionTrue))
		time.Sleep(1 * time.Nanosecond)
		// Transition if the status is the same, but the Reason is different
		Expect(conditions.SetFalse(test.ConditionTypeBar, "another-reason", "another-message")).To(BeTrue())
		updatedBarCondition := conditions.Get(test.ConditionTypeBar)
		Expect(updatedBarCondition.Type).To(Equal(test.ConditionTypeBar))
		Expect(updatedBarCondition.Status).To(Equal(metav1.ConditionFalse))
		Expect(updatedBarCondition.Reason).To(Equal("another-reason"))
		Expect(updatedBarCondition.LastTransitionTime.UnixNano()).ToNot(BeNumerically("==", fooCondition2.LastTransitionTime.UnixNano()))
		Expect(updatedBarCondition.ObservedGeneration).To(Equal(int64(1)))
		// Dont transition if reason and message are the same
		Expect(conditions.SetTrue(test.ConditionTypeFoo)).To(BeFalse())
		Expect(conditions.SetFalse(test.ConditionTypeBar, "another-reason", "another-message")).To(BeFalse())
		// set certain condition for first time when it is never set in object conditions
		Expect(conditions.SetTrue(test.ConditionTypeBaz)).To(BeTrue())
		updatedBazCondition := conditions.Get(test.ConditionTypeBaz)
		Expect(updatedBazCondition.LastTransitionTime.UnixNano()).To(BeNumerically(">", 0))
		Expect(updatedBazCondition.ObservedGeneration).To(Equal(int64(1)))
		testObject.Generation = 2
		Expect(conditions.SetFalse(test.ConditionTypeBar, "another-reason", "another-message")).To(BeTrue())
		updatedBarCondition2 := conditions.Get(test.ConditionTypeBar)
		Expect(updatedBarCondition2.LastTransitionTime.UnixNano()).To(BeNumerically("==", updatedBarCondition.LastTransitionTime.UnixNano()))
		Expect(updatedBarCondition2.ObservedGeneration).To(Equal(int64(2)))
		// root should be false when any dependent condition is false
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionFalse))
		Expect(conditions.Root().Reason).To(Equal("UnhealthyDependents"))
		// root should be unknown when no dependent condition is false and any observedGeneration doesn't match with latest generation
		Expect(conditions.SetTrue(test.ConditionTypeBar)).To(BeTrue())
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionUnknown))
		Expect(conditions.Root().Reason).To(Equal("ReconcilingDependents"))
		Expect(conditions.SetTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionTrue))
	})

	It("all true", func() {
		testObject := test.CustomObject{}
		Expect(testObject.StatusConditions().IsTrue()).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar, test.ConditionTypeBaz)).To(BeFalse())

		testObject.StatusConditions().SetFalse(test.ConditionTypeBaz, "reason", "message")
		Expect(testObject.StatusConditions().IsTrue()).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar, test.ConditionTypeBaz)).To(BeFalse())

		testObject.StatusConditions().SetTrue(test.ConditionTypeFoo)
		Expect(testObject.StatusConditions().IsTrue()).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar, test.ConditionTypeBaz)).To(BeFalse())

		testObject.StatusConditions().SetTrue(test.ConditionTypeBar)
		Expect(testObject.StatusConditions().IsTrue()).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBar)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBaz)).To(BeFalse())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar, test.ConditionTypeBaz)).To(BeFalse())

		testObject.StatusConditions().SetTrue(test.ConditionTypeBaz)
		Expect(testObject.StatusConditions().IsTrue()).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBar)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeBaz)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBaz)).To(BeTrue())
		Expect(testObject.StatusConditions().IsTrue(test.ConditionTypeFoo, test.ConditionTypeBar, test.ConditionTypeBaz)).To(BeTrue())
	})
	It("should sort status conditions", func() {
		testObject := test.CustomObject{}
		// Ready condition should be at the end
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-1].Type).To(Equal(status.ConditionReady))

		testObject.StatusConditions().SetTrue(test.ConditionTypeFoo)
		// Ready condition should be last with Foo condition second to last since it was recently updated
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-1].Type).To(Equal(status.ConditionReady))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-2].Type).To(Equal(test.ConditionTypeFoo))

		testObject.StatusConditions().SetTrue(test.ConditionTypeBar)
		// Ready condition should be last with Bar condition second to last since it was recently updated and Foo condition third to last
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-1].Type).To(Equal(status.ConditionReady))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-2].Type).To(Equal(test.ConditionTypeBar))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-3].Type).To(Equal(test.ConditionTypeFoo))

		testObject.StatusConditions().SetTrue(test.ConditionTypeBaz)
		// Ready condition should be last with Bar condition second to last since it was recently updated, Bar condition third to last, and Foo condition at the top
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-1].Type).To(Equal(status.ConditionReady))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-2].Type).To(Equal(test.ConditionTypeBaz))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-3].Type).To(Equal(test.ConditionTypeBar))
		Expect(testObject.StatusConditions().List()[len(testObject.StatusConditions().List())-4].Type).To(Equal(test.ConditionTypeFoo))
	})

	It("should bump generation of all conditions when deleting", func() {
		testObject := test.Object(&test.CustomObject{})
		// Conditions should be initialized
		conditions := testObject.StatusConditions()
		// Expect status to be unkown
		Expect(conditions.Get(test.ConditionTypeFoo).GetStatus()).To(Equal(metav1.ConditionUnknown))
		Expect(conditions.Get(test.ConditionTypeBar).GetStatus()).To(Equal(metav1.ConditionUnknown))
		Expect(conditions.Root().GetStatus()).To(Equal(metav1.ConditionUnknown))

		// set conditions to true and expect generation set
		Expect(conditions.SetTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(conditions.SetTrue(test.ConditionTypeBar)).To(BeTrue())
		Expect(conditions.Get(test.ConditionTypeFoo).Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions.Get(test.ConditionTypeBar).Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions.Get(test.ConditionTypeFoo).ObservedGeneration).To(Equal(int64(1)))
		Expect(conditions.Get(test.ConditionTypeBar).ObservedGeneration).To(Equal(int64(1)))

		// set deletion timestamp and bump observed generation
		testObject.SetDeletionTimestamp(lo.ToPtr(metav1.Now()))
		testObject.SetGeneration(2)

		// set one condition to true again; ensure all the other conditions observed generation is bumped
		// make sure root condition is also true and not unknown
		Expect(conditions.SetTrue(test.ConditionTypeFoo)).To(BeTrue())
		Expect(conditions.Get(test.ConditionTypeFoo).ObservedGeneration).To(Equal(int64(2)))
		Expect(conditions.Get(test.ConditionTypeBar).ObservedGeneration).To(Equal(int64(2)))
		Expect(conditions.Root().Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions.Root().ObservedGeneration).To(Equal(int64(2)))
	})
})
