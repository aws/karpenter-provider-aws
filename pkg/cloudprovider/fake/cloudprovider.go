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
	"sync"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Pallinder/go-randomdata"

	"k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/pkg/apis"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var sequentialNodeID uint64

type CloudProvider struct {
	InstanceTypes []cloudprovider.InstanceType

	// CreateCalls contains the arguments for every create call that was made since it was cleared
	mu          sync.Mutex
	CreateCalls []*cloudprovider.NodeRequest
	DeleteCalls []*v1.Node
}

type CreateCallArgs struct {
	Constraints   *v1alpha5.Constraints
	InstanceTypes []cloudprovider.InstanceType
	Quantity      int
}

func (c *CloudProvider) Create(ctx context.Context, nodeRequest *cloudprovider.NodeRequest) (*v1alpha5.InFlightNode, error) {
	c.mu.Lock()
	c.CreateCalls = append(c.CreateCalls, nodeRequest)
	c.mu.Unlock()
	name := fmt.Sprintf("n%04d-%s", atomic.AddUint64(&sequentialNodeID, 1), strings.ToLower(randomdata.SillyName()))
	instance := nodeRequest.InstanceTypeOptions[0]
	var zone, capacityType string
	for _, o := range instance.Offerings() {
		if nodeRequest.Template.Requirements.CapacityTypes().Has(o.CapacityType) && nodeRequest.Template.Requirements.Zones().Has(o.Zone) {
			zone = o.Zone
			capacityType = o.CapacityType
			break
		}
	}
	labels := map[string]string{
		v1.LabelTopologyZone:       zone,
		v1.LabelInstanceTypeStable: instance.Name(),
		v1alpha5.LabelCapacityType: capacityType,
	}
	for k, v := range nodeRequest.Template.Labels {
		labels[k] = v
	}
	return &v1alpha5.InFlightNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha5.InFlightNodeSpec{
			Labels:   labels,
			Capacity: instance.Resources(),
			Overhead: instance.Overhead(),
		},
	}, nil
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context) ([]cloudprovider.InstanceType, error) {
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
			OperatingSystems: sets.NewString("ios", "linux", "windows", "darwin"),
			Resources: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse("16"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
			},
		}),
	}, nil
}

func (c *CloudProvider) GetRequirements(ctx context.Context, provider *v1alpha5.Provider) (v1alpha5.Requirements, error) {
	return v1alpha5.Requirements{}, nil
}

func (c *CloudProvider) Delete(_ context.Context, node *v1.Node) error {
	c.mu.Lock()
	c.DeleteCalls = append(c.DeleteCalls, node)
	c.mu.Unlock()
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
