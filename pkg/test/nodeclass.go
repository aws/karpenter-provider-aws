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
	"context"
	"fmt"

	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

func EC2NodeClass(overrides ...v1beta1.EC2NodeClass) *v1beta1.EC2NodeClass {
	options := v1beta1.EC2NodeClass{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}
	if options.Spec.AMIFamily == nil {
		options.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
		options.Status.AMIs = []v1beta1.AMI{
			{
				ID: "ami-test1",
				Requirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
					{Key: v1beta1.LabelInstanceGPUCount, Operator: v1.NodeSelectorOpDoesNotExist},
					{Key: v1beta1.LabelInstanceAcceleratorCount, Operator: v1.NodeSelectorOpDoesNotExist},
				},
			},
			{
				ID: "ami-test2",
				Requirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
					{Key: v1beta1.LabelInstanceGPUCount, Operator: v1.NodeSelectorOpExists},
				},
			},
			{
				ID: "ami-test3",
				Requirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureAmd64}},
					{Key: v1beta1.LabelInstanceAcceleratorCount, Operator: v1.NodeSelectorOpExists},
				},
			},
			{
				ID: "ami-test4",
				Requirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.ArchitectureArm64}},
					{Key: v1beta1.LabelInstanceGPUCount, Operator: v1.NodeSelectorOpDoesNotExist},
					{Key: v1beta1.LabelInstanceAcceleratorCount, Operator: v1.NodeSelectorOpDoesNotExist},
				},
			},
		}
	}
	if options.Spec.Role == "" {
		options.Spec.Role = "test-role"
		options.Status.InstanceProfile = "test-profile"
	}
	if len(options.Spec.SecurityGroupSelectorTerms) == 0 {
		options.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
		options.Status.SecurityGroups = []v1beta1.SecurityGroup{
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
		options.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{
					"*": "*",
				},
			},
		}
		options.Status.Subnets = []v1beta1.Subnet{
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
	return &v1beta1.EC2NodeClass{
		ObjectMeta: test.ObjectMeta(options.ObjectMeta),
		Spec:       options.Spec,
		Status:     options.Status,
	}
}

func EC2NodeClassFieldIndexer(ctx context.Context) func(cache.Cache) error {
	return func(c cache.Cache) error {
		return c.IndexField(ctx, &corev1beta1.NodeClaim{}, "spec.nodeClassRef.name", func(obj client.Object) []string {
			nc := obj.(*corev1beta1.NodeClaim)
			if nc.Spec.NodeClassRef == nil {
				return []string{""}
			}
			return []string{nc.Spec.NodeClassRef.Name}
		})
	}
}
