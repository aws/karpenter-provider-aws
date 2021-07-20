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
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

type CloudProvider struct{}

func (c *CloudProvider) Create(ctx context.Context, provisioner *v1alpha3.Provisioner, packing *cloudprovider.Packing, bind func(*v1.Node) error) chan error {
	name := strings.ToLower(randomdata.SillyName())
	// Pick first instance type option
	instance := packing.InstanceTypeOptions[0]
	// Pick first zone
	zones := instance.Zones()
	if len(packing.Constraints.Zones) != 0 {
		zones = functional.IntersectStringSlice(packing.Constraints.Zones, instance.Zones())
	}
	zone := zones[0]

	err := make(chan error)
	go func() {
		err <- bind(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: packing.Constraints.Labels,
			},
			Spec: v1.NodeSpec{
				ProviderID: fmt.Sprintf("fake:///%s/%s", name, zone),
				Taints:     packing.Constraints.Taints,
			},
			Status: v1.NodeStatus{
				NodeInfo: v1.NodeSystemInfo{
					Architecture:    instance.Architectures()[0],
					OperatingSystem: instance.OperatingSystems()[0],
				},
				Allocatable: v1.ResourceList{
					v1.ResourcePods:   *instance.Pods(),
					v1.ResourceCPU:    *instance.CPU(),
					v1.ResourceMemory: *instance.Memory(),
				},
			},
		})
	}()
	return err
}

func (c *CloudProvider) GetInstanceTypes(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	return []cloudprovider.InstanceType{
		NewInstanceType(InstanceTypeOptions{
			name: "default-instance-type",
		}),
		NewInstanceType(InstanceTypeOptions{
			name:       "nvidia-gpu-instance-type",
			nvidiaGPUs: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			name:    "amd-gpu-instance-type",
			amdGPUs: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			name:       "aws-neuron-instance-type",
			awsNeurons: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			name:             "windows-instance-type",
			operatingSystems: []string{"windows"},
		}),
		NewInstanceType(InstanceTypeOptions{
			name:          "arm-instance-type",
			architectures: []string{"arm64"},
		}),
	}, nil
}

func (c *CloudProvider) ValidateSpec(context.Context, *v1alpha3.ProvisionerSpec) *apis.FieldError {
	return nil
}

func (c *CloudProvider) ValidateConstraints(context.Context, *v1alpha3.Constraints) *apis.FieldError {
	return nil
}

func (c *CloudProvider) Terminate(ctx context.Context, node *v1.Node) error {
	return nil
}
