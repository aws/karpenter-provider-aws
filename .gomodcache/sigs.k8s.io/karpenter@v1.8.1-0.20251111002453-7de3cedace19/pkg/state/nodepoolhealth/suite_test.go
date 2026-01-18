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

package nodepoolhealth_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"

	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"
)

var (
	npState *nodepoolhealth.State
	npUUID  types.UID
)

var _ = BeforeSuite(func() {
	npUUID = uuid.NewUUID()
	npState = nodepoolhealth.NewState()
})

var _ = AfterEach(func() {
	npState.SetStatus(npUUID, nodepoolhealth.StatusUnknown)
})

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodePoolHealthState")
}

var _ = Describe("NodePoolHealthState", func() {
	It("should record all concurrent updates correctly", func() {
		concurrentState := nodepoolhealth.NewState()
		done := make(chan bool, 4)
		sameUID := types.UID("test")
		// Add exactly 2 false values (50% of buffer size 4)
		for i := 0; i < 2; i++ {
			go func() {
				defer func() { done <- true }()
				concurrentState.Update(sameUID, false)
			}()
		}
		// Add exactly 2 true values
		for i := 0; i < 2; i++ {
			go func() {
				defer func() { done <- true }()
				concurrentState.Update(sameUID, true)
			}()
		}
		for i := 0; i < 4; i++ {
			<-done
		}
		// With proper locks: exactly 50% false = StatusUnhealthy
		// Without locks: could lose updates = wrong status
		Expect(concurrentState.Status(sameUID)).To(Equal(nodepoolhealth.StatusUnhealthy))
	})
	It("should expect status unknown for a new nodePool with empty buffer", func() {
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusUnknown))
	})
	It("should expect status healthy for a nodePool with one true entry", func() {
		npState.Update(npUUID, true)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusHealthy))
	})
	It("should expect status unhealthy for a nodePool with two false entries", func() {
		npState.Update(npUUID, false)
		npState.Update(npUUID, false)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusUnhealthy))
	})
	It("should expect status unhealthy for a nodePool with two false entries and one true entry", func() {
		npState.SetStatus(npUUID, nodepoolhealth.StatusUnhealthy)
		npState.Update(npUUID, true)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusUnhealthy))
	})
	It("should return the correct status in case of multiple nodepools stored in the state", func() {
		npState.Update(npUUID, true)
		npUUID2 := uuid.NewUUID()
		npState.Update(npUUID2, false)
		npState.Update(npUUID2, false)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusHealthy))
		Expect(npState.Status(npUUID2)).To(Equal(nodepoolhealth.StatusUnhealthy))
		npState.SetStatus(npUUID2, nodepoolhealth.StatusUnknown)
		Expect(npState.Status(npUUID2)).To(Equal(nodepoolhealth.StatusUnknown))
	})
	It("should create a copy of the tracker and update it and not the original tracker in case of Dry Run", func() {
		npState.SetStatus(npUUID, nodepoolhealth.StatusHealthy)
		npState.Update(npUUID, false)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusHealthy))

		Expect(npState.DryRun(npUUID, false).Status()).To(Equal(nodepoolhealth.StatusUnhealthy))
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusHealthy))
	})
	It("should reset the buffer first when setting status", func() {
		npState.Update(npUUID, false)
		npState.Update(npUUID, false)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusUnhealthy))

		// This SetStatus call should first reset the buffer and then add entries to the buffer such that the status becomes healthy
		npState.SetStatus(npUUID, nodepoolhealth.StatusHealthy)
		Expect(npState.Status(npUUID)).To(Equal(nodepoolhealth.StatusHealthy))
	})
})
