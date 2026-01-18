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

package metrics_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/metrics"
)

var _ = Describe("Cloudprovider", func() {
	var nodeClaimNotFoundErr = cloudprovider.NewNodeClaimNotFoundError(errors.New("not found"))
	var insufficientCapacityErr = cloudprovider.NewInsufficientCapacityError(errors.New("not enough capacity"))
	var nodeClassNotReadyErr = cloudprovider.NewNodeClassNotReadyError(errors.New("not ready"))
	var unknownErr = errors.New("this is an error we don't know about")

	Describe("CloudProvider nodeclaim errors via GetErrorTypeLabelValue()", func() {
		Context("when the error is known", func() {
			It("nodeclaim not found should be recognized", func() {
				Expect(metrics.GetErrorTypeLabelValue(nodeClaimNotFoundErr)).To(Equal(metrics.NodeClaimNotFoundError))
			})
			It("insufficient capacity should be recognized", func() {
				Expect(metrics.GetErrorTypeLabelValue(insufficientCapacityErr)).To(Equal(metrics.InsufficientCapacityError))
			})
			It("nodeclass not ready should be recognized", func() {
				Expect(metrics.GetErrorTypeLabelValue(nodeClassNotReadyErr)).To(Equal(metrics.NodeClassNotReadyError))
			})
		})
		Context("when the error is unknown", func() {
			It("should always return empty string", func() {
				Expect(metrics.GetErrorTypeLabelValue(unknownErr)).To(Equal(metrics.MetricLabelErrorDefaultVal))
			})
		})
	})
})
