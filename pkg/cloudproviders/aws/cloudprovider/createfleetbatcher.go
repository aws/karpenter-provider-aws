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

package cloudprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"knative.dev/pkg/logging"
)

// CreateFleetBatcher is used to batch CreateFleet calls from the cloud provider with identical parameters into a single
// call that launches more instances simultaneously.
type CreateFleetBatcher struct {
	ctx      context.Context
	ec2api   ec2iface.EC2API
	mu       sync.Mutex
	trigger  chan struct{}
	requests map[uint64][]*createFleetRequest
}

func NewCreateFleetBatcher(ctx context.Context, ec2api ec2iface.EC2API) *CreateFleetBatcher {
	b := &CreateFleetBatcher{
		ctx:      ctx,
		ec2api:   ec2api,
		requests: map[uint64][]*createFleetRequest{},
		trigger:  make(chan struct{}),
	}
	go b.run()
	return b
}

type createFleetRequest struct {
	ctx       context.Context
	hash      uint64
	input     *ec2.CreateFleetInput
	requestor chan createFleetResult
}

type createFleetResult struct {
	output *ec2.CreateFleetOutput
	err    error
}

func (b *CreateFleetBatcher) CreateFleet(ctx context.Context, createFleetInput *ec2.CreateFleetInput) (*ec2.CreateFleetOutput, error) {
	if createFleetInput.TargetCapacitySpecification != nil && *createFleetInput.TargetCapacitySpecification.TotalTargetCapacity != 1 {
		return nil, fmt.Errorf("expected to receive a single instance only, found %d", *createFleetInput.TargetCapacitySpecification.TotalTargetCapacity)
	}
	hash, err := hashstructure.Hash(createFleetInput, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		logging.FromContext(ctx).Errorf("error hashing")
	}

	result := <-b.createFleet(ctx, hash, createFleetInput)
	return result.output, result.err
}

func (b *CreateFleetBatcher) createFleet(ctx context.Context, hash uint64, createFleetInput *ec2.CreateFleetInput) chan createFleetResult {
	request := &createFleetRequest{
		ctx:   ctx,
		hash:  hash,
		input: createFleetInput,
		// The requestor channel is buffered to ensure that the exec runner can always write the result out preventing
		// any single caller from blocking the others. Specifically since we register our request and then trigger, the
		// request may be processed while the triggering blocks.
		requestor: make(chan createFleetResult, 1),
	}
	b.mu.Lock()
	b.requests[hash] = append(b.requests[hash], request)
	b.mu.Unlock()
	b.trigger <- struct{}{}
	return request.requestor
}

func (b *CreateFleetBatcher) run() {
	for {
		select {
		// context that we started with has completed so the app is shutting down
		case <-b.ctx.Done():
			return
		case <-b.trigger:
			// wait to start the batch of create fleet calls
		}
		b.waitForIdle()
		b.runCalls()
	}
}

func (b *CreateFleetBatcher) waitForIdle() {
	timeout := time.NewTimer(100 * time.Millisecond)
	idle := time.NewTimer(10 * time.Millisecond)
	for {
		select {
		case <-b.trigger:
			if !idle.Stop() {
				<-idle.C
			}
			idle.Reset(10 * time.Millisecond)
		case <-timeout.C:
			return
		case <-idle.C:
			return
		}
	}
}

func (b *CreateFleetBatcher) runCalls() {
	b.mu.Lock()
	defer b.mu.Unlock()
	requestMap := b.requests
	b.requests = map[uint64][]*createFleetRequest{}

	for _, requestBatch := range requestMap {
		// we know that these create fleet calls are identical so we can just run the first one and increase the number
		// of instances that we request
		call := requestBatch[0]
		// deep copy the input we are about to modify so that we don't modify any caller's input parameter
		input, err := deepCopy(call.input)
		if err != nil {
			// shouldn't occur, but if it does we log an error and just modify the caller's input so we
			// can continue to launch instances
			logging.FromContext(context.Background()).Infof("error copying input, %s", err)
			input = call.input
		}
		input.TargetCapacitySpecification.SetTotalTargetCapacity(int64(len(requestBatch)))
		outputs, err := b.ec2api.CreateFleetWithContext(call.ctx, input)

		// error occurred at the CreateFleet call level, so notify all requestors of the same error
		if err != nil {
			for i := range requestBatch {
				requestBatch[i].requestor <- createFleetResult{
					output: nil,
					err:    err,
				}
			}
			continue
		}

		// we can get partial fulfillment of a CreateFleet request, so we:
		// 1) split out the single instance IDs and deliver to each requestor
		// 2) deliver errors to any remaining requestors for which we don't have an instance

		requestIdx := -1
		for _, reservation := range outputs.Instances {
			for _, instanceID := range reservation.InstanceIds {
				requestIdx++
				if requestIdx >= len(requestBatch) {
					logging.FromContext(call.ctx).Errorf("received more instances than requested, ignoring instance %s", aws.StringValue(instanceID))
					continue
				}
				// split out the single result into multiple create fleet outputs
				requestBatch[requestIdx].requestor <- createFleetResult{
					output: &ec2.CreateFleetOutput{
						FleetId: outputs.FleetId,
						Errors:  outputs.Errors,
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
				}
			}
		}

		if requestIdx != len(requestBatch) {
			// we should receive some sort of error, but just in case
			if len(outputs.Errors) == 0 {
				outputs.Errors = append(outputs.Errors, &ec2.CreateFleetError{
					ErrorCode:    aws.String("too few instances returned"),
					ErrorMessage: aws.String("too few instances returned"),
				})
			}
			for i := requestIdx + 1; i < len(requestBatch); i++ {
				requestBatch[i].requestor <- createFleetResult{
					output: &ec2.CreateFleetOutput{
						Errors: outputs.Errors,
					},
				}
			}
		}
	}
}

func deepCopy(v *ec2.CreateFleetInput) (*ec2.CreateFleetInput, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&buf)
	var cp ec2.CreateFleetInput
	if err := dec.Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
}
