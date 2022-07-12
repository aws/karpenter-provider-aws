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

package fake

import (
	"context"
	"fmt"
	"sync"

	utilsets "k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/pkg/apis"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CloudProvider struct {
	InstanceTypes []cloudprovider.InstanceType

	// CreateCalls contains the arguments for every create call that was made since it was cleared
	mu          sync.Mutex
	CreateCalls []*cloudprovider.NodeRequest
}

var _ cloudprovider.CloudProvider = (*CloudProvider)(nil)
var _ cloudprovider.InstanceType = (*InstanceType)(nil)

func (c *CloudProvider) Create(ctx context.Context, nodeRequest *cloudprovider.NodeRequest) (*v1.Node, error) {
	c.mu.Lock()
	c.CreateCalls = append(c.CreateCalls, nodeRequest)
	c.mu.Unlock()
	name := test.RandomName()
	instanceType := nodeRequest.InstanceTypeOptions[0]
	// Labels
	labels := map[string]string{}
	for key, values := range instanceType.Requirements() {
		if values.Len() == 1 {
			labels[key] = values.Any()
		}
	}
	// Find Offering
	for _, o := range instanceType.Offerings() {
		if nodeRequest.Template.Requirements.Compatible(scheduling.Requirements{
			v1.LabelTopologyZone:       sets.NewSet(o.Zone),
			v1alpha5.LabelCapacityType: sets.NewSet(o.CapacityType),
		}) == nil {
			labels[v1.LabelTopologyZone] = o.Zone
			labels[v1alpha5.LabelCapacityType] = o.CapacityType
			break
		}
	}
	n := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("fake://%s", name),
		},
	}
	return n, nil
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, provisioner *v1alpha5.Provisioner) ([]cloudprovider.InstanceType, error) {
	if c.InstanceTypes != nil {
		return c.InstanceTypes, nil
	}
	return []cloudprovider.InstanceType{
		NewInstanceType(InstanceTypeOptions{
			Name: "default-instance-type",
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "pod-eni-instance-type",
			Resources: map[v1.ResourceName]resource.Quantity{
				v1alpha1.ResourceAWSPodENI: resource.MustParse("1"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "small-instance-type",
			Resources: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "nvidia-gpu-instance-type",
			Resources: map[v1.ResourceName]resource.Quantity{
				v1alpha1.ResourceNVIDIAGPU: resource.MustParse("2"),
			}}),
		NewInstanceType(InstanceTypeOptions{
			Name: "amd-gpu-instance-type",
			Resources: map[v1.ResourceName]resource.Quantity{
				v1alpha1.ResourceAMDGPU: resource.MustParse("2"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name: "aws-neuron-instance-type",
			Resources: map[v1.ResourceName]resource.Quantity{
				v1alpha1.ResourceAWSNeuron: resource.MustParse("2"),
			},
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:             "arm-instance-type",
			Architecture:     "arm64",
			OperatingSystems: utilsets.NewString("ios", "linux", "windows", "darwin"),
			Resources: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("16"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
			},
		}),
	}, nil
}

func (c *CloudProvider) GetRequirements(ctx context.Context, provider *v1alpha5.Provider) (scheduling.Requirements, error) {
	return scheduling.NewRequirements(), nil
}

func (c *CloudProvider) Delete(context.Context, *v1.Node) error {
	return nil
}

func (c *CloudProvider) Default(context.Context, *v1alpha5.Provisioner) {
}

func (c *CloudProvider) Validate(context.Context, *v1alpha5.Provisioner) *apis.FieldError {
	return nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "fake"
}
