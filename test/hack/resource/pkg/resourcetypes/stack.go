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
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

const (
	// maxStackDeletesPerRun bounds the DeleteStack calls of a single run so a backlog can't throttle CloudFormation
	maxStackDeletesPerRun   = 50
	maxDeleteFailedRedrives = 10
	// maxDeleteFailedChecks bounds the read calls spent looking for DELETE_FAILED stacks that can be unblocked
	maxDeleteFailedChecks        = 3 * maxDeleteFailedRedrives
	deleteFailedRedrivesInFlight = 5
	deleteFailedRedriveInterval  = 24 * time.Hour
	stackDeleteWaitTimeout       = 10 * time.Minute
	reviewInProgressMinAge       = 48 * time.Hour

	iamStackPrefix       = "iam-"
	eksctlClusterNameTag = "alpha.eksctl.io/cluster-name"
)

type stackAPI interface {
	cloudformation.DescribeStacksAPIClient
	DescribeStackResources(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error)
	DeleteStack(ctx context.Context, params *cloudformation.DeleteStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error)
}

// StuckStacksError lists the stacks left in DELETE_FAILED by this run. Only the Failed ones fail the sweep.
type StuckStacksError struct {
	// Failed were retried in this run, or couldn't be checked, and are still stuck
	Failed []string
	// Skipped were left for a later run or for a person, and are only reported
	Skipped []string
}

func (e *StuckStacksError) Count() int {
	if e == nil {
		return 0
	}
	return len(e.Failed) + len(e.Skipped)
}

func (e *StuckStacksError) Error() string {
	return fmt.Sprintf("%d stacks stuck in DELETE_FAILED, %d of them failed in this run: %s", e.Count(), len(e.Failed), strings.Join(append(slices.Clone(e.Failed), e.Skipped...), "; "))
}

type Stack struct {
	cloudFormationClient stackAPI
	iamClient            profileReleaseAPI
	eni                  *ENI
	live                 *LiveClusters
	// A single cluster cleanup retries its own stacks right away, without backoff
	candidates map[string]stackCandidate

	now            func() time.Time
	rotate         func(n int) int
	waitStartDelay time.Duration
	waitMinDelay   time.Duration
}

type stackCandidate struct {
	stack   cloudformationtypes.Stack
	backoff bool
}

type redrive struct {
	stack     cloudformationtypes.Stack
	blockedBy string
	roles     []heldRole
	enis      []string
}

type heldRole struct {
	name     string
	profiles []iamtypes.InstanceProfile
}

func NewStack(cloudFormationClient stackAPI, iamClient profileReleaseAPI, ec2Client eniAPI, live *LiveClusters) *Stack {
	return &Stack{
		cloudFormationClient: cloudFormationClient,
		iamClient:            iamClient,
		eni:                  NewENI(ec2Client, live),
		live:                 live,
		candidates:           map[string]stackCandidate{},
		now:                  time.Now,
		rotate:               rand.IntN,
		// DeleteStack can return before the stack leaves DELETE_FAILED, so don't read the old status as the result
		waitStartDelay: 15 * time.Second,
		waitMinDelay:   15 * time.Second,
	}
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
	for _, stack := range selectExpiredStacks(stacks, expirationTime, s.now(), excludedClusters) {
		s.candidates[lo.FromPtr(stack.StackName)] = stackCandidate{stack: stack, backoff: true}
		names = append(names, lo.FromPtr(stack.StackName))
	}
	return names, nil
}

func selectExpiredStacks(stacks []cloudformationtypes.Stack, expirationTime, now time.Time, excludedClusters []string) []cloudformationtypes.Stack {
	return lo.Filter(stacks, func(stack cloudformationtypes.Stack, _ int) bool {
		if stack.StackStatus == cloudformationtypes.StackStatusDeleteComplete || stack.StackStatus == cloudformationtypes.StackStatusDeleteInProgress {
			return false
		}
		if slices.Contains(excludedClusters, stackCluster(stack)) {
			return false
		}
		created := lo.FromPtr(stack.CreationTime)
		if lo.ContainsBy(stack.Tags, func(t cloudformationtypes.Tag) bool {
			return lo.FromPtr(t.Key) == karpenterTestingTag || lo.FromPtr(t.Key) == githubRunURLTag
		}) {
			return created.Before(expirationTime)
		}
		// A failed `aws cloudformation deploy` leaves an untagged iam-<cluster> stack in REVIEW_IN_PROGRESS with no resources
		return stack.StackStatus == cloudformationtypes.StackStatusReviewInProgress &&
			strings.HasPrefix(lo.FromPtr(stack.StackName), iamStackPrefix) &&
			created.Before(expirationTime) &&
			created.Before(now.Add(-reviewInProgressMinAge))
	})
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
			s.candidates[lo.FromPtr(stack.StackName)] = stackCandidate{stack: stack}
			names = append(names, lo.FromPtr(stack.StackName))
		}
	}
	return names, nil
}

