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
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"go.uber.org/multierr"
	"knative.dev/pkg/apis"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CloudProvider struct {
	InstanceTypes []cloudprovider.InstanceType
}

func (c *CloudProvider) Create(_ context.Context, constraints *v1alpha5.Constraints, instanceTypes []cloudprovider.InstanceType, quantity int, bind func(*v1.Node) error) error {
	var err error
	for i := 0; i < quantity; i++ {
		name := strings.ToLower(randomdata.SillyName())
		instance := instanceTypes[0]
		var zone, capacityType string
		for _, o := range instance.Offerings() {
			if constraints.Requirements.CapacityTypes().Has(o.CapacityType) && constraints.Requirements.Zones().Has(o.Zone) {
				zone = o.Zone
				capacityType = o.CapacityType
				break
			}
		}

		err = multierr.Append(err, bind(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					v1.LabelTopologyZone:       zone,
					v1.LabelInstanceTypeStable: instance.Name(),
					v1alpha5.LabelCapacityType: capacityType,
				},
			},
			Spec: v1.NodeSpec{
				ProviderID: fmt.Sprintf("fake:///%s/%s", name, zone),
			},
			Status: v1.NodeStatus{
				NodeInfo: v1.NodeSystemInfo{
					Architecture:    instance.Architecture(),
					OperatingSystem: v1alpha5.OperatingSystemLinux,
				},
				Allocatable: v1.ResourceList{
					v1.ResourcePods:   *instance.Pods(),
					v1.ResourceCPU:    *instance.CPU(),
					v1.ResourceMemory: *instance.Memory(),
				},
			},
		}))
	}
	return err
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, _ *v1alpha5.Provider) ([]cloudprovider.InstanceType, error) {
	if c.InstanceTypes != nil {
		return c.InstanceTypes, nil
	}
	return []cloudprovider.InstanceType{
		NewInstanceType(InstanceTypeOptions{
			Name: "default-instance-type",
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:      "pod-eni-instance-type",
			AWSPodENI: resource.MustParse("1"),
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:   "small-instance-type",
			CPU:    resource.MustParse("2"),
			Memory: resource.MustParse("2Gi"),
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:       "nvidia-gpu-instance-type",
			NvidiaGPUs: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:    "amd-gpu-instance-type",
			AMDGPUs: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:       "aws-neuron-instance-type",
			AWSNeurons: resource.MustParse("2"),
		}),
		NewInstanceType(InstanceTypeOptions{
			Name:         "arm-instance-type",
			Architecture: "arm64",
		}),
	}, nil
}

func (c *CloudProvider) Delete(context.Context, *v1.Node) error {
	return nil
}

func (c *CloudProvider) Default(context.Context, *v1alpha5.Constraints) {
}

func (c *CloudProvider) Validate(context.Context, *v1alpha5.Constraints) *apis.FieldError {
	return nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "fake"
}
