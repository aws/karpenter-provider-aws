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
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

// fakeCloudFormation's DeleteStack asks deleteSucceeds whether the stack's blockers are gone, and takes deleteDelay
type fakeCloudFormation struct {
	mu             sync.Mutex
	stacks         map[string]*cloudformationtypes.Stack
	resources      map[string][]cloudformationtypes.StackResource
	deleteSucceeds func(name string) bool
	deletes        []string
	deleteDelay    time.Duration
	describes      int
	inFlight       int
	maxInFlight    int
}

func (f *fakeCloudFormation) DescribeStacks(_ context.Context, in *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if in.StackName != nil {
		return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{*f.stacks[lo.FromPtr(in.StackName)]}}, nil
	}
	out := &cloudformation.DescribeStacksOutput{}
	for _, name := range slices.Sorted(maps.Keys(f.stacks)) {
		if f.stacks[name].StackStatus != cloudformationtypes.StackStatusDeleteComplete {
			out.Stacks = append(out.Stacks, *f.stacks[name])
		}
	}
	return out, nil
}

func (f *fakeCloudFormation) DescribeStackResources(_ context.Context, in *cloudformation.DescribeStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.describes++
	return &cloudformation.DescribeStackResourcesOutput{StackResources: f.resources[lo.FromPtr(in.StackName)]}, nil
}

func (f *fakeCloudFormation) DeleteStack(_ context.Context, in *cloudformation.DeleteStackInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error) {
	name := lo.FromPtr(in.StackName)
	f.mu.Lock()
	f.inFlight++
	f.maxInFlight = max(f.maxInFlight, f.inFlight)
	f.mu.Unlock()
	time.Sleep(f.deleteDelay)
	ok := f.deleteSucceeds == nil || f.deleteSucceeds(name)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight--
	f.deletes = append(f.deletes, name)
	f.stacks[name].StackStatus = lo.Ternary(ok, cloudformationtypes.StackStatusDeleteComplete, cloudformationtypes.StackStatusDeleteFailed)
	return &cloudformation.DeleteStackOutput{}, nil
}

func testStack(name string, status cloudformationtypes.StackStatus, created time.Time, tags map[string]string) *cloudformationtypes.Stack {
	stack := &cloudformationtypes.Stack{StackName: lo.ToPtr(name), StackStatus: status, CreationTime: lo.ToPtr(created)}
	for _, k := range slices.Sorted(maps.Keys(tags)) {
		stack.Tags = append(stack.Tags, cloudformationtypes.Tag{Key: lo.ToPtr(k), Value: lo.ToPtr(tags[k])})
	}
	return stack
}

func newTestStack(cfn *fakeCloudFormation, f *fakeIAM, e *fakeEC2, live *LiveClusters) *Stack {
	s := NewStack(cfn, f, e, live)
	s.now = func() time.Time { return testNow }
	s.rotate = func(int) int { return 0 }
	s.waitStartDelay, s.waitMinDelay = 0, time.Millisecond
	return s
}

