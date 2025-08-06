// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudprovider

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/controllers/nodeoverlay/validation"
	"sigs.k8s.io/karpenter/pkg/events"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
)

type CloudProvider struct {
	*cloudprovider.CloudProvider
}

func New(
	instanceTypeProvider instancetype.Provider,
	instanceProvider instance.Provider,
	recorder events.Recorder,
	kubeClient client.Client,
	amiProvider amifamily.Provider,
	securityGroupProvider securitygroup.Provider,
	capacityReservationProvider capacityreservation.Provider,
	instanceTypeStore *validation.InstanceTypeStore,
) *CloudProvider {
	return &CloudProvider{
		CloudProvider: cloudprovider.New(
			instanceTypeProvider,
			instanceProvider,
			recorder,
			kubeClient,
			amiProvider,
			securityGroupProvider,
			capacityReservationProvider,
			instanceTypeStore,
		),
	}
}

func (c *CloudProvider) Create(ctx context.Context, nodeClaim *karpv1.NodeClaim) (*karpv1.NodeClaim, error) {
	out, err := c.CloudProvider.Create(ctx, nodeClaim)
	if err != nil {
		return nil, err
	}
	out.Status.ProviderID = strings.Replace(out.Status.ProviderID, "aws", "kwok-aws", 1)
	return out, nil
}

func (c *CloudProvider) List(ctx context.Context) ([]*karpv1.NodeClaim, error) {
	out, err := c.CloudProvider.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, elem := range out {
		elem.Status.ProviderID = strings.Replace(elem.Status.ProviderID, "aws", "kwok-aws", 1)
	}
	return out, nil
}

func (c *CloudProvider) Get(ctx context.Context, providerID string) (*karpv1.NodeClaim, error) {
	elem, err := c.CloudProvider.Get(ctx, providerID)
	if err != nil {
		return nil, err
	}
	elem.Status.ProviderID = strings.Replace(elem.Status.ProviderID, "aws", "kwok-aws", 1)
	return elem, nil
}
