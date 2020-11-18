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
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"knative.dev/pkg/ptr"
)

var (
	NotImplementedError = fmt.Errorf("provider is not implemented. Are you running the correct release for your cloud provider?")
)

type Factory struct {
	WantErr error
	// NodeReplicas is use by tests to control observed replicas.
	NodeReplicas map[string]int32

	nodegroup *NodeGroup
}

func NewFactory(options cloudprovider.Options) *Factory {
	return &Factory{
		NodeReplicas: make(map[string]int32),
	}
}

func NewNotImplementedFactory() *Factory {
	return &Factory{WantErr: NotImplementedError}
}

func (f *Factory) NodeGroupFor(sng *v1alpha1.ScalableNodeGroupSpec) cloudprovider.NodeGroup {
	if f.nodegroup == nil {
		f.nodegroup = &NodeGroup{
			WantErr:  f.WantErr,
			Stable:   true,
			Replicas: ptr.Int32(f.NodeReplicas[sng.ID]),
		}
	}
	return f.nodegroup
}

func (f *Factory) QueueFor(spec *v1alpha1.QueueSpec) cloudprovider.Queue {
	return &Queue{Id: spec.ID, WantErr: f.WantErr}
}