func TestSelectExpiredStacks(t *testing.T) {
	expiration := testNow.Add(-12 * time.Hour)
	old, mid, young := testNow.Add(-72*time.Hour), testNow.Add(-24*time.Hour), testNow.Add(-time.Hour)
	tagged := map[string]string{"testing/cluster": "c"}
	for _, tc := range []struct {
		stack      *cloudformationtypes.Stack
		expiration time.Time
		excluded   []string
		want       bool
	}{
		{stack: testStack("iam-c", cloudformationtypes.StackStatusCreateComplete, old, tagged), want: true},
		{stack: testStack("eksctl-c-cluster", cloudformationtypes.StackStatusDeleteFailed, old, map[string]string{"github.com/run-url": "x"}), want: true},
		{stack: testStack("iam-c", cloudformationtypes.StackStatusCreateComplete, young, tagged)},
		{stack: testStack("iam-c", cloudformationtypes.StackStatusDeleteInProgress, old, tagged)},
		{stack: testStack("iam-c", cloudformationtypes.StackStatusDeleteComplete, old, tagged)},
		{stack: testStack("iam-c", cloudformationtypes.StackStatusCreateComplete, old, tagged), excluded: []string{"c"}},
		{stack: testStack("eksctl-c-cluster", cloudformationtypes.StackStatusDeleteFailed, old, map[string]string{"github.com/run-url": "x", "alpha.eksctl.io/cluster-name": "c"}), excluded: []string{"c"}},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusReviewInProgress, old, nil), want: true},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusReviewInProgress, mid, nil)},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusReviewInProgress, old, nil), expiration: testNow.Add(-192 * time.Hour)},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusReviewInProgress, old, nil), excluded: []string{"termination-1"}},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusCreateComplete, old, nil)},
		{stack: testStack("iam-termination-1", cloudformationtypes.StackStatusDeleteFailed, old, nil)},
		{stack: testStack("eksctl-c-cluster", cloudformationtypes.StackStatusReviewInProgress, old, nil)},
		{stack: testStack("my-iam-stack", cloudformationtypes.StackStatusReviewInProgress, old, nil)},
	} {
		exp := lo.Ternary(tc.expiration.IsZero(), expiration, tc.expiration)
		got := len(selectExpiredStacks([]cloudformationtypes.Stack{*tc.stack}, exp, testNow, tc.excluded)) == 1
		if got != tc.want {
			t.Errorf("selectExpiredStacks(%s %s created %s, excluded=%v) = %v, want %v", lo.FromPtr(tc.stack.StackName), tc.stack.StackStatus, tc.stack.CreationTime, tc.excluded, got, tc.want)
		}
	}
}

