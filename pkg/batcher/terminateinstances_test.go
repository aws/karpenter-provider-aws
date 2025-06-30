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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/karpenter-provider-aws/pkg/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TerminateInstances Batcher", func() {
	var cfb *TerminateInstancesBatcher

	BeforeEach(func() {
		fakeEC2API.Reset()
		cfb = NewTerminateInstancesBatcher(ctx, fakeEC2API)
	})

	It("should batch input into a single call", func() {
		instanceIDs := []string{"i-1", "i-2", "i-3", "i-4", "i-5"}
		for _, id := range instanceIDs {
			fakeEC2API.Instances.Store(id, ec2types.Instance{})
		}

		var wg sync.WaitGroup
		var receivedInstance int64
		for _, instanceID := range instanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{instanceID},
				})
				Expect(err).To(BeNil())
				atomic.AddInt64(&receivedInstance, 1)
				Expect(rsp.TerminatingInstances).To(HaveLen(1))
			}(instanceID)
		}
		wg.Wait()

		Expect(receivedInstance).To(BeNumerically("==", len(instanceIDs)))
		Expect(fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		call := fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Pop()
		Expect(len(call.InstanceIds)).To(BeNumerically("==", len(instanceIDs)))
	})
	It("should batch input correctly when receiving multiple calls with the same instance id", func() {
		instanceIDs := []string{"i-1", "i-1", "i-1", "i-2", "i-2"}
		for _, id := range instanceIDs {
			fakeEC2API.Instances.Store(id, ec2types.Instance{})
		}

		var wg sync.WaitGroup
		var receivedInstance int64
		for _, instanceID := range instanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{instanceID},
				})
				Expect(err).To(BeNil())
				atomic.AddInt64(&receivedInstance, 1)
				Expect(rsp.TerminatingInstances).To(HaveLen(1))
			}(instanceID)
		}
		wg.Wait()

		Expect(receivedInstance).To(BeNumerically("==", len(instanceIDs)))
		Expect(fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		call := fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Pop()
		Expect(len(call.InstanceIds)).To(BeNumerically("==", len(instanceIDs)))
	})
	It("should handle partial terminations on batched call and recover with individual requests", func() {
		instanceIDs := []string{"i-1", "i-2", "i-3"}
		// Output with only the first Terminating Instance
		fakeEC2API.TerminateInstancesBehavior.Output.Set(&ec2.TerminateInstancesOutput{
			TerminatingInstances: []ec2types.InstanceStateChange{
				{
					PreviousState: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
					CurrentState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameShuttingDown, Code: aws.Int32(32)},
					InstanceId:    aws.String(instanceIDs[0]),
				},
			},
		})
		var wg sync.WaitGroup
		var receivedInstance int64
		var numUnfulfilled int64
		for _, instanceID := range instanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer GinkgoRecover()
				defer wg.Done()
				rsp, err := cfb.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{instanceID},
				})
				Expect(err).To(BeNil())
				Expect(len(rsp.TerminatingInstances)).To(BeNumerically("<=", 1))
				if len(rsp.TerminatingInstances) == 0 {
					atomic.AddInt64(&numUnfulfilled, 1)
				} else {
					atomic.AddInt64(&receivedInstance, 1)
				}
			}(instanceID)
		}
		wg.Wait()

		// should execute the batched call and then one for each that failed in the batched request
		Expect(fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Len()).To(BeNumerically("==", 3))
		lastCall := fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Pop()
		Expect(len(lastCall.InstanceIds)).To(BeNumerically("==", 1))
		nextToLastCall := fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Pop()
		Expect(len(nextToLastCall.InstanceIds)).To(BeNumerically("==", 1))
		firstCall := fakeEC2API.TerminateInstancesBehavior.CalledWithInput.Pop()
		Expect(len(firstCall.InstanceIds)).To(BeNumerically("==", 3))
		Expect(receivedInstance).To(BeNumerically("==", 3))
		Expect(numUnfulfilled).To(BeNumerically("==", 0))
	})
	It("should return errors to all callers when erroring on the batched call", func() {
		instanceIDs := []string{"i-1", "i-2", "i-3", "i-4", "i-5"}
		fakeEC2API.TerminateInstancesBehavior.Error.Set(fmt.Errorf("error"), fake.MaxCalls(6))
		var wg sync.WaitGroup
		for _, instanceID := range instanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer GinkgoRecover()
				defer wg.Done()
				_, err := cfb.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{instanceID},
				})
				Expect(err).ToNot(BeNil())
			}(instanceID)
		}
		wg.Wait()
		// We expect 6 calls since we do one full batched call and 5 individual since the batched call returns an error
		Expect(fakeEC2API.TerminateInstancesBehavior.Calls()).To(BeNumerically("==", 6))
	})
})
