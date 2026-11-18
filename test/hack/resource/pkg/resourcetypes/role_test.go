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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
)

var testNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

func liveSnapshot(names ...string) *LiveClusters {
	return ListLiveClusters(context.Background(), func(_ context.Context, _ string) ([]string, error) { return names, nil }, []string{"us-east-2"})
}

func failedSnapshot() *LiveClusters {
	return ListLiveClusters(context.Background(), func(_ context.Context, _ string) ([]string, error) {
		return nil, errors.New("AccessDenied")
	}, []string{"us-east-2"})
}

func TestLiveClusters(t *testing.T) {
	lister := func(_ context.Context, region string) ([]string, error) {
		return map[string][]string{"us-east-2": {"a"}, "eu-north-1": {"soak"}}[region], nil
	}
	live := ListLiveClusters(context.Background(), lister, []string{"us-east-2", "eu-north-1"})
	for name, want := range map[string]bool{"a": false, "soak": false, "gone": true, "": false} {
		if got := live.Gone(name); got != want {
			t.Errorf("Gone(%q) = %v, want %v", name, got, want)
		}
	}
	// One failing region poisons the whole snapshot, so nothing is reported as gone
	failing := ListLiveClusters(context.Background(), func(ctx context.Context, region string) ([]string, error) {
		if region == "eu-north-1" {
			return nil, errors.New("AccessDenied")
		}
		return lister(ctx, region)
	}, []string{"us-east-2", "eu-north-1"})
	var nilSnapshot *LiveClusters
	for _, l := range []*LiveClusters{failing, nilSnapshot, ListLiveClusters(context.Background(), lister, nil)} {
		if l.Err() == nil || l.Gone("gone") {
			t.Errorf("expected a failed snapshot to report no cluster as gone")
		}
	}
}

func TestNodeRoleCluster(t *testing.T) {
	tags := func(kv ...string) []iamtypes.Tag {
		return toIAMTags(lo.FromEntries(lo.Map(lo.Chunk(kv, 2), func(c []string, _ int) lo.Entry[string, string] {
			return lo.Entry[string, string]{Key: c[0], Value: c[1]}
		})))
	}
	for _, tc := range []struct {
		role, want string
		tags       []iamtypes.Tag
		ok         bool
	}{
		{role: "KarpenterNodeRole-termination-255754628", tags: tags("testing/cluster", "termination-255754628"), want: "termination-255754628", ok: true},
		{role: "KarpenterNodeRole-drift-1-0a1b2c3d", tags: tags("karpenter.sh/discovery", "drift-1"), want: "drift-1", ok: true},
		// The tag must name the cluster in the role name, on a "-" boundary
		{role: "KarpenterNodeRole-drift-10", tags: tags("testing/cluster", "drift-1")},
		{role: "KarpenterNodeRole-a", tags: tags("testing/cluster", "b")},
		{role: "KarpenterNodeRole-a"},
		{role: "KarpenterNodeRole-a", tags: tags("testing/cluster", "")},
		{role: "eksctl-a-cluster-ServiceRole", tags: tags("testing/cluster", "a")},
		{role: "MyKarpenterNodeRole-a", tags: tags("testing/cluster", "a")},
	} {
		got, ok := nodeRoleCluster(tc.role, tc.tags)
		if got != tc.want || ok != tc.ok {
			t.Errorf("nodeRoleCluster(%q) = %q, %v, want %q, %v", tc.role, got, ok, tc.want, tc.ok)
		}
	}
}

func TestOwnedInstanceProfile(t *testing.T) {
	for _, tc := range []struct {
		profile, cluster string
		want             bool
	}{
		{"termination-255754628_2756385430149416605", "termination-255754628", true},
		{"KarpenterNodeInstanceProfile-c", "c", true},
		{"KarpenterNodeInstanceProfile-Drift-c", "c", true},
		{"termination-255754628_2756385430149416605", "termination-25575462", false},
		{"other_123", "c", false},
		{"c_", "c", false},
		{"c_12a", "c", false},
		{"_123", "", false},
		{"KarpenterNodeInstanceProfile-c-2", "c", false},
		{"c_123", "", false},
	} {
		if got := ownedInstanceProfile(tc.profile, tc.cluster); got != tc.want {
			t.Errorf("ownedInstanceProfile(%q, %q) = %v, want %v", tc.profile, tc.cluster, got, tc.want)
		}
	}
}

