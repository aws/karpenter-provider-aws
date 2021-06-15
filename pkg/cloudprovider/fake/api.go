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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var (
	NotImplementedError = fmt.Errorf("provider is not implemented. Are you running the correct release for your cloud provider?")
)

type API struct {
	WantErr error
	// NodeReplicas is used by tests to control observed replicas.
	NodeReplicas    map[string]*int32
	NodeGroupStable bool
}

func NewAPI() *API {
	return &API{
		WantErr:         NotImplementedError,
		NodeReplicas:    make(map[string]*int32),
		NodeGroupStable: true,
	}
}

func (a *API) Create(ctx context.Context, packings []*cloudprovider.Packing, provisioner *v1alpha1.Provisioner) ([]*cloudprovider.PackedNode, error) {
	packedNodes := []*cloudprovider.PackedNode{}
	for _, packing := range packings {
		name := strings.ToLower(randomdata.SillyName())
		packedNodes = append(packedNodes, &cloudprovider.PackedNode{
			Node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: packing.Constraints.Labels,
				},
				Spec: v1.NodeSpec{
					ProviderID: fmt.Sprintf("fake:///%s", name),
					Taints:     packing.Constraints.Taints,
				},
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{
						v1.ResourcePods:   *packing.InstanceTypeOptions[0].Pods(),
						v1.ResourceCPU:    *packing.InstanceTypeOptions[0].CPU(),
						v1.ResourceMemory: *packing.InstanceTypeOptions[0].Memory(),
					},
				},
			},
			Pods: packing.Pods,
		})
	}
	return packedNodes, nil
}

func (a *API) GetInstanceTypes(ctx context.Context) ([]cloudprovider.InstanceType, error) {
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

func (a *API) Validate(ctx context.Context, spec *v1alpha1.ProvisionerSpec) *apis.FieldError {
	return nil
}

func (a *API) Terminate(ctx context.Context, nodes []*v1.Node) error {
	return nil
}
