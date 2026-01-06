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

package batcher

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/expectations"

	"github.com/aws/karpenter-provider-aws/pkg/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var fakeEC2API *fake.EC2API
var ctx context.Context

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Batcher")
}

var _ = BeforeSuite(func() {
	fakeEC2API = &fake.EC2API{}
})

var _ = Describe("Batcher", func() {
	var cancelCtx context.Context
	var cancel context.CancelFunc
	var fakeBatcher *FakeBatcher

	BeforeEach(func() {
		cancelCtx, cancel = context.WithCancel(ctx)
	})
	AfterEach(func() {
		// Cancel the context to make sure that we properly clean-up
		cancel()
	})
	Context("Concurrency", func() {
		It("should limit the number of threads that run concurrently from the batcher", func() {
			// This batcher will get canceled at the end of the test run
			fakeBatcher = NewFakeBatcher(cancelCtx, time.Minute, 100)

			// Generate 300 items that add to the batcher
			for i := 0; i < 300; i++ {
				go func() {
					fakeBatcher.batcher.Add(cancelCtx, lo.ToPtr(test.RandomName()))
				}()
			}

			// Check that we get to 100 threads, and we stay at 100 threads
			Eventually(fakeBatcher.activeBatches.Load).Should(BeNumerically("==", 100))
			Consistently(fakeBatcher.activeBatches.Load, time.Second*10).Should(BeNumerically("==", 100))
		})
		It("should process 300 items in parallel to get quicker batching", func() {
			// This batcher will get canceled at the end of the test run
			fakeBatcher = NewFakeBatcher(cancelCtx, time.Second, 300)

			// Generate 300 items that add to the batcher
			for i := 0; i < 300; i++ {
				go func() {
					fakeBatcher.batcher.Add(cancelCtx, lo.ToPtr(test.RandomName()))
				}()
			}

			Eventually(fakeBatcher.activeBatches.Load).Should(BeNumerically("==", 300))
			Eventually(fakeBatcher.completedBatches.Load, time.Second*3).Should(BeNumerically("==", 300))
		})
	})
	Context("Metrics", func() {
		It("should create a batch_size metric when a batch is run", func() {
			// This batcher will get canceled at the end of the test run
			fakeBatcher = NewFakeBatcher(cancelCtx, time.Minute, 100)

			// Generate 300 items that add to the batcher
			for i := 0; i < 100; i++ {
				go func() {
					fakeBatcher.batcher.Add(cancelCtx, lo.ToPtr(test.RandomName()))
				}()
			}
			Eventually(fakeBatcher.activeBatches.Load).Should(BeNumerically("==", 100))

			metric, ok := expectations.FindMetricWithLabelValues("karpenter_cloudprovider_batcher_batch_size", map[string]string{
				"batcher": "fake",
			})
			Expect(ok).To(BeTrue())
			Expect(metric.GetHistogram().GetSampleCount()).To(BeNumerically(">=", 100))
		})
		It("should create a batch_window_duration metric when a batch is run", func() {
			// This batcher will get canceled at the end of the test run
			fakeBatcher = NewFakeBatcher(cancelCtx, time.Minute, 100)

			// Generate 300 items that add to the batcher
			for i := 0; i < 100; i++ {
				go func() {
					fakeBatcher.batcher.Add(cancelCtx, lo.ToPtr(test.RandomName()))
				}()
			}
			Eventually(fakeBatcher.activeBatches.Load).Should(BeNumerically("==", 100))

			_, ok := expectations.FindMetricWithLabelValues("karpenter_cloudprovider_batcher_batch_time_seconds", map[string]string{
				"batcher": "fake",
			})
			Expect(ok).To(BeTrue())
		})
	})
})

// FakeBatcher is a batcher with a mocked request that takes a long time to execute that also ref-counts the number
// of active requests that are running at a given time
type FakeBatcher struct {
	activeBatches    *atomic.Int64
	completedBatches *atomic.Int64
	batcher          *Batcher[string, string]
}

func NewFakeBatcher(ctx context.Context, requestLength time.Duration, maxRequestWorkers int) *FakeBatcher {
	activeBatches := &atomic.Int64{}
	completedBatches := &atomic.Int64{}
	options := Options[string, string]{
		Name:              "fake",
		IdleTimeout:       100 * time.Millisecond,
		MaxTimeout:        1 * time.Second,
		MaxRequestWorkers: maxRequestWorkers,
		RequestHasher:     DefaultHasher[string],
		BatchExecutor: func(ctx context.Context, items []*string) []Result[string] {
			// Keep a ref count of the number of batches that we are currently running
			activeBatches.Add(1)
			defer activeBatches.Add(-1)
			defer completedBatches.Add(1)

			// Wait for an arbitrary request length while running this call
			select {
			case <-ctx.Done():
			case <-time.After(requestLength):
			}

			// Return back request responses
			return lo.Map(items, func(i *string, _ int) Result[string] {
				return Result[string]{
					Output: lo.ToPtr[string](""),
					Err:    nil,
				}
			})
		},
	}
	return &FakeBatcher{
		activeBatches:    activeBatches,
		completedBatches: completedBatches,
		batcher:          NewBatcher(ctx, options),
	}
}