func TestReleasableProfilesRefusesForeignProfiles(t *testing.T) {
	f := newFakeIAM()
	f.roles["KarpenterNodeRole-c"] = &fakeRole{}
	f.profiles["c_1"] = &fakeProfile{roles: []string{"KarpenterNodeRole-c"}}
	f.profiles["someone-elses-profile"] = &fakeProfile{roles: []string{"KarpenterNodeRole-c"}}
	var skipped *SkippedError
	if _, err := releasableProfiles(context.Background(), f, "KarpenterNodeRole-c", "c"); !errors.As(err, &skipped) {
		t.Fatalf("releasableProfiles() error = %v, want a SkippedError for a profile that is not owned by the cluster", err)
	}
}

func TestRoleSweep(t *testing.T) {
	old, young := testNow.Add(-72*time.Hour), testNow.Add(-time.Hour)
	f := newFakeIAM()
	f.roles["KarpenterNodeRole-gone-1"] = &fakeRole{created: old, tags: map[string]string{"testing/cluster": "gone-1"}, inline: []string{"inline"}, attached: []string{"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"}}
	f.roles["KarpenterNodeRole-gone-1-0a1b2c3d"] = &fakeRole{created: old, tags: map[string]string{"karpenter.sh/discovery": "gone-1"}}
	f.roles["KarpenterNodeRole-live-1"] = &fakeRole{created: old, tags: map[string]string{"testing/cluster": "live-1"}}
	f.roles["KarpenterNodeRole-excluded-1"] = &fakeRole{created: old, tags: map[string]string{"testing/cluster": "excluded-1"}}
	f.roles["KarpenterNodeRole-young-1"] = &fakeRole{created: young, tags: map[string]string{"testing/cluster": "young-1"}}
	f.roles["KarpenterNodeRole-untagged"] = &fakeRole{created: old}
	f.roles["KarpenterNodeRole-vanished-1"] = &fakeRole{created: old, tags: map[string]string{"testing/cluster": "vanished-1"}, vanished: true}
	f.roles["eksctl-gone-1-cluster-ServiceRole"] = &fakeRole{created: old, tags: map[string]string{"testing/cluster": "gone-1"}}
	f.profiles["gone-1_2756385430149416605"] = &fakeProfile{roles: []string{"KarpenterNodeRole-gone-1"}}
	f.profiles["KarpenterNodeInstanceProfile-Drift-gone-1"] = &fakeProfile{roles: []string{"KarpenterNodeRole-gone-1"}}

	r := NewRole(f, liveSnapshot("live-1"))
	names, err := r.GetExpired(context.Background(), testNow.Add(-12*time.Hour), []string{"excluded-1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"KarpenterNodeRole-gone-1", "KarpenterNodeRole-gone-1-0a1b2c3d"}; !slices.Equal(names, want) {
		t.Fatalf("GetExpired() = %v, want %v", names, want)
	}
	deleted, err := r.Cleanup(context.Background(), names)
	if err != nil || !slices.Equal(deleted, names) {
		t.Fatalf("Cleanup() = %v, %v", deleted, err)
	}
	want := []string{
		"RemoveRoleFromInstanceProfile KarpenterNodeInstanceProfile-Drift-gone-1 KarpenterNodeRole-gone-1",
		"DeleteInstanceProfile KarpenterNodeInstanceProfile-Drift-gone-1",
		"RemoveRoleFromInstanceProfile gone-1_2756385430149416605 KarpenterNodeRole-gone-1",
		"DeleteInstanceProfile gone-1_2756385430149416605",
		"DeleteRolePolicy KarpenterNodeRole-gone-1 inline",
		"DetachRolePolicy KarpenterNodeRole-gone-1 arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"DeleteRole KarpenterNodeRole-gone-1",
		"DeleteRole KarpenterNodeRole-gone-1-0a1b2c3d",
	}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v\nwant %v", f.calls, want)
	}
	if _, ok := f.roles["KarpenterNodeRole-live-1"]; !ok {
		t.Error("the live cluster's role was deleted")
	}
	if names, err := r.GetExpired(context.Background(), testNow.Add(-12*time.Hour), []string{"excluded-1"}); err != nil || len(names) != 0 {
		t.Errorf("second GetExpired() = %v, %v", names, err)
	}
}

