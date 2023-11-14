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

package resourcetypes

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type Stack struct {
	cloudFormationClient *cloudformation.Client
}

func NewStack(cloudFormationClient *cloudformation.Client) *Stack {
	return &Stack{cloudFormationClient: cloudFormationClient}
}

func (s *Stack) String() string {
	return "CloudformationStacks"
}

func (s *Stack) GetExpired(ctx context.Context, expirationTime time.Time) (names []string, err error) {
	var nextToken *string
	for {
		out, err := s.cloudFormationClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}

		stacks := lo.Reject(out.Stacks, func(s cloudformationtypes.Stack, _ int) bool {
			return s.StackStatus == cloudformationtypes.StackStatusDeleteComplete ||
				s.StackStatus == cloudformationtypes.StackStatusDeleteInProgress
		})
		for _, stack := range stacks {
			if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
				return lo.FromPtr(t.Key) == karpenterTestingTag || lo.FromPtr(t.Key) == githubRunURLTag
			}); found && lo.FromPtr(stack.CreationTime).Before(expirationTime) {
				names = append(names, lo.FromPtr(stack.StackName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, err
}

func (s *Stack) Get(ctx context.Context, clusterName string) (names []string, err error) {
	var nextToken *string
	for {
		out, err := s.cloudFormationClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}
		for _, stack := range out.Stacks {
			if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
				return lo.FromPtr(t.Key) == karpenterTestingTag && lo.FromPtr(t.Value) == clusterName
			}); found {
				names = append(names, lo.FromPtr(stack.StackName))
			}
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, nil
}

// Cleanup any old stacks that were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (s *Stack) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var deleted []string
	var errs error
	for i := range names {
		_, err := s.cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: lo.ToPtr(names[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, names[i])
	}
	return deleted, errs
}
