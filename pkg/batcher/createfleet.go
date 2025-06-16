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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/serrors"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CreateFleetBatcher struct {
	batcher *Batcher[ec2.CreateFleetInput, ec2.CreateFleetOutput]
}

func NewCreateFleetBatcher(ctx context.Context, ec2api sdk.EC2API) *CreateFleetBatcher {
	options := Options[ec2.CreateFleetInput, ec2.CreateFleetOutput]{
		Name:          "create_fleet",
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
		return nil, serrors.Wrap(fmt.Errorf("expected to receive a single instance only"), "instance-count", *createFleetInput.TargetCapacitySpecification.TotalTargetCapacity)
	}
	result := b.batcher.Add(ctx, createFleetInput)
	return result.Output, result.Err
}

// processInstances processes the instances from a CreateFleet output and returns the new request index
func processInstances(ctx context.Context, output *ec2.CreateFleetOutput, totalCapacity int, results []Result[ec2.CreateFleetOutput], startIdx int) ([]Result[ec2.CreateFleetOutput], int) {
	requestIdx := startIdx
	if output == nil {
		return results, requestIdx
	}
	for _, reservation := range output.Instances {
		for _, instanceID := range reservation.InstanceIds {
			if requestIdx >= totalCapacity {
				log.FromContext(ctx).Error(serrors.Wrap(fmt.Errorf("received more instances than requested, ignoring instance"), "instance-id", instanceID), "received error while batching")
				continue
			}
			results = append(results, Result[ec2.CreateFleetOutput]{
				Output: &ec2.CreateFleetOutput{
					FleetId: output.FleetId,
					Errors:  output.Errors,
					Instances: []ec2types.CreateFleetInstance{
						{
							InstanceIds:                []string{instanceID},
							InstanceType:               reservation.InstanceType,
							LaunchTemplateAndOverrides: reservation.LaunchTemplateAndOverrides,
							Lifecycle:                  reservation.Lifecycle,
							Platform:                   reservation.Platform,
						},
					},
					ResultMetadata: output.ResultMetadata,
				},
			})
			requestIdx++
		}
	}
	return results, requestIdx
}

func execCreateFleetBatch(ec2api sdk.EC2API) BatchExecutor[ec2.CreateFleetInput, ec2.CreateFleetOutput] {
	return func(ctx context.Context, inputs []*ec2.CreateFleetInput) []Result[ec2.CreateFleetOutput] {
		results := make([]Result[ec2.CreateFleetOutput], 0, len(inputs))
		if len(inputs) == 0 {
			return results
		}

		maxRetries := 3
		retryCount := 0
		requestIdx := 0
		var output *ec2.CreateFleetOutput

		for retryCount < maxRetries && requestIdx < len(inputs) {
			currentInput := inputs[requestIdx]
			currentInput.TargetCapacitySpecification.TotalTargetCapacity = aws.Int32(int32(len(inputs) - requestIdx))
			var err error
			output, err = ec2api.CreateFleet(ctx, currentInput)
			if err != nil {
				log.FromContext(ctx).Error(err, "retry attempt failed", "attempt", retryCount+1)
				retryCount++
				continue
			}

			results, requestIdx = processInstances(ctx, output, len(inputs), results, requestIdx)
			if requestIdx < len(inputs) {
				retryCount++
			}
		}

		// If we still have remaining requests after all retries, return errors
		if requestIdx < len(inputs) {
			if output == nil || len(output.Errors) == 0 {
				output = &ec2.CreateFleetOutput{
					Errors: []ec2types.CreateFleetError{
						{
							ErrorCode:    aws.String("too few instances returned after retries"),
							ErrorMessage: aws.String(fmt.Sprintf("failed to create all instances after %d retries", maxRetries)),
						},
					},
				}
			}
			for i := requestIdx; i < len(inputs); i++ {
				results = append(results, Result[ec2.CreateFleetOutput]{
					Output: &ec2.CreateFleetOutput{
						Errors:         output.Errors,
						ResultMetadata: output.ResultMetadata,
					}})
			}
		}
		return results
	}
}
