package finalizer

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockFinalizer struct {
	result Result
	err    error
}

func (f mockFinalizer) Finalize(context.Context, client.Object) (Result, error) {
	return f.result, f.err
}

func TestFinalizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Finalizer Suite")
}

var _ = Describe("TestFinalizer", func() {
	var err error
	var pod *corev1.Pod
	var finalizers Finalizers
	var f mockFinalizer
	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{},
		}
		finalizers = NewFinalizers()
		f = mockFinalizer{}
	})
	Describe("Register", func() {
		It("successfully registers a finalizer", func() {
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail when trying to register a finalizer that was already registered", func() {
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			// calling Register again with the same key should return an error
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already registered"))

		})
	})

	Describe("Finalize", func() {
		It("successfully finalizes and returns true for Updated when deletion timestamp is nil and finalizer does not exist", func(ctx SpecContext) {
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			pod.DeletionTimestamp = nil
			pod.Finalizers = []string{}

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeTrue())
			// when deletion timestamp is nil and finalizer is not present, the registered finalizer would be added to the obj
			Expect(pod.Finalizers).To(HaveLen(1))
			Expect(pod.Finalizers[0]).To(Equal("finalizers.sigs.k8s.io/testfinalizer"))

		})

		It("successfully finalizes and returns true for Updated when deletion timestamp is not nil and the finalizer exists", func(ctx SpecContext) {
			now := metav1.Now()
			pod.DeletionTimestamp = &now

			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			pod.Finalizers = []string{"finalizers.sigs.k8s.io/testfinalizer"}

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeTrue())
			// finalizer will be removed from the obj upon successful finalization
			Expect(pod.Finalizers).To(BeEmpty())
		})

		It("should return no error and return false for Updated when deletion timestamp is nil and finalizer doesn't exist", func(ctx SpecContext) {
			pod.DeletionTimestamp = nil
			pod.Finalizers = []string{}

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeFalse())
			Expect(pod.Finalizers).To(BeEmpty())

		})

		It("should return no error and return false for Updated when deletion timestamp is not nil and the finalizer doesn't exist", func(ctx SpecContext) {
			now := metav1.Now()
			pod.DeletionTimestamp = &now
			pod.Finalizers = []string{}

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeFalse())
			Expect(pod.Finalizers).To(BeEmpty())

		})

		It("successfully finalizes multiple finalizers and returns true for Updated when deletion timestamp is not nil and the finalizer exists", func(ctx SpecContext) {
			now := metav1.Now()
			pod.DeletionTimestamp = &now

			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			err = finalizers.Register("finalizers.sigs.k8s.io/newtestfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			pod.Finalizers = []string{"finalizers.sigs.k8s.io/testfinalizer", "finalizers.sigs.k8s.io/newtestfinalizer"}

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeTrue())
			Expect(result.StatusUpdated).To(BeFalse())
			Expect(pod.Finalizers).To(BeEmpty())
		})

		It("should return result as false and a non-nil error", func(ctx SpecContext) {
			now := metav1.Now()
			pod.DeletionTimestamp = &now
			pod.Finalizers = []string{"finalizers.sigs.k8s.io/testfinalizer"}

			f.result.Updated = false
			f.result.StatusUpdated = false
			f.err = fmt.Errorf("finalizer failed for %q", pod.Finalizers[0])

			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer", f)
			Expect(err).ToNot(HaveOccurred())

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("finalizer failed"))
			Expect(result.Updated).To(BeFalse())
			Expect(result.StatusUpdated).To(BeFalse())
			Expect(pod.Finalizers).To(HaveLen(1))
			Expect(pod.Finalizers[0]).To(Equal("finalizers.sigs.k8s.io/testfinalizer"))
		})

		It("should return expected result values and error values when registering multiple finalizers", func(ctx SpecContext) {
			now := metav1.Now()
			pod.DeletionTimestamp = &now
			pod.Finalizers = []string{
				"finalizers.sigs.k8s.io/testfinalizer1",
				"finalizers.sigs.k8s.io/testfinalizer2",
				"finalizers.sigs.k8s.io/testfinalizer3",
			}

			// registering multiple finalizers with different return values
			// test for Updated as true, and nil error
			f.result.Updated = true
			f.result.StatusUpdated = false
			f.err = nil
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer1", f)
			Expect(err).ToNot(HaveOccurred())

			result, err := finalizers.Finalize(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Updated).To(BeTrue())
			Expect(result.StatusUpdated).To(BeFalse())
			// `finalizers.sigs.k8s.io/testfinalizer1` will be removed from the list
			// of finalizers, so length will be 2.
			Expect(pod.Finalizers).To(HaveLen(2))
			Expect(pod.Finalizers[0]).To(Equal("finalizers.sigs.k8s.io/testfinalizer2"))
			Expect(pod.Finalizers[1]).To(Equal("finalizers.sigs.k8s.io/testfinalizer3"))

			// test for Updated and StatusUpdated as false, and non-nil error
			f.result.Updated = false
			f.result.StatusUpdated = false
			f.err = fmt.Errorf("finalizer failed")
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer2", f)
			Expect(err).ToNot(HaveOccurred())

			result, err = finalizers.Finalize(ctx, pod)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("finalizer failed"))
			Expect(result.Updated).To(BeFalse())
			Expect(result.StatusUpdated).To(BeFalse())
			Expect(pod.Finalizers).To(HaveLen(2))
			Expect(pod.Finalizers[0]).To(Equal("finalizers.sigs.k8s.io/testfinalizer2"))
			Expect(pod.Finalizers[1]).To(Equal("finalizers.sigs.k8s.io/testfinalizer3"))

			// test for result as true, and non-nil error
			f.result.Updated = true
			f.result.StatusUpdated = true
			f.err = fmt.Errorf("finalizer failed")
			err = finalizers.Register("finalizers.sigs.k8s.io/testfinalizer3", f)
			Expect(err).ToNot(HaveOccurred())

			result, err = finalizers.Finalize(ctx, pod)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("finalizer failed"))
			Expect(result.Updated).To(BeTrue())
			Expect(result.StatusUpdated).To(BeTrue())
			Expect(pod.Finalizers).To(HaveLen(2))
			Expect(pod.Finalizers[0]).To(Equal("finalizers.sigs.k8s.io/testfinalizer2"))
			Expect(pod.Finalizers[1]).To(Equal("finalizers.sigs.k8s.io/testfinalizer3"))
		})
	})
})
