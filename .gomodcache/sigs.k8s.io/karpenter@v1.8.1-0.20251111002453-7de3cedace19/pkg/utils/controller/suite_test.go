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

package controller_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/karpenter/pkg/utils/controller"
)

func TestReconciles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerUtils")
}

var _ = Describe("ControllerUtils", func() {
	minReconciles := 10
	maxReconciles := 1000
	Context("LinearScaleReconciles Calculations", func() {
		DescribeTable("should calculate reconciles correctly based on CPU cores",
			func(cpuRequests float64, expectedReconciles int) {
				result := controller.LinearScaleReconciles(cpuRequests, minReconciles, maxReconciles)
				Expect(result).To(Equal(expectedReconciles))
			},
			// Arguments are: cpuRequests (in cores), expectedReconciles
			Entry("0.5 CPU core should return minReconciles", 0.5, 10),
			Entry("1 CPU core should return minReconciles", 1.0, 10),
			Entry("1.5 CPU cores should handle fractional cores (ceil to 2)", 1.5, 26),
			Entry("15 CPU cores should follow linear scaling formula", 15.0, 244),
			Entry("60 CPU cores should return maxReconciles", 60.0, 1000),
			Entry("100 CPU cores should return maxReconciles (clamped)", 100.0, 1000),
		)
	})
	Context("GetTypedBucketConfigs calculations", func() {
		DescribeTable("should calculate QPS and bucket size correctly",
			func(minQPS, minReconciles, concurrentReconciles, expectedQPS, expectedBucketSize int) {
				qps, bucketSize := controller.GetTypedBucketConfigs(minQPS, minReconciles, concurrentReconciles)
				Expect(qps).To(Equal(expectedQPS))
				Expect(bucketSize).To(Equal(expectedBucketSize))
			},
			// Arguments are: minQPS, minReconciles, concurrentReconciles, expectedQPS, expectedBucketSize
			Entry("scale of QPS is 100%, concurrentReconciles is equal to minimumReconciles", 10, 10, 10, 10, 100),
			Entry("scale of QPS is 100%, concurrentReconciles is double minimumReconciles", 10, 10, 20, 20, 200),
			Entry("scale of QPS is 10%, concurrentReconciles is equal to minimumReconciles", 10, 100, 100, 10, 100),
			Entry("scale of QPS is 10%, concurrentReconciles is double minimumReconciles", 10, 100, 200, 20, 200),
			Entry("scale of QPS is 25%, concurrentReconciles is 1.5x minimumReconciles", 25, 100, 150, 38, 380),
		)
	})
})
