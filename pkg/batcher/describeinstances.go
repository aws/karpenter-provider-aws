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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type DescribeInstancesBatcher struct {
	batcher *Batcher[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput]
}

func NewDescribeInstancesBatcher(ctx context.Context, ec2api sdk.EC2API) *DescribeInstancesBatcher {
	options := Options[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput]{
		Name:          "describe_instances",
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
		return nil, serrors.Wrap(fmt.Errorf("expected to receive a single instance only"), "instance-count", len(describeInstancesInput.InstanceIds))
	}
	result := b.batcher.Add(ctx, describeInstancesInput)
	return result.Output, result.Err
}

func FilterHasher(ctx context.Context, input *ec2.DescribeInstancesInput) uint64 {
	hash, err := hashstructure.Hash(input.Filters, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		log.FromContext(ctx).Error(err, "failed hashing input filters")
	}
	return hash
}

func execDescribeInstancesBatch(ec2api sdk.EC2API) BatchExecutor[ec2.DescribeInstancesInput, ec2.DescribeInstancesOutput] {
	return func(ctx context.Context, inputs []*ec2.DescribeInstancesInput) []Result[ec2.DescribeInstancesOutput] {
		results := make([]Result[ec2.DescribeInstancesOutput], len(inputs))
		aggregatedInput := prepareAggregatedInput(inputs)

		missingInstanceIDs := sets.NewString(lo.Map(aggregatedInput.InstanceIds, func(i string, _ int) string { return i })...)
		paginator := ec2.NewDescribeInstancesPaginator(ec2api, aggregatedInput)

		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				break
			}

			for _, r := range output.Reservations {
				for _, instance := range r.Instances {
					missingInstanceIDs.Delete(*instance.InstanceId)
					// Find all indexes where we are requesting this instance and populate with the result
					for reqID := range inputs {
						if inputs[reqID].InstanceIds[0] == *instance.InstanceId {
							inst := instance
							results[reqID] = Result[ec2.DescribeInstancesOutput]{Output: &ec2.DescribeInstancesOutput{
								Reservations: []ec2types.Reservation{{
									OwnerId:       r.OwnerId,
									RequesterId:   r.RequesterId,
									ReservationId: r.ReservationId,
									Instances:     []ec2types.Instance{inst},
								}},
								ResultMetadata: output.ResultMetadata,
							}}
						}
					}
				}
			}
		}

		// If we have any missing instanceIDs, we need to describe them individually

		// Some or all instances may have failed to be described due to eventual consistency or transient zonal issue.
		// A single instance lookup failure can result in all of an availability zone's instances failing to describe.
		// So we try to describe them individually now. This should be rare and only results in a handfull of extra calls per batch than without batching.
		var wg sync.WaitGroup
		for instanceID := range missingInstanceIDs {
			wg.Add(1)
			go func(instanceID string) {
				defer wg.Done()
				// try to execute separately
				out, err := ec2api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
					Filters:     aggregatedInput.Filters,
					InstanceIds: []string{instanceID},
				})

				// Find all indexes where we are requesting this instance and populate with the result
				for reqID := range inputs {
					if inputs[reqID].InstanceIds[0] == instanceID {
						results[reqID] = Result[ec2.DescribeInstancesOutput]{Output: out, Err: err}
					}
				}
			}(instanceID)
		}
		wg.Wait()
		return results
	}
}

func prepareAggregatedInput(inputs []*ec2.DescribeInstancesInput) *ec2.DescribeInstancesInput {
	aggregatedInput := inputs[0]

	// aggregate instanceIDs into 1 input
	for _, input := range inputs[1:] {
		aggregatedInput.InstanceIds = append(aggregatedInput.InstanceIds, input.InstanceIds...)
	}

	// MaxResults is not supported when the request includes instanceids
	// Ref: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html#api-pagination
	if len(aggregatedInput.InstanceIds) == 0 {
		// MaxResults for DescribeInstances is capped at 1000
		aggregatedInput.MaxResults = lo.ToPtr[int32](1000)
	}

	return aggregatedInput
}
