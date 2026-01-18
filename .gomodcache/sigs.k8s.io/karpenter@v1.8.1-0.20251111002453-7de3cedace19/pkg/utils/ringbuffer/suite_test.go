/*
Copyright The Kubernetes Authors.

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

package ringbuffer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/karpenter/pkg/utils/ringbuffer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RingBufferUtils")
}

var _ = Describe("RingBufferUtils", func() {
	It("should validate ring buffer functionality", func() {
		boolBuffer := ringbuffer.New[bool](4)
		Expect(cap(boolBuffer.Items())).To(Equal(4))
		boolBuffer.Insert(true)
		Expect(boolBuffer.Len()).To(Equal(1))
		for _, value := range boolBuffer.Items() {
			Expect(value).To(Equal(true))
		}
		boolBuffer.Reset()
		Expect(boolBuffer.Len()).To(Equal(0))
	})
	It("should expect the oldest entry in the buffer to be replaced when the buffer is full", func() {
		boolBuffer := ringbuffer.New[bool](2)
		boolBuffer.Insert(true)
		boolBuffer.Insert(true)
		Expect(boolBuffer.Len()).To(Equal(2))
		for _, value := range boolBuffer.Items() {
			Expect(value).To(Equal(true))
		}
		boolBuffer.Insert(false)
		Expect(boolBuffer.Len()).To(Equal(2))
		Expect(boolBuffer.Items()[0]).To(Equal(false))
		Expect(boolBuffer.Items()[1]).To(Equal(true))
	})
})
