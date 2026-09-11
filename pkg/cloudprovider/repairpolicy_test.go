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

package cloudprovider_test

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
)

func cm(data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{Data: map[string]string{cloudprovider.RepairPolicyConfigMapKey: data}}
}

func TestParseRepairPolicies(t *testing.T) {
	t.Run("parses a valid policy list", func(t *testing.T) {
		policies, err := cloudprovider.ParseRepairPolicies(cm(`[
			{"conditionType":"Ready","conditionStatus":"False","tolerationDuration":"30m"},
			{"conditionType":"NetworkingReady","conditionStatus":"False","tolerationDuration":"5m"}
		]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(policies) != 2 {
			t.Fatalf("expected 2 policies, got %d", len(policies))
		}
		if policies[0].ConditionType != corev1.NodeReady ||
			policies[0].ConditionStatus != corev1.ConditionFalse ||
			policies[0].TolerationDuration != 30*time.Minute {
			t.Fatalf("policy[0] mismatch: %+v", policies[0])
		}
		if policies[1].TolerationDuration != 5*time.Minute {
			t.Fatalf("policy[1] duration mismatch: %+v", policies[1])
		}
	})

	t.Run("errors when the data key is missing", func(t *testing.T) {
		if _, err := cloudprovider.ParseRepairPolicies(&corev1.ConfigMap{}); err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("errors on malformed json", func(t *testing.T) {
		if _, err := cloudprovider.ParseRepairPolicies(cm(`not json`)); err == nil {
			t.Fatal("expected error for malformed json")
		}
	})

	t.Run("errors on unparseable duration", func(t *testing.T) {
		if _, err := cloudprovider.ParseRepairPolicies(cm(`[{"conditionType":"Ready","conditionStatus":"False","tolerationDuration":"soon"}]`)); err == nil {
			t.Fatal("expected error for bad duration")
		}
	})
}

func TestSetRepairPoliciesRoundTrip(t *testing.T) {
	cp := &cloudprovider.CloudProvider{}
	if got := cp.RepairPolicies(); got != nil {
		t.Fatalf("expected nil before seeding, got %+v", got)
	}
	want := cloudprovider.DefaultRepairPolicies()
	cp.SetRepairPolicies(want)
	got := cp.RepairPolicies()
	if len(got) != len(want) {
		t.Fatalf("expected %d policies, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("policy[%d] mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}