func (s *Stack) Cleanup(ctx context.Context, names []string) ([]string, error) {
	candidates := lo.Map(names, func(name string, _ int) stackCandidate {
		if c, ok := s.candidates[name]; ok {
			return c
		}
		return stackCandidate{stack: cloudformationtypes.Stack{StackName: lo.ToPtr(name)}}
	})
	slices.SortStableFunc(candidates, func(a, b stackCandidate) int {
		return lo.FromPtr(a.stack.CreationTime).Compare(lo.FromPtr(b.stack.CreationTime))
	})
	deleteFailed, others := lo.FilterReject(candidates, func(c stackCandidate, _ int) bool {
		return c.stack.StackStatus == cloudformationtypes.StackStatusDeleteFailed
	})

	stuck := &StuckStacksError{}
	redrives := s.selectRedrives(ctx, deleteFailed, stuck)
	var deleted []string
	var errs error
	for _, c := range others[:min(len(others), maxStackDeletesPerRun-len(redrives))] {
		name := lo.FromPtr(c.stack.StackName)
		if _, err := s.cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: lo.ToPtr(name)}); err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, name)
	}

	results := make([]error, len(redrives))
	sem := make(chan struct{}, deleteFailedRedrivesInFlight)
	var wg sync.WaitGroup
	for i := range redrives {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = s.redrive(ctx, redrives[i])
		}()
	}
	wg.Wait()
	for i, err := range results {
		name := lo.FromPtr(redrives[i].stack.StackName)
		if err != nil {
			stuck.Failed = append(stuck.Failed, fmt.Sprintf("%s (%v)", name, err))
			continue
		}
		deleted = append(deleted, name)
	}
	if stuck.Count() > 0 {
		errs = multierr.Append(errs, stuck)
	}
	return deleted, errs
}

// selectRedrives starts at a random stack so that stacks which never recover can't take every slot
func (s *Stack) selectRedrives(ctx context.Context, candidates []stackCandidate, stuck *StuckStacksError) []redrive {
	var eligible []stackCandidate
	for _, c := range candidates {
		if reason := s.skipRedrive(c); reason != "" {
			stuck.Skipped = append(stuck.Skipped, fmt.Sprintf("%s (%s)", lo.FromPtr(c.stack.StackName), reason))
			continue
		}
		eligible = append(eligible, c)
	}
	if len(eligible) > 0 {
		offset := s.rotate(len(eligible))
		eligible = slices.Concat(eligible[offset:], eligible[:offset])
	}
	var redrives []redrive
	checks := 0
	for _, c := range eligible {
		name := lo.FromPtr(c.stack.StackName)
		if len(redrives) >= maxDeleteFailedRedrives || checks >= maxDeleteFailedChecks {
			stuck.Skipped = append(stuck.Skipped, fmt.Sprintf("%s (deferred to a later run by the per-run limits)", name))
			continue
		}
		checks++
		r, err := s.planRedrive(ctx, c.stack)
		var skipped *SkippedError
		switch {
		case errors.As(err, &skipped):
			stuck.Skipped = append(stuck.Skipped, fmt.Sprintf("%s (%v)", name, err))
		case err != nil:
			stuck.Failed = append(stuck.Failed, fmt.Sprintf("%s (%v)", name, err))
		default:
			redrives = append(redrives, r)
		}
	}
	return redrives
}

// skipRedrive returns why a DELETE_FAILED stack must not be retried in this run, or "" to retry it
func (s *Stack) skipRedrive(c stackCandidate) string {
	if last := lastDeleteAttempt(c.stack); c.backoff && last.After(s.now().Add(-deleteFailedRedriveInterval)) {
		return fmt.Sprintf("last delete attempt at %s, retried at most once per %s", last.UTC().Format(time.RFC3339), deleteFailedRedriveInterval)
	}
	clusterName := stackCluster(c.stack)
	if clusterName == "" {
		return "owning cluster unknown"
	}
	if err := s.live.Err(); err != nil {
		return fmt.Sprintf("can't tell if cluster %s still exists, %v", clusterName, err)
	}
	if !s.live.Gone(clusterName) {
		return fmt.Sprintf("cluster %s still exists", clusterName)
	}
	return ""
}