func TestStackCleanupUnblocksDeleteFailedStacks(t *testing.T) {
	old := testNow.Add(-72 * time.Hour)
	deleteFailed := func(name, cluster string, lastDelete time.Time) *cloudformationtypes.Stack {
		s := testStack(name, cloudformationtypes.StackStatusDeleteFailed, old, map[string]string{"testing/cluster": cluster})
		s.DeletionTime = lo.ToPtr(lastDelete)
		return s
	}
	failedRole := func(role string) []cloudformationtypes.StackResource {
		return []cloudformationtypes.StackResource{{
			ResourceType: lo.ToPtr("AWS::IAM::Role"), LogicalResourceId: lo.ToPtr("KarpenterNodeRole"), PhysicalResourceId: lo.ToPtr(role),
			ResourceStatus: cloudformationtypes.ResourceStatusDeleteFailed, ResourceStatusReason: lo.ToPtr("Cannot delete entity, must remove roles from instance profile first"),
		}}
	}
	cfn := &fakeCloudFormation{
		stacks: map[string]*cloudformationtypes.Stack{
			"iam-gone-1":              deleteFailed("iam-gone-1", "gone-1", old),
			"eksctl-gone-2-cluster":   deleteFailed("eksctl-gone-2-cluster", "gone-2", old),
			"iam-live-1":              deleteFailed("iam-live-1", "live-1", old),
			"iam-recent-1":            deleteFailed("iam-recent-1", "recent-1", testNow.Add(-time.Hour)),
			"iam-foreign-1":           deleteFailed("iam-foreign-1", "foreign-1", old),
			"eksctl-gone-3-nodegroup": deleteFailed("eksctl-gone-3-nodegroup", "gone-3", old),
			"eksctl-expired-cluster": testStack("eksctl-expired-cluster", cloudformationtypes.StackStatusCreateComplete, old,
				map[string]string{"testing/cluster": "expired"}),
		},
		resources: map[string][]cloudformationtypes.StackResource{
			"iam-gone-1":    failedRole("KarpenterNodeRole-gone-1"),
			"iam-foreign-1": failedRole("KarpenterNodeRole-foreign-1"),
			// Instance profiles don't hold this role, and the CI role can't list its profiles
			"eksctl-gone-3-nodegroup": failedRole("eksctl-gone-3-nodegroup-NodeInstanceRole-0A1B2C3D"),
			"eksctl-gone-2-cluster": {
				{ResourceType: lo.ToPtr("AWS::EC2::VPC"), LogicalResourceId: lo.ToPtr("VPC"), PhysicalResourceId: lo.ToPtr("vpc-2"), ResourceStatus: cloudformationtypes.ResourceStatusCreateComplete},
				{ResourceType: lo.ToPtr("AWS::EC2::SecurityGroup"), LogicalResourceId: lo.ToPtr("ControlPlaneSecurityGroup"), PhysicalResourceId: lo.ToPtr("sg-2"),
					ResourceStatus: cloudformationtypes.ResourceStatusDeleteFailed, ResourceStatusReason: lo.ToPtr("resource sg-2 has a dependent object")},
			},
		},
	}
	f := newFakeIAM()
	for _, c := range []string{"gone-1", "live-1", "recent-1", "foreign-1"} {
		f.roles["KarpenterNodeRole-"+c] = &fakeRole{}
		f.profiles[c+"_2756385430149416605"] = &fakeProfile{roles: []string{"KarpenterNodeRole-" + c}}
	}
	f.profiles["KarpenterNodeInstanceProfile-Drift-gone-1"] = &fakeProfile{roles: []string{"KarpenterNodeRole-gone-1"}}
	f.profiles["not-a-test-profile"] = &fakeProfile{roles: []string{"KarpenterNodeRole-foreign-1"}}
	e := &fakeEC2{enis: []ec2types.NetworkInterface{
		branchENI("eni-gone-2", "vpc-2", "gone-2", ec2types.NetworkInterfaceStatusAvailable),
		branchENI("eni-other-cluster", "vpc-2", "live-1", ec2types.NetworkInterfaceStatusAvailable),
		branchENI("eni-other-vpc", "vpc-3", "gone-3", ec2types.NetworkInterfaceStatusAvailable),
	}}
	cfn.deleteSucceeds = func(name string) bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		e.mu.Lock()
		defer e.mu.Unlock()
		switch {
		case strings.HasPrefix(name, "iam-"):
			role := "KarpenterNodeRole-" + strings.TrimPrefix(name, "iam-")
			return !lo.ContainsBy(lo.Values(f.profiles), func(p *fakeProfile) bool { return slices.Contains(p.roles, role) })
		case name == "eksctl-gone-2-cluster":
			return !lo.ContainsBy(e.enis, func(ni ec2types.NetworkInterface) bool { return lo.FromPtr(ni.NetworkInterfaceId) == "eni-gone-2" })
		}
		return true
	}

	s := newTestStack(cfn, f, e, liveSnapshot("live-1"))
	names, err := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := s.Cleanup(context.Background(), names)
	slices.Sort(deleted)
	if want := []string{"eksctl-expired-cluster", "eksctl-gone-2-cluster", "eksctl-gone-3-nodegroup", "iam-gone-1"}; !slices.Equal(deleted, want) {
		t.Errorf("Cleanup() deleted %v, want %v", deleted, want)
	}
	var stuck *StuckStacksError
	if !errors.As(err, &stuck) || len(stuck.Skipped) != 3 || len(stuck.Failed) != 0 || Fatal(err) {
		t.Fatalf("Cleanup() error = %v, want 3 skipped stacks that don't fail the sweep", err)
	}
	for _, want := range []string{"iam-foreign-1 (blocked by [AWS::IAM::Role KarpenterNodeRole: Cannot delete entity", "iam-live-1 (cluster live-1 still exists)", "iam-recent-1 (last delete attempt at"} {
		if !lo.ContainsBy(stuck.Skipped, func(s string) bool { return strings.HasPrefix(s, want) }) {
			t.Errorf("skipped stacks %v don't explain %q", stuck.Skipped, want)
		}
	}
	slices.Sort(cfn.deletes)
	if want := []string{"eksctl-expired-cluster", "eksctl-gone-2-cluster", "eksctl-gone-3-nodegroup", "iam-gone-1"}; !slices.Equal(cfn.deletes, want) {
		t.Errorf("DeleteStack calls = %v, want %v", cfn.deletes, want)
	}
	if want := []string{"DeleteNetworkInterface eni-gone-2"}; !slices.Equal(e.calls, want) {
		t.Errorf("EC2 calls = %v, want %v", e.calls, want)
	}
	for _, p := range []string{"live-1_2756385430149416605", "recent-1_2756385430149416605", "foreign-1_2756385430149416605", "not-a-test-profile"} {
		if _, ok := f.profiles[p]; !ok {
			t.Errorf("instance profile %s was deleted", p)
		}
	}
}