func TestRoleSweepFailsClosed(t *testing.T) {
	f := newFakeIAM()
	f.roles["KarpenterNodeRole-gone-1"] = &fakeRole{created: testNow.Add(-72 * time.Hour), tags: map[string]string{"testing/cluster": "gone-1"}}
	for _, live := range []*LiveClusters{nil, failedSnapshot()} {
		r := NewRole(f, live)
		if names, err := r.GetExpired(context.Background(), testNow, nil); err == nil || len(names) != 0 {
			t.Errorf("GetExpired() = %v, %v, want an error and no roles", names, err)
		}
		// Cleanup re-checks the cluster even when handed a name directly
		if deleted, err := r.Cleanup(context.Background(), []string{"KarpenterNodeRole-gone-1"}); err == nil || len(deleted) != 0 {
			t.Errorf("Cleanup() = %v, %v, want an error and nothing deleted", deleted, err)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("expected no mutating calls, got %v", f.calls)
	}
}

func TestRoleDeleteRetriesConflicts(t *testing.T) {
	for _, tc := range []struct {
		conflicts int
		deleted   bool
	}{{conflicts: roleDeleteAttempts - 1, deleted: true}, {conflicts: roleDeleteAttempts}} {
		f := newFakeIAM()
		f.roles["KarpenterNodeRole-gone-1"] = &fakeRole{created: testNow.Add(-72 * time.Hour), tags: map[string]string{"testing/cluster": "gone-1"}}
		f.deleteConflicts["KarpenterNodeRole-gone-1"] = tc.conflicts
		r := NewRole(f, liveSnapshot())
		r.deleteConflictDelay = 0
		names, err := r.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
		if err != nil {
			t.Fatal(err)
		}
		deleted, err := r.Cleanup(context.Background(), names)
		if got := len(deleted) == 1 && err == nil; got != tc.deleted {
			t.Errorf("with %d conflicts, Cleanup() = %v, %v, want deleted = %v", tc.conflicts, deleted, err, tc.deleted)
		}
	}
}

func TestRoleCleanupIsBounded(t *testing.T) {
	f := newFakeIAM()
	// Roles held by a foreign profile are refused before any change, so they don't use up the limit
	for i := range 5 {
		c := fmt.Sprintf("a-held-%d", i)
		f.roles[nodeRolePrefix+c] = &fakeRole{created: testNow.Add(-72 * time.Hour), tags: map[string]string{"testing/cluster": c}}
		f.profiles["someone-elses-profile-"+c] = &fakeProfile{roles: []string{nodeRolePrefix + c}}
	}
	for i := range 60 {
		c := fmt.Sprintf("b-gone-%02d", i)
		f.roles[nodeRolePrefix+c] = &fakeRole{created: testNow.Add(-72 * time.Hour), tags: map[string]string{"testing/cluster": c}}
	}
	r := NewRole(f, liveSnapshot())
	names, err := r.GetExpired(context.Background(), testNow.Add(-12*time.Hour), nil)
	if err != nil || len(names) != 65 {
		t.Fatalf("GetExpired() = %d roles, %v", len(names), err)
	}
	deleted, err := r.Cleanup(context.Background(), names)
	if len(deleted) != maxRoleDeletesPerRun || lo.CountBy(f.calls, func(c string) bool { return strings.HasPrefix(c, "DeleteRole ") }) != maxRoleDeletesPerRun {
		t.Errorf("Cleanup() deleted %d roles with calls %v, want %d", len(deleted), f.calls, maxRoleDeletesPerRun)
	}
	var skipped *SkippedError
	if !errors.As(err, &skipped) {
		t.Errorf("Cleanup() error = %v, want the held roles reported as skipped", err)
	}
}

type fakeRegions struct {
	regions []string
	err     error
}

func (f fakeRegions) DescribeRegions(_ context.Context, _ *ec2.DescribeRegionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return &ec2.DescribeRegionsOutput{Regions: lo.Map(f.regions, func(r string, _ int) ec2types.Region { return ec2types.Region{RegionName: lo.ToPtr(r)} })}, f.err
}

func TestSnapshotLiveClusters(t *testing.T) {
	clusters := map[string][]string{"us-east-2": {"a"}, "eu-north-1": {"soak"}, "ap-south-1": {"elsewhere"}}
	var listed []string
	lister := func(_ context.Context, region string) ([]string, error) {
		listed = append(listed, region)
		return clusters[region], nil
	}
	// The current region is listed even when DescribeRegions doesn't return it
	live := SnapshotLiveClusters(context.Background(), fakeRegions{regions: []string{"us-east-2", "eu-north-1"}}, lister, "ap-south-1")
	if want := []string{"us-east-2", "eu-north-1", "ap-south-1"}; !slices.Equal(listed, want) {
		t.Errorf("listed regions %v, want %v", listed, want)
	}
	for name, want := range map[string]bool{"a": false, "soak": false, "elsewhere": false, "gone": true} {
		if got := live.Gone(name); got != want {
			t.Errorf("Gone(%q) = %v, want %v", name, got, want)
		}
	}
	if failed := SnapshotLiveClusters(context.Background(), fakeRegions{err: errors.New("AccessDenied")}, lister, "us-east-2"); failed.Err() == nil || failed.Gone("gone") {
		t.Errorf("expected a failed DescribeRegions to report no cluster as gone")
	}
}
