package resource

import (
	"context"
	"fmt"
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

func (s *Stack) Type() string {
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
				return lo.FromPtr(t.Key) == githubRunURLTag
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

func (s *Stack) Get(_ context.Context, clusterName string) (names []string, err error) {
	return []string{fmt.Sprintf("iam-%s", clusterName), fmt.Sprintf("eksctl-%s-cluster", clusterName)}, nil
}

// Cleanup any old stacks that were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (s *Stack) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var errs error
	deleted := []string{}
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
