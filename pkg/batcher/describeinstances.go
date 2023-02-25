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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"
)

type DescribeInstancesBatcher struct {
	batcher *Batcher[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput]
}

func NewDescribeInstancesBatcher(ctx context.Context, ec2api ec2iface.EC2API) *DescribeInstancesBatcher {
	options := Options[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput]{
		IdleTimeout:   100 * time.Millisecond,
		MaxTimeout:    1 * time.Second,
		MaxItems:      500,
		RequestHasher: FilterHasher,
		BatchExecutor: execDescribeInstancesBatch(ec2api),
	}
	return &DescribeInstancesBatcher{batcher: NewBatcher(ctx, options)}
}

func (b *DescribeInstancesBatcher) DescribeInstances(ctx context.Context, describeInstancesInput *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if len(describeInstancesInput.InstanceIds) != 1 {
		return nil, fmt.Errorf("expected to receive a single instance only, found %d", len(describeInstancesInput.InstanceIds))
	}
	result := b.batcher.Add(ctx, describeInstancesInput)
	return result.Output, result.Err
}

func FilterHasher(ctx context.Context, input *ec2.DescribeInstancesInput) uint64 {
	hash, err := hashstructure.Hash(input.Filters, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		logging.FromContext(ctx).Errorf("error hashing")
	}
	return hash
}

func execDescribeInstancesBatch(ec2api ec2iface.EC2API) BatchExecutor[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput] {
	return func(ctx context.Context, inputs []*ec2.DescribeInstancesInput) []Result[ec2.DescribeInstancesOutput] {
		results := make([]Result[ec2.DescribeInstancesOutput], len(inputs))
		firstInput := inputs[0]
		// aggregate instanceIDs into 1 input
		for _, input := range inputs[1:] {
			firstInput.InstanceIds = append(firstInput.InstanceIds, input.InstanceIds...)
		}

		missingInstanceIDs := lo.SliceToMap(firstInput.InstanceIds, func(instanceID *string) (string, struct{}) { return *instanceID, struct{}{} })

		// Execute fully aggregated request
		// We don't care about the error here since we'll break up the batch upon any sort of failure
		_ = ec2api.DescribeInstancesPagesWithContext(ctx, firstInput, func(dio *ec2.DescribeInstancesOutput, b bool) bool {
			for _, r := range dio.Reservations {
				for _, instance := range r.Instances {
					if _, reqID, ok := lo.FindLastIndexOf(inputs, func(input *ec2.DescribeInstancesInput) bool {
						return *input.InstanceIds[0] == *instance.InstanceId
					}); ok {
						delete(missingInstanceIDs, *instance.InstanceId)
						inst := instance
						results[reqID] = Result[ec2.DescribeInstancesOutput]{Output: &ec2.DescribeInstancesOutput{
							Reservations: []*ec2.Reservation{{
								OwnerId:       r.OwnerId,
								RequesterId:   r.RequesterId,
								ReservationId: r.ReservationId,
								Instances:     []*ec2.Instance{inst},
							}},
						}}
					}
				}
			}
			return true
		})

		// Some or all instances may have failed to be described due to eventual consistency or transient zonal issue.
		// A single instance lookup failure can result in all of an availability zone's instances failing to describe.
		// So we try to describe them individually now. This should be rare and only results in a handfull of extra calls per batch than without batching.
		var wg sync.WaitGroup
		for instanceID := range missingInstanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer wg.Done()
				// try to execute separately
				out, err := ec2api.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{
					Filters:     firstInput.Filters,
					InstanceIds: []*string{aws.String(instanceID)}})
				// Order by inputs' index so that instance IDs from input and output are in the same order
				_, reqID, ok := lo.FindIndexOf(inputs, func(input *ec2.DescribeInstancesInput) bool {
					return *input.InstanceIds[0] == instanceID
				})
				// if the instance ID returned from DescribeInstances was not passed as a DescribeInstancesInput, just skip
				if !ok {
					return
				}
				results[reqID] = Result[ec2.DescribeInstancesOutput]{Output: out, Err: err}
			}(instanceID)
		}
		wg.Wait()
		return results
	}
}
