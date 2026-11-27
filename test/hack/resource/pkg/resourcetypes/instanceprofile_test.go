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
	"slices"
	"testing"
	"time"
)

func TestOrphanedKarpenterProfile(t *testing.T) {
	live := liveSnapshot("live-1")
	karpenterTags := func(extra map[string]string) map[string]string {
		tags := map[string]string{"karpenter.k8s.aws/ec2nodeclass": "default", "eks:eks-cluster-name": "gone-1"}
		for k, v := range extra {
			tags[k] = v
		}
		return tags
	}
	for _, tc := range []struct {
		name     string
		tags     map[string]string
		excluded []string
		live     *LiveClusters
		want     bool
	}{
		{name: "gone-1_2756385430149416605", tags: karpenterTags(nil), want: true},
		{name: "gone-1_2756385430149416605", tags: karpenterTags(map[string]string{"topology.kubernetes.io/region": "us-east-2"}), want: true},
		// Another region's sweeper owns profiles tagged for its region
		{name: "gone-1_2756385430149416605", tags: karpenterTags(map[string]string{"topology.kubernetes.io/region": "us-west-2"})},
		{name: "live-1_2756385430149416605", tags: karpenterTags(nil)},
		{name: "gone-1_2756385430149416605", tags: karpenterTags(nil), excluded: []string{"gone-1"}},
		// Without the Karpenter nodeclass tag, it's not a Karpenter-managed profile
		{name: "gone-1_2756385430149416605", tags: map[string]string{"testing/cluster": "gone-1"}},
		{name: "KarpenterNodeInstanceProfile-gone-1", tags: karpenterTags(nil)},
		{name: "gone-1_abc", tags: karpenterTags(nil)},
		{name: "gone-1_2756385430149416605", tags: karpenterTags(nil), live: failedSnapshot()},
	} {
		l := live
		if tc.live != nil {
			l = tc.live
		}
		if got := orphanedKarpenterProfile(tc.name, toIAMTags(tc.tags), "us-east-2", tc.excluded, l); got != tc.want {
			t.Errorf("orphanedKarpenterProfile(%q, %v, excluded=%v) = %v, want %v", tc.name, tc.tags, tc.excluded, got, tc.want)
		}
	}
}

func TestInstanceProfileCleanupRemovesEveryRole(t *testing.T) {
	f := newFakeIAM()
	f.profiles["gone-1_1"] = &fakeProfile{roles: []string{"KarpenterNodeRole-gone-1", "KarpenterNodeRole-gone-1-0a1b2c3d"}}
	f.profiles["gone-1_2"] = &fakeProfile{}
	ip := NewInstanceProfile(f, liveSnapshot(), "us-east-2")
	// gone-1_3 was already deleted, which counts as cleaned
	deleted, err := ip.Cleanup(context.Background(), []string{"gone-1_1", "gone-1_2", "gone-1_3"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gone-1_1", "gone-1_2", "gone-1_3"}; !slices.Equal(deleted, want) {
		t.Errorf("Cleanup() = %v, want %v", deleted, want)
	}
	if len(f.profiles) != 0 {
		t.Errorf("profiles left behind: %v", f.profiles)
	}
}

func TestInstanceProfileGetExpired(t *testing.T) {
	expiration := testNow.Add(-12 * time.Hour)
	tags := map[string]string{"karpenter.k8s.aws/ec2nodeclass": "default"}
	f := newFakeIAM()
	// CreateDate only has day precision, so a profile must be a day older than the expiration
	f.profiles["gone-1_1"] = &fakeProfile{created: expiration.Add(-25 * time.Hour), tags: tags}
	f.profiles["gone-1_2"] = &fakeProfile{created: expiration.Add(-23 * time.Hour), tags: tags}
	f.profiles["gone-1_3"] = &fakeProfile{created: expiration.Add(-25 * time.Hour), tags: tags, vanished: true}
	f.profiles["KarpenterNodeInstanceProfile-c"] = &fakeProfile{created: expiration.Add(-25 * time.Hour), tags: map[string]string{"testing/cluster": "c", "topology.kubernetes.io/region": "us-east-2"}}
	f.profiles["KarpenterNodeInstanceProfile-d"] = &fakeProfile{created: expiration.Add(-25 * time.Hour), tags: map[string]string{"testing/cluster": "d", "topology.kubernetes.io/region": "us-west-2"}}
	names, err := NewInstanceProfile(f, liveSnapshot(), "us-east-2").GetExpired(context.Background(), expiration, nil)
	if want := []string{"KarpenterNodeInstanceProfile-c", "gone-1_1"}; err != nil || !slices.Equal(names, want) {
		t.Errorf("GetExpired() = %v, %v, want %v", names, err, want)
	}
}
