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

package binpacking_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/provisioning/binpacking"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func BenchmarkPacker(b *testing.B) {
	// Setup Mocks
	ctx := context.Background()
	instanceTypes := fake.InstanceTypes(100)
	instanceTypeNames := []string{}
	for _, it := range instanceTypes {
		instanceTypeNames = append(instanceTypeNames, it.Name())
	}

	kubeClient := testclient.NewClientBuilder().WithLists(&appsv1.DaemonSetList{}).Build()
	fakeCloud := fake.CloudProvider{InstanceTypes: instanceTypes}
	packer := binpacking.NewPacker(kubeClient, &fakeCloud)

	pods := test.Pods(10_000, test.PodOptions{
		ResourceRequirements: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", 1)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", 512)),
			},
		},
	})

	schedule := &scheduling.Schedule{
		Constraints: &v1alpha5.Constraints{
			Requirements: v1alpha5.NewRequirements([]v1.NodeSelectorRequirement{
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}},
				{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: instanceTypeNames},
				{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.ArchitectureAmd64, v1alpha5.ArchitectureArm64}},
				{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{"spot", "on-demand"}},
				{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{"linux"}},
			}...),
		},
		Pods: pods,
	}

	// Pack benchmark
	for i := 0; i < b.N; i++ {
		if packings, err := packer.Pack(ctx, schedule.Constraints, pods, instanceTypes); err != nil || len(packings) == 0 {
			b.FailNow()
		}
	}
}
