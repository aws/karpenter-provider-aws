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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

// RepairPolicyConfigMapKey is the ConfigMap data key holding the policy JSON.
const RepairPolicyConfigMapKey = "policies.json"

type repairPolicyEntry struct {
	ConditionType   string `json:"conditionType"`
	ConditionStatus string `json:"conditionStatus"`
	// TolerationDuration is a Go duration string (e.g. "30m").
	TolerationDuration string `json:"tolerationDuration"`
}

// DefaultRepairPolicies is the built-in fallback used when the ConfigMap is
// absent or unparseable.
func DefaultRepairPolicies() []cloudprovider.RepairPolicy {
	return []cloudprovider.RepairPolicy{
		// Supported Kubelet Node Conditions
		{ConditionType: corev1.NodeReady, ConditionStatus: corev1.ConditionFalse, TolerationDuration: 30 * time.Minute},
		{ConditionType: corev1.NodeReady, ConditionStatus: corev1.ConditionUnknown, TolerationDuration: 30 * time.Minute},
		// Supported Node Monitoring Agent Conditions
		{ConditionType: "AcceleratedHardwareReady", ConditionStatus: corev1.ConditionFalse, TolerationDuration: 10 * time.Minute},
		{ConditionType: "StorageReady", ConditionStatus: corev1.ConditionFalse, TolerationDuration: 30 * time.Minute},
		{ConditionType: "NetworkingReady", ConditionStatus: corev1.ConditionFalse, TolerationDuration: 30 * time.Minute},
		{ConditionType: "KernelReady", ConditionStatus: corev1.ConditionFalse, TolerationDuration: 30 * time.Minute},
		{ConditionType: "ContainerRuntimeReady", ConditionStatus: corev1.ConditionFalse, TolerationDuration: 30 * time.Minute},
	}
}

// ParseRepairPolicies parses a ConfigMap's policies.json key into RepairPolicy.
func ParseRepairPolicies(cm *corev1.ConfigMap) ([]cloudprovider.RepairPolicy, error) {
	raw, ok := cm.Data[RepairPolicyConfigMapKey]
	if !ok {
		return nil, fmt.Errorf("configmap missing key %q", RepairPolicyConfigMapKey)
	}
	var entries []repairPolicyEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("parsing repair policies: %w", err)
	}
	policies := make([]cloudprovider.RepairPolicy, 0, len(entries))
	for i, e := range entries {
		d, err := time.ParseDuration(e.TolerationDuration)
		if err != nil {
			return nil, fmt.Errorf("policy[%d] tolerationDuration %q: %w", i, e.TolerationDuration, err)
		}
		policies = append(policies, cloudprovider.RepairPolicy{
			ConditionType:      corev1.NodeConditionType(e.ConditionType),
			ConditionStatus:    corev1.ConditionStatus(e.ConditionStatus),
			TolerationDuration: d,
		})
	}
	return policies, nil
}
