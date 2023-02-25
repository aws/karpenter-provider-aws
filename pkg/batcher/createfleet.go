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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"knative.dev/pkg/logging"
)

type CreateFleetBatcher struct {
	batcher *Batcher[ec2.CreateFleetInput, ec2.CreateFleetOutput]
}

func NewCreateFleetBatcher(ctx context.Context, ec2api ec2iface.EC2API) *CreateFleetBatcher {
	options := Options[ec2.CreateFleetInput, ec2.CreateFleetOutput]{
		IdleTimeout:   35 * time.Millisecond,
		MaxTimeout:    1 * time.Second,
		MaxItems:      1_000,
		RequestHasher: DefaultHasher[ec2.CreateFleetInput],
		BatchExecutor: execCreateFleetBatch(ec2api),
	}
	return &CreateFleetBatcher{batcher: NewBatcher(ctx, options)}
}

func (b *CreateFleetBatcher) CreateFleet(ctx context.Context, createFleetInput *ec2.CreateFleetInput) (*ec2.CreateFleetOutput, error) {
	if createFleetInput.TargetCapacitySpecification != nil && *createFleetInput.TargetCapacitySpecification.TotalTargetCapacity != 1 {
		return nil, fmt.Errorf("expected to receive a single instance only, found %d", *createFleetInput.TargetCapacitySpecification.TotalTargetCapacity)
	}
	result := b.batcher.Add(ctx, createFleetInput)
	return result.Output, result.Err
}

func execCreateFleetBatch(ec2api ec2iface.EC2API) BatchExecutor[ec2.CreateFleetInput, ec2.CreateFleetOutput] {
	return func(ctx context.Context, inputs []*ec2.CreateFleetInput) []Result[ec2.CreateFleetOutput] {
		results := make([]Result[ec2.CreateFleetOutput], 0, len(inputs))
		firstInput := inputs[0]
		firstInput.TargetCapacitySpecification.TotalTargetCapacity = aws.Int64(int64(len(inputs)))
		output, err := ec2api.CreateFleetWithContext(ctx, firstInput)
		if err != nil {
			for range inputs {
				results = append(results, Result[ec2.CreateFleetOutput]{Err: err})
			}
			return results
		}

		// we can get partial fulfillment of a CreateFleet request, so we:
		// 1) split out the single instance IDs and deliver to each requestor
		// 2) deliver errors to any remaining requestors for which we don't have an instance
		requestIdx := -1
		for _, reservation := range output.Instances {
			for _, instanceID := range reservation.InstanceIds {
				requestIdx++
				if requestIdx >= len(inputs) {
					logging.FromContext(ctx).Errorf("received more instances than requested, ignoring instance %s", aws.StringValue(instanceID))
					continue
				}
				results = append(results, Result[ec2.CreateFleetOutput]{
					Output: &ec2.CreateFleetOutput{
						FleetId: output.FleetId,
						Errors:  output.Errors,
						Instances: []*ec2.CreateFleetInstance{
							{
								InstanceIds:                []*string{instanceID},
								InstanceType:               reservation.InstanceType,
								LaunchTemplateAndOverrides: reservation.LaunchTemplateAndOverrides,
								Lifecycle:                  reservation.Lifecycle,
								Platform:                   reservation.Platform,
							},
						},
					},
				})
			}
		}

		if requestIdx != len(inputs) {
			// we should receive some sort of error, but just in case
			if len(output.Errors) == 0 {
				output.Errors = append(output.Errors, &ec2.CreateFleetError{
					ErrorCode:    aws.String("too few instances returned"),
					ErrorMessage: aws.String("too few instances returned"),
				})
			}
			for i := requestIdx + 1; i < len(inputs); i++ {
				results = append(results, Result[ec2.CreateFleetOutput]{
					Output: &ec2.CreateFleetOutput{
						Errors: output.Errors,
					}})
			}
		}
		return results
	}
}