func TestStackCleanupIsBounded(t *testing.T) {
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}}
	for i := range 60 {
		name := fmt.Sprintf("eksctl-c%02d-cluster", i)
		// Higher numbers are older, so name order and age order disagree
		cfn.stacks[name] = testStack(name, cloudformationtypes.StackStatusCreateComplete, testNow.Add(-time.Duration(40+i)*time.Hour), map[string]string{"testing/cluster": fmt.Sprintf("c%02d", i)})
	}
	for i := range 12 {
		name := fmt.Sprintf("iam-f%02d", i)
		cfn.stacks[name] = testStack(name, cloudformationtypes.StackStatusDeleteFailed, testNow.Add(-time.Duration(200-i)*time.Hour), map[string]string{"testing/cluster": fmt.Sprintf("f%02d", i)})
	}
	s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, liveSnapshot())
	names, err := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	if err != nil || len(names) != 72 {
		t.Fatalf("GetExpired() = %d stacks, %v", len(names), err)
	}
	deleted, err := s.Cleanup(context.Background(), names)
	if len(cfn.deletes) != maxStackDeletesPerRun || len(deleted) != maxStackDeletesPerRun {
		t.Errorf("got %d DeleteStack calls and %d deleted, want %d", len(cfn.deletes), len(deleted), maxStackDeletesPerRun)
	}
	var stuck *StuckStacksError
	if !errors.As(err, &stuck) || len(stuck.Skipped) != 2 || !strings.Contains(stuck.Skipped[0], "per-run limits") || Fatal(err) {
		t.Errorf("Cleanup() error = %v, want 2 deferred stacks", err)
	}
	if lo.CountBy(cfn.deletes, func(n string) bool { return strings.HasPrefix(n, "iam-") }) != maxDeleteFailedRedrives {
		t.Errorf("DeleteStack calls = %v, want %d DELETE_FAILED retries", cfn.deletes, maxDeleteFailedRedrives)
	}
	if slices.Contains(cfn.deletes, "eksctl-c00-cluster") || !slices.Contains(cfn.deletes, "eksctl-c59-cluster") {
		t.Errorf("DeleteStack calls = %v, want the oldest stacks first", cfn.deletes)
	}
}

func TestStackCleanupFailsClosed(t *testing.T) {
	newCFN := func() *fakeCloudFormation {
		return &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{
			"iam-gone-1": testStack("iam-gone-1", cloudformationtypes.StackStatusDeleteFailed, testNow.Add(-72*time.Hour), map[string]string{"testing/cluster": "gone-1"}),
			"iam-other":  testStack("iam-other", cloudformationtypes.StackStatusDeleteFailed, testNow.Add(-72*time.Hour), map[string]string{"github.com/run-url": "x"}),
		}}
	}
	for _, live := range []*LiveClusters{nil, failedSnapshot()} {
		cfn := newCFN()
		s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, live)
		names, _ := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
		deleted, err := s.Cleanup(context.Background(), names)
		var stuck *StuckStacksError
		if len(deleted) != 0 || len(cfn.deletes) != 0 || !errors.As(err, &stuck) || len(stuck.Skipped) != 2 {
			t.Errorf("Cleanup() = %v, %v with DeleteStack calls %v, want nothing deleted and 2 stuck stacks", deleted, err, cfn.deletes)
		}
	}
}

