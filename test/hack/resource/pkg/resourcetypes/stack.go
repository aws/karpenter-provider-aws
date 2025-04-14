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
	"golang.org/x/exp/slices"
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

func (s *Stack) Global() bool {
	return false
}

func (s *Stack) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (names []string, err error) {
	stacks, err := s.getAllStacks(ctx)
	if err != nil {
		return names, err
	}

	activeStacks := lo.Reject(stacks, func(s cloudformationtypes.Stack, _ int) bool {
		return s.StackStatus == cloudformationtypes.StackStatusDeleteComplete ||
			s.StackStatus == cloudformationtypes.StackStatusDeleteInProgress
	})
	for _, stack := range activeStacks {
		clusterName, found := lo.Find(stack.Tags, func(tag cloudformationtypes.Tag) bool {
			return *tag.Key == karpenterTestingTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
			return lo.FromPtr(t.Key) == karpenterTestingTag || lo.FromPtr(t.Key) == githubRunURLTag
		}); found && lo.FromPtr(stack.CreationTime).Before(expirationTime) {
			names = append(names, lo.FromPtr(stack.StackName))
		}
	}
	return names, err
}

func (s *Stack) CountAll(ctx context.Context) (count int, err error) {
	stacks, err := s.getAllStacks(ctx)
	if err != nil {
		return count, err
	}

	return len(stacks), nil
}

func (s *Stack) Get(ctx context.Context, clusterName string) (names []string, err error) {
	stacks, err := s.getAllStacks(ctx)
	if err != nil {
		return names, err
	}

	for _, stack := range stacks {
		if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
			return lo.FromPtr(t.Key) == karpenterTestingTag && lo.FromPtr(t.Value) == clusterName
		}); found {
			names = append(names, lo.FromPtr(stack.StackName))
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

func (s *Stack) getAllStacks(ctx context.Context) (stacks []cloudformationtypes.Stack, err error) {
	paginator := cloudformation.NewDescribeStacksPaginator(s.cloudFormationClient, &cloudformation.DescribeStacksInput{})

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return stacks, err
		}
		stacks = append(stacks, out.Stacks...)
	}

	return stacks, nil
}
