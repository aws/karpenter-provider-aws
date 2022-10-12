/*
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

package atomic_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter/pkg/utils/atomic"
)

func TestAtomic(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Atomic")
}

var _ = Describe("Atomic", func() {
	It("should resolve a value when set", func() {
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { return "", nil }
		str.Set("value")
		ret, err := str.TryGet(context.Background())
		Expect(err).To(Succeed())
		Expect(ret).To(Equal("value"))
	})
	It("should resolve a value and set a value when empty", func() {
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { return "value", nil }
		ret, err := str.TryGet(context.Background())
		Expect(err).To(Succeed())
		Expect(ret).To(Equal("value"))
	})
	It("should error out when the fallback function returns an err", func() {
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { return "value", fmt.Errorf("failed") }
		ret, err := str.TryGet(context.Background())
		Expect(err).ToNot(Succeed())
		Expect(ret).To(BeEmpty())
	})
	It("should ignore the cache when option set", func() {
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { return "newvalue", nil }
		str.Set("hasvalue")
		ret, err := str.TryGet(context.Background(), atomic.IgnoreCacheOption)
		Expect(err).To(Succeed())
		Expect(ret).To(Equal("newvalue"))
	})
	It("shouldn't deadlock on multiple reads", func() {
		calls := 0
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { calls++; return "value", nil }
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				ret, err := str.TryGet(context.Background())
				Expect(err).To(Succeed())
				Expect(ret).To(Equal("value"))
			}()
		}
		wg.Wait()
		Expect(calls).To(Equal(1))
	})
	It("shouldn't deadlock on multiple writes", func() {
		calls := 0
		str := atomic.CachedVal[string]{}
		str.Resolve = func(_ context.Context) (string, error) { calls++; return "value", nil }
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				ret, err := str.TryGet(context.Background(), atomic.IgnoreCacheOption)
				Expect(err).To(Succeed())
				Expect(ret).To(Equal("value"))
			}()
		}
		wg.Wait()
		Expect(calls).To(Equal(100))
	})
})
