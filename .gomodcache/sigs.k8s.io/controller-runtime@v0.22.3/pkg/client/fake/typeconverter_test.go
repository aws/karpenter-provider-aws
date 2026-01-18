/*
Copyright 2025 The Kubernetes Authors.

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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

var _ = Describe("multiTypeConverter", func() {
	Describe("ObjectToTyped", func() {
		It("should use first converter when it succeeds", func() {
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}
			testTyped := &typed.TypedValue{}

			firstConverter := &mockTypeConverter{
				objectToTypedResult: testTyped,
			}
			secondConverter := &mockTypeConverter{
				objectToTypedError: errors.New("second converter should not be called"),
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.ObjectToTyped(testObj)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(testTyped))
		})

		It("should use second converter when first fails", func() {
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}
			testTyped := &typed.TypedValue{}

			firstConverter := &mockTypeConverter{
				objectToTypedError: errors.New("first converter error"),
			}
			secondConverter := &mockTypeConverter{
				objectToTypedResult: testTyped,
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.ObjectToTyped(testObj)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(testTyped))
		})

		It("should return aggregate error when all converters fail", func() {
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}

			firstConverter := &mockTypeConverter{
				objectToTypedError: errors.New("first converter error"),
			}
			secondConverter := &mockTypeConverter{
				objectToTypedError: errors.New("second converter error"),
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.ObjectToTyped(testObj)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to convert Object to Typed"))
			Expect(err.Error()).To(ContainSubstring("first converter error"))
			Expect(err.Error()).To(ContainSubstring("second converter error"))
		})

		It("should return error when no converters provided", func() {
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{},
			}

			result, err := converter.ObjectToTyped(testObj)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to convert Object to Typed"))
		})
	})

	Describe("TypedToObject", func() {
		It("should use first converter when it succeeds", func() {
			testTyped := &typed.TypedValue{}
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}

			firstConverter := &mockTypeConverter{
				typedToObjectResult: testObj,
			}
			secondConverter := &mockTypeConverter{
				typedToObjectError: errors.New("second converter should not be called"),
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.TypedToObject(testTyped)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(testObj))
		})

		It("should use second converter when first fails", func() {
			testTyped := &typed.TypedValue{}
			testObj := &corev1.ConfigMap{Data: map[string]string{"key": "value"}}

			firstConverter := &mockTypeConverter{
				typedToObjectError: errors.New("first converter error"),
			}
			secondConverter := &mockTypeConverter{
				typedToObjectResult: testObj,
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.TypedToObject(testTyped)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(testObj))
		})

		It("should return aggregate error when all converters fail", func() {
			testTyped := &typed.TypedValue{}

			firstConverter := &mockTypeConverter{
				typedToObjectError: errors.New("first converter error"),
			}
			secondConverter := &mockTypeConverter{
				typedToObjectError: errors.New("second converter error"),
			}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{firstConverter, secondConverter},
			}

			result, err := converter.TypedToObject(testTyped)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to convert TypedValue to Object"))
			Expect(err.Error()).To(ContainSubstring("first converter error"))
			Expect(err.Error()).To(ContainSubstring("second converter error"))
		})

		It("should return error when no converters provided", func() {
			testTyped := &typed.TypedValue{}

			converter := multiTypeConverter{
				upstream: []managedfields.TypeConverter{},
			}

			result, err := converter.TypedToObject(testTyped)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to convert TypedValue to Object"))
		})
	})
})

type mockTypeConverter struct {
	objectToTypedResult *typed.TypedValue
	objectToTypedError  error

	typedToObjectResult runtime.Object
	typedToObjectError  error
}

func (m *mockTypeConverter) ObjectToTyped(r runtime.Object, o ...typed.ValidationOptions) (*typed.TypedValue, error) {
	return m.objectToTypedResult, m.objectToTypedError
}

func (m *mockTypeConverter) TypedToObject(v *typed.TypedValue) (runtime.Object, error) {
	return m.typedToObjectResult, m.typedToObjectError
}
