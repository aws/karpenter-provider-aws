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

package test

import (
	"fmt"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

func EC2NodeClass(overrides ...v1.EC2NodeClass) *v1.EC2NodeClass {
	options := v1.EC2NodeClass{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}
	if len(options.Spec.AMISelectorTerms) == 0 {
		options.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
		options.Status.AMIs = []v1.AMI{
			{
				ID: "ami-test1",
				Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
					{Key: v1.LabelInstanceGPUCount, Operator: corev1.NodeSelectorOpDoesNotExist},
					{Key: v1.LabelInstanceAcceleratorCount, Operator: corev1.NodeSelectorOpDoesNotExist},
				},
			},
			{
				ID: "ami-test2",
				Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
					{Key: v1.LabelInstanceGPUCount, Operator: corev1.NodeSelectorOpExists},
				},
			},
			{
				ID: "ami-test3",
				Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
					{Key: v1.LabelInstanceAcceleratorCount, Operator: corev1.NodeSelectorOpExists},
				},
			},
			{
				ID: "ami-test4",
				Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureArm64}},
					{Key: v1.LabelInstanceGPUCount, Operator: corev1.NodeSelectorOpDoesNotExist},
					{Key: v1.LabelInstanceAcceleratorCount, Operator: corev1.NodeSelectorOpDoesNotExist},
				},
			},
		}
	}
	if options.Spec.Role == "" {
		options.Spec.Role = "test-role"
		options.Status.InstanceProfile = "test-profile"
	}
	if len(options.Spec.SecurityGroupSelectorTerms) == 0 {
		options.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
		options.Status.SecurityGroups = []v1.SecurityGroup{
			{
				ID: "sg-test1",
			},
			{
				ID: "sg-test2",
			},
			{
				ID: "sg-test3",
			},
		}
	}
	if len(options.Spec.SubnetSelectorTerms) == 0 {
		options.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
		options.Status.Subnets = []v1.Subnet{
			{
				ID:     "subnet-test1",
				Zone:   "test-zone-1a",
				ZoneID: "tstz1-1a",
			},
			{
				ID:     "subnet-test2",
				Zone:   "test-zone-1b",
				ZoneID: "tstz1-1b",
			},
			{
				ID:     "subnet-test3",
				Zone:   "test-zone-1c",
				ZoneID: "tstz1-1c",
			},
		}
	}
	return &v1.EC2NodeClass{
		ObjectMeta: test.ObjectMeta(options.ObjectMeta),
		Spec:       options.Spec,
		Status:     options.Status,
	}
}

type TestNodeClass struct {
	v1.EC2NodeClass
}

func (t *TestNodeClass) InstanceProfileTags(clusterName string) map[string]string {
	return nil
}
