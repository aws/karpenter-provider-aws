/*
Copyright The Kubernetes Authors.

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

package v1

import (
	"github.com/awslabs/operatorpkg/status"
	v1 "k8s.io/api/core/v1"
)

const (
	// ConditionTypeValidationSucceeded = "ValidationSucceeded" condition indicates that the
	// runtime-based configuration is valid for this NodePool
	ConditionTypeValidationSucceeded = "ValidationSucceeded"
	// ConditionTypeNodeClassReady = "NodeClassReady" condition indicates that underlying nodeClass was resolved and is reporting as Ready
	ConditionTypeNodeClassReady = "NodeClassReady"
)

// NodePoolStatus defines the observed state of NodePool
type NodePoolStatus struct {
	// Resources is the list of resources that have been provisioned.
	// +optional
	Resources v1.ResourceList `json:"resources,omitempty"`
	// Conditions contains signals for health and readiness
	// +optional
	Conditions []status.Condition `json:"conditions,omitempty"`
}

func (in *NodePool) StatusConditions() status.ConditionSet {
	return status.NewReadyConditions(
		ConditionTypeValidationSucceeded,
		ConditionTypeNodeClassReady,
	).For(in)
}

func (in *NodePool) GetConditions() []status.Condition {
	return in.Status.Conditions
}

func (in *NodePool) SetConditions(conditions []status.Condition) {
	in.Status.Conditions = conditions
}
