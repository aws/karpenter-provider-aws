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

	"github.com/awslabs/karpenter/pkg/cloudprovider"
)

type Capacity struct {
}

func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.Constraints) (cloudprovider.PodPackings, error) {
	return nil, nil
}

func (c *Capacity) GetTopologyDomains(ctx context.Context, key cloudprovider.TopologyKey) ([]string, error) {
	return nil, nil
}
