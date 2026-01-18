package serrors_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/multierr"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ctx context.Context
var kubeClient client.Client

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Structured Errors")
}

var _ = BeforeSuite(func() {
	ctx = log.IntoContext(context.Background(), ginkgo.GinkgoLogr)
})

var _ = Describe("Structured Errors", func() {
	It("should parse values from a structured error", func() {
		err := serrors.Wrap(fmt.Errorf("test"), "key1", "value1", "key2", "value2", "key3", "value3", "key4", "value4")
		values := serrors.UnwrapValues(err)
		Expect(values).To(HaveLen(8))
		Expect(values).To(HaveExactElements("key1", "value1", "key2", "value2", "key3", "value3", "key4", "value4"))
		Expect(err.Error()).To(Equal("test (key1=value1, key2=value2, key3=value3, key4=value4)"))
	})
	It("should handle a multierr with the same key", func() {
		var err error

		var parts []string
		for i := range 100 {
			err = multierr.Append(err, serrors.Wrap(fmt.Errorf("test error %d", i), "key", fmt.Sprintf("value%d", i)))
			parts = append(parts, fmt.Sprintf("test error %d (key=value%d)", i, i))
		}
		values := serrors.UnwrapValues(err)
		Expect(values).To(HaveLen(2))
		Expect(values[0]).To(Equal("keys"))
		Expect(values[1]).To(HaveLen(100))

		Expect(err.Error()).To(Equal(strings.Join(parts, "; ")))
	})
	It("should handle a multierr with the same key using klog.KRef", func() {
		var err error
		err = multierr.Append(err, serrors.Wrap(fmt.Errorf("test error"), "TestObject", klog.KRef("elem", "test-object-1")))
		err = multierr.Append(err, serrors.Wrap(fmt.Errorf("test error"), "TestObject", klog.KRef("elem", "test-object-2")))
		err = multierr.Append(err, serrors.Wrap(fmt.Errorf("test error"), "TestObject", klog.KRef("elem", "test-object-3")))

		values := serrors.UnwrapValues(err)
		Expect(values).To(HaveLen(2))
		Expect(values[0]).To(Equal("TestObjects"))
		Expect(values[1]).To(HaveLen(3))

		Expect(err.Error()).To(Equal("test error (TestObject=elem/test-object-1); test error (TestObject=elem/test-object-2); test error (TestObject=elem/test-object-3)"))
	})
	It("should handle a multierr that's wrapped with another structured error", func() {
		var err error
		for i := range 100 {
			err = multierr.Append(err, serrors.Wrap(fmt.Errorf("test error %d", i), "key", fmt.Sprintf("value%d", i)))
		}
		err = serrors.Wrap(fmt.Errorf("wrapped error 1, %w", err), "wrappedKey1", "wrappedValue1")
		err = serrors.Wrap(fmt.Errorf("wrapped error 2, %w", err), "wrappedKey2", "wrappedValue2")

		values := serrors.UnwrapValues(err)
		Expect(values).To(HaveLen(6))
		Expect(values).To(ContainElements("keys", "wrappedKey1", "wrappedKey2"))
	})
})
