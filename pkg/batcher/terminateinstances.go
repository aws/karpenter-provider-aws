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
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"karpenter-provider-aws/pkg/aws/awsclient"
	"karpenter-provider-aws/pkg/aws/awsapi"
)

type TerminateInstancesBatcher struct {
	batcher *Batcher[ec2.TerminateInstancesInput, ec2.TerminateInstancesOutput]
}

func NewTerminateInstancesBatcher(ctx context.Context, ec2Client awsapi.EC2API) *TerminateInstancesBatcher {
	options := Options[ec2.TerminateInstancesInput, ec2.TerminateInstancesOutput]{
		Name:          "terminate_instances",
		IdleTimeout:   100 * time.Millisecond,
		MaxTimeout:    1 * time.Second,
		MaxItems:      500,
		RequestHasher: OneBucketHasher[ec2.TerminateInstancesInput],
		BatchExecutor: execTerminateInstancesBatch(ec2Client),
	}
	return &TerminateInstancesBatcher{batcher: NewBatcher(ctx, options)}
}

func (b *TerminateInstancesBatcher) TerminateInstances(ctx context.Context, terminateInstancesInput *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	if len(terminateInstancesInput.InstanceIds) != 1 {
		return nil, fmt.Errorf("expected to receive a single instance only, found %d", len(terminateInstancesInput.InstanceIds))
	}
	result := b.batcher.Add(ctx, terminateInstancesInput)
	return result.Output, result.Err
}

func execTerminateInstancesBatch(ec2Client awsapi.EC2API) BatchExecutor[ec2.TerminateInstancesInput, ec2.TerminateInstancesOutput] {
	return func(ctx context.Context, inputs []*ec2.TerminateInstancesInput) []Result[ec2.TerminateInstancesOutput] {
		results := make([]Result[ec2.TerminateInstancesOutput], len(inputs))
		firstInput := inputs[0]

		// aggregate instanceIDs into 1 input
		for _, input := range inputs[1:] {
			firstInput.InstanceIds = append(firstInput.InstanceIds, input.InstanceIds...)
		}
		// Create a set of all instance IDs
		stillRunning := sets.NewString(lo.Map(firstInput.InstanceIds, func(i *string, _ int) string { return *i })...)

		// Execute fully aggregated request
		// We don't care about the error here since we'll break up the batch upon any sort of failure
		output, err := ec2Client.TerminateInstances(ctx, firstInput)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed terminating instances")
		}

		if output == nil {
			output = &ec2.TerminateInstancesOutput{}
		}

		// Check the fulfillment for partial or no fulfillment by checking for missing instance IDs or invalid instance states
		for _, instanceStateChanges := range output.TerminatingInstances {
			// Remove all instances that successfully terminated and separate into distinct outputs
			if lo.Contains([]string{types.InstanceStateNameShuttingDown, types.InstanceStateNameTerminated}, string(instanceStateChanges.CurrentState.Name)) {
				stillRunning.Delete(*instanceStateChanges.InstanceId)

				// Find all indexes where we are requesting this instance and populate with the result
				for reqID := range inputs {
					if *inputs[reqID].InstanceIds[0] == *instanceStateChanges.InstanceId {
						results[reqID] = Result[ec2.TerminateInstancesOutput]{
							Output: &ec2.TerminateInstancesOutput{
								TerminatingInstances: []*ec2.InstanceStateChange{{
									InstanceId:    instanceStateChanges.InstanceId,
									CurrentState:  instanceStateChanges.CurrentState,
									PreviousState: instanceStateChanges.PreviousState,
								}},
							},
						}
					}
				}
			}
		}

		// Some or all instances may have failed to terminate due to instance protection or some other error.
		// A single instance failure can result in all of an availability zone's instances failing to terminate.
		// So we try to terminate them individually now. This should be rare and only results in 1 extra call per batch than without batching.
		var wg sync.WaitGroup
		for instanceID := range stillRunning {
			wg.Add(1)
			go func(instanceID string) {
				defer wg.Done()
				// try to execute separately
				out, err := ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []*string{instanceID}})

				// Find all indexes where we are requesting this instance and populate with the result
				for reqID := range inputs {
					if *inputs[reqID].InstanceIds[0] == instanceID {
						results[reqID] = Result[ec2.TerminateInstancesOutput]{Output: out, Err: err}
					}
				}
			}(instanceID)
		}
		wg.Wait()
		return results
	}
}