func TestStackClusterCleanupDoesNotBackOff(t *testing.T) {
	stack := testStack("iam-gone-1", cloudformationtypes.StackStatusDeleteFailed, testNow.Add(-2*time.Hour), map[string]string{"testing/cluster": "gone-1"})
	stack.DeletionTime = lo.ToPtr(testNow.Add(-time.Minute))
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{"iam-gone-1": stack}}
	s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, liveSnapshot())
	names, err := s.Get(context.Background(), "gone-1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.Cleanup(context.Background(), names); err != nil || !slices.Equal(deleted, []string{"iam-gone-1"}) {
		t.Errorf("Cleanup() = %v, %v, want the stack retried right away", deleted, err)
	}
}

// deleteFailedStacks adds DELETE_FAILED iam-<prefix>NN stacks of gone clusters, oldest first
func deleteFailedStacks(cfn *fakeCloudFormation, prefix string, n int, oldest time.Duration) []string {
	var names []string
	for i := range n {
		name := fmt.Sprintf("iam-%s%02d", prefix, i)
		cfn.stacks[name] = testStack(name, cloudformationtypes.StackStatusDeleteFailed, testNow.Add(-oldest+time.Duration(i)*time.Hour), map[string]string{"testing/cluster": strings.TrimPrefix(name, "iam-")})
		names = append(names, name)
	}
	return names
}

func TestStackRedriveThatDoesNotComplete(t *testing.T) {
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}, deleteSucceeds: func(string) bool { return false }}
	deleteFailedStacks(cfn, "stubborn", 1, 200*time.Hour)
	s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, liveSnapshot())
	names, _ := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	deleted, err := s.Cleanup(context.Background(), names)
	var stuck *StuckStacksError
	if len(deleted) != 0 || !errors.As(err, &stuck) || len(stuck.Failed) != 1 || !strings.Contains(stuck.Failed[0], "retried delete didn't complete") || !Fatal(err) {
		t.Errorf("Cleanup() = %v, %v, want the retried stack reported as failed", deleted, err)
	}
}

// heldStacks adds DELETE_FAILED stacks whose node role is also held by an instance profile the sweeper doesn't own
func heldStacks(cfn *fakeCloudFormation, f *fakeIAM, n int) []string {
	names := deleteFailedStacks(cfn, "held", n, 200*time.Hour)
	for _, name := range names {
		role := nodeRolePrefix + strings.TrimPrefix(name, "iam-")
		f.roles[role] = &fakeRole{}
		f.profiles["someone-elses-profile-"+name] = &fakeProfile{roles: []string{role}}
		cfn.resources[name] = []cloudformationtypes.StackResource{{ResourceType: lo.ToPtr("AWS::IAM::Role"), LogicalResourceId: lo.ToPtr("KarpenterNodeRole"),
			PhysicalResourceId: lo.ToPtr(role), ResourceStatus: cloudformationtypes.ResourceStatusDeleteFailed}}
	}
	return names
}

func TestStackRedriveSkipsUnblockableStacks(t *testing.T) {
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}, resources: map[string][]cloudformationtypes.StackResource{}}
	f := newFakeIAM()
	// The held stacks are older, so they come first, but they must not take a slot
	unblockable := heldStacks(cfn, f, maxDeleteFailedRedrives)
	ready := deleteFailedStacks(cfn, "ready", 5, 100*time.Hour)
	s := newTestStack(cfn, f, &fakeEC2{}, liveSnapshot())
	names, _ := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	deleted, err := s.Cleanup(context.Background(), names)
	slices.Sort(deleted)
	if !slices.Equal(deleted, ready) || lo.Some(cfn.deletes, unblockable) {
		t.Errorf("Cleanup() deleted %v with DeleteStack calls %v, want %v", deleted, cfn.deletes, ready)
	}
	if len(f.calls) != 0 || Fatal(err) {
		t.Errorf("Cleanup() error = %v with IAM calls %v, want the held stacks skipped untouched", err, f.calls)
	}
}