// planRedrive only reads. It finds node roles held by instance profiles and branch ENIs holding the VPC.
func (s *Stack) planRedrive(ctx context.Context, stack cloudformationtypes.Stack) (redrive, error) {
	name, clusterName := lo.FromPtr(stack.StackName), stackCluster(stack)
	out, err := s.cloudFormationClient.DescribeStackResources(ctx, &cloudformation.DescribeStackResourcesInput{StackName: lo.ToPtr(name)})
	if err != nil {
		return redrive{}, fmt.Errorf("describing stack resources, %w", err)
	}
	failed := lo.Filter(out.StackResources, func(r cloudformationtypes.StackResource, _ int) bool {
		return r.ResourceStatus == cloudformationtypes.ResourceStatusDeleteFailed
	})
	r := redrive{stack: stack, blockedBy: strings.Join(lo.Map(failed, func(r cloudformationtypes.StackResource, _ int) string {
		return fmt.Sprintf("%s %s: %s", lo.FromPtr(r.ResourceType), lo.FromPtr(r.LogicalResourceId), lo.FromPtr(r.ResourceStatusReason))
	}), ", ")}

	for _, res := range failed {
		role := lo.FromPtr(res.PhysicalResourceId)
		// Only node roles are held by instance profiles, and the CI role can only list the profiles of those
		if lo.FromPtr(res.ResourceType) != "AWS::IAM::Role" || !strings.HasPrefix(role, nodeRolePrefix) {
			continue
		}
		profiles, err := releasableProfiles(ctx, s.iamClient, role, clusterName)
		if err != nil {
			return redrive{}, fmt.Errorf("blocked by [%s], %w", r.blockedBy, err)
		}
		r.roles = append(r.roles, heldRole{name: role, profiles: profiles})
	}
	if vpc, found := lo.Find(out.StackResources, func(r cloudformationtypes.StackResource) bool {
		return lo.FromPtr(r.ResourceType) == "AWS::EC2::VPC" && lo.FromPtr(r.PhysicalResourceId) != "" &&
			r.ResourceStatus != cloudformationtypes.ResourceStatusDeleteComplete
	}); found {
		enis, err := s.eni.getLeakedBranchENIs(ctx, []ec2types.Filter{{Name: lo.ToPtr("vpc-id"), Values: []string{lo.FromPtr(vpc.PhysicalResourceId)}}}, nil)
		if err != nil {
			return redrive{}, fmt.Errorf("blocked by [%s], listing branch ENIs, %w", r.blockedBy, err)
		}
		r.enis = lo.FilterMap(enis, func(ni ec2types.NetworkInterface, _ int) (string, bool) {
			c, _ := branchENICluster(ni)
			return lo.FromPtr(ni.NetworkInterfaceId), c == clusterName
		})
	}
	return r, nil
}

func (s *Stack) redrive(ctx context.Context, r redrive) error {
	name := lo.FromPtr(r.stack.StackName)
	for _, role := range r.roles {
		if err := detachProfiles(ctx, s.iamClient, role.name, role.profiles); err != nil {
			return fmt.Errorf("blocked by [%s], freeing role %s, %w", r.blockedBy, role.name, err)
		}
	}
	if _, err := s.eni.Cleanup(ctx, r.enis); err != nil {
		return fmt.Errorf("blocked by [%s], deleting branch ENIs, %w", r.blockedBy, err)
	}
	if _, err := s.cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: lo.ToPtr(name)}); err != nil {
		return fmt.Errorf("blocked by [%s], retrying delete, %w", r.blockedBy, err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.waitStartDelay):
	}
	if err := cloudformation.NewStackDeleteCompleteWaiter(s.cloudFormationClient, func(o *cloudformation.StackDeleteCompleteWaiterOptions) {
		o.MinDelay = s.waitMinDelay
		o.MaxDelay = max(s.waitMinDelay, time.Minute)
	}).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: lo.ToPtr(name)}, stackDeleteWaitTimeout); err != nil {
		return fmt.Errorf("blocked by [%s], retried delete didn't complete, %w", r.blockedBy, err)
	}
	return nil
}

func stackCluster(stack cloudformationtypes.Stack) string {
	for _, key := range []string{karpenterTestingTag, eksctlClusterNameTag} {
		if tag, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool { return lo.FromPtr(t.Key) == key }); found && lo.FromPtr(tag.Value) != "" {
			return lo.FromPtr(tag.Value)
		}
	}
	if name := lo.FromPtr(stack.StackName); strings.HasPrefix(name, iamStackPrefix) {
		return strings.TrimPrefix(name, iamStackPrefix)
	}
	return ""
}

func lastDeleteAttempt(stack cloudformationtypes.Stack) time.Time {
	return lo.FromPtr(lo.CoalesceOrEmpty(stack.DeletionTime, stack.LastUpdatedTime, stack.CreationTime))
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
