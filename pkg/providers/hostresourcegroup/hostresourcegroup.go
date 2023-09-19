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

package hostresourcegroup

import (
	"context"
	"sync"

	//"github.com/aws/aws-sdk-go/service/resourcegroups"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	"github.com/aws/aws-sdk-go/service/resourcegroups/resourcegroupsiface"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
)

type Provider struct {
	sync.RWMutex
	resourcegroups resourcegroupsiface.ResourceGroupsAPI
	cache          *cache.Cache
	cm             *pretty.ChangeMonitor
}

func NewProvider(rgapi resourcegroupsiface.ResourceGroupsAPI, cache *cache.Cache) *Provider {
	return &Provider{
		resourcegroups: rgapi,
		cm:             pretty.NewChangeMonitor(),
		// TODO: Remove cache for v1beta1, utilize resolved subnet from the AWSNodeTemplate.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache: cache,
	}
}

func (p *Provider) Get(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (*v1beta1.HostResourceGroup, error) {
	p.Lock()
	defer p.Unlock()
    var match *v1beta1.HostResourceGroup
	err := p.resourcegroups.ListGroupsPagesWithContext(ctx, &resourcegroups.ListGroupsInput{}, func(page *resourcegroups.ListGroupsOutput, lastPage bool) bool {
		for i := range page.GroupIdentifiers {
			for x := range nodeClass.Spec.HostResourceGroupSelectorTerms {
				selector := nodeClass.Spec.HostResourceGroupSelectorTerms[x]
				if *page.GroupIdentifiers[i].GroupName == selector.Name {
                    match = &v1beta1.HostResourceGroup{ ARN: *page.GroupIdentifiers[i].GroupArn, Name: *page.GroupIdentifiers[i].GroupName }
                    return false
				}
			}
		}
		return lastPage
	})

	if err != nil {
		logging.FromContext(ctx).Errorf("discovery resource groups, %w", err)
		return nil, err
	}

	return match, nil
}
