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

package packing

import (
	"context"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

type PodPacker struct {
	ec2 ec2iface.EC2API
}

// PodPacker helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error)
}

// Packings contains a list of pods that can be placed on any of Instance type
// in the InstanceTypeOptions
type Packings struct {
	Pods                []*v1.Pod
	InstanceTypeOptions []string
}

func NewPacker(ec2 ec2iface.EC2API) *PodPacker {
	return &PodPacker{ec2: ec2}
}

// Pack returns the packings for the provided pods. Computes a set of viable
// instance types for each packing of pods. Instance variety enables EC2 to make
// better cost and availability decisions.
func (p *PodPacker) Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error) {
	zap.S().Debugf("Successfully packed %d pod(s) onto %d node(s)", len(pods), 1)
	return []*Packings{
		{
			InstanceTypeOptions: []string{"m5.large"}, // TODO, prioritize possible instance types
			Pods:                pods,
		},
	}, nil
}