func TestStackRedriveChecksAreBounded(t *testing.T) {
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}, resources: map[string][]cloudformationtypes.StackResource{}}
	f := newFakeIAM()
	heldStacks(cfn, f, maxDeleteFailedChecks+5)
	s := newTestStack(cfn, f, &fakeEC2{}, liveSnapshot())
	names, _ := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	_, err := s.Cleanup(context.Background(), names)
	var stuck *StuckStacksError
	if cfn.describes != maxDeleteFailedChecks || !errors.As(err, &stuck) || lo.CountBy(stuck.Skipped, func(s string) bool { return strings.Contains(s, "per-run limits") }) != 5 {
		t.Errorf("checked %d stacks, Cleanup() error = %v, want %d checked and 5 deferred", cfn.describes, err, maxDeleteFailedChecks)
	}
}

func TestStackRedrivesRotateAcrossRuns(t *testing.T) {
	// The stacks stay in DELETE_FAILED and their DeletionTime doesn't move, so backing off alone wouldn't free a slot
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}, deleteSucceeds: func(string) bool { return false }}
	all := deleteFailedStacks(cfn, "stubborn", maxDeleteFailedRedrives+5, 200*time.Hour)
	var runs [][]string
	for run, offset := range []int{0, maxDeleteFailedRedrives} {
		cfn.deletes = nil
		s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, liveSnapshot())
		s.now = func() time.Time { return testNow.Add(time.Duration(run) * 12 * time.Hour) }
		s.rotate = func(n int) int { return offset % n }
		names, _ := s.GetExpired(context.Background(), s.now().Add(-12*time.Hour), nil)
		_, _ = s.Cleanup(context.Background(), names)
		runs = append(runs, slices.Sorted(slices.Values(cfn.deletes)))
	}
	if len(runs[0]) != maxDeleteFailedRedrives || len(runs[1]) != maxDeleteFailedRedrives {
		t.Fatalf("redrives per run = %v, want %d each", runs, maxDeleteFailedRedrives)
	}
	if neverTried := lo.Without(all, runs[0]...); !lo.Every(runs[1], neverTried) {
		t.Errorf("second run retried %v, want it to include the stacks the first run left out, %v", runs[1], neverTried)
	}
}

func TestStackRedrivesInFlightBound(t *testing.T) {
	cfn := &fakeCloudFormation{stacks: map[string]*cloudformationtypes.Stack{}, deleteDelay: 50 * time.Millisecond}
	deleteFailedStacks(cfn, "gone", maxDeleteFailedRedrives, 200*time.Hour)
	s := newTestStack(cfn, newFakeIAM(), &fakeEC2{}, liveSnapshot())
	names, _ := s.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	if deleted, err := s.Cleanup(context.Background(), names); err != nil || len(deleted) != maxDeleteFailedRedrives {
		t.Fatalf("Cleanup() = %v, %v", deleted, err)
	}
	if cfn.maxInFlight > deleteFailedRedrivesInFlight || cfn.maxInFlight < 2 {
		t.Errorf("%d redrives in flight at once, want between 2 and %d", cfn.maxInFlight, deleteFailedRedrivesInFlight)
	}
}

func TestFatal(t *testing.T) {
	skipped := &SkippedError{Reason: "held"}
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{err: nil},
		{err: &StuckStacksError{Skipped: []string{"a (backed off)"}}},
		{err: fmt.Errorf("deleting role a, %w", skipped)},
		{err: multierr.Combine(fmt.Errorf("deleting role a, %w", skipped), &StuckStacksError{Skipped: []string{"b"}})},
		{err: &StuckStacksError{Failed: []string{"a (retried delete didn't complete)"}, Skipped: []string{"b"}}, want: true},
		{err: errors.New("Throttling"), want: true},
		{err: multierr.Combine(skipped, errors.New("AccessDenied")), want: true},
	} {
		if got := Fatal(tc.err); got != tc.want {
			t.Errorf("Fatal(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
