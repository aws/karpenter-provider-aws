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

package amifamily_test

import (
	"testing"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

func TestFilterInstanceTypesByAMICompatibility(t *testing.T) {
	testCases := []struct {
		name          string
		instanceTypes []*cloudprovider.InstanceType
		amis          []v1.AMI
		expected      int
	}{
		{
			name: "amd64 instance type with amd64 ami",
			instanceTypes: []*cloudprovider.InstanceType{
				{
					Name: "m5.large",
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
					),
				},
			},
			amis: []v1.AMI{
				{
					ID: "ami-123",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "arm64 instance type with amd64 ami",
			instanceTypes: []*cloudprovider.InstanceType{
				{
					Name: "m6g.large",
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "arm64"),
					),
				},
			},
			amis: []v1.AMI{
				{
					ID: "ami-123",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "mixed instance types with both amis",
			instanceTypes: []*cloudprovider.InstanceType{
				{
					Name: "m5.large",
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
					),
				},
				{
					Name: "m6g.large",
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "arm64"),
					),
				},
			},
			amis: []v1.AMI{
				{
					ID: "ami-123",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
				{
					ID: "ami-456",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"arm64"},
						},
					},
				},
			},
			expected: 2,
		},
		{
			name:          "empty instance types",
			instanceTypes: []*cloudprovider.InstanceType{},
			amis: []v1.AMI{
				{
					ID: "ami-123",
					Requirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "empty amis",
			instanceTypes: []*cloudprovider.InstanceType{
				{
					Name: "m5.large",
					Requirements: scheduling.NewRequirements(
						scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
					),
				},
			},
			amis:     []v1.AMI{},
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compatible := amifamily.FilterInstanceTypesByAMICompatibility(tc.instanceTypes, tc.amis)
			assert.Len(t, compatible, tc.expected)
		})
	}
}
