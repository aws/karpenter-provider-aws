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

func (p *Provider) Get(ctx context.Context, nodeClass *v1beta1.NodeClass) (*v1beta1.HostResourceGroup, error) {
	p.Lock()
	defer p.Unlock()
    var nextToken *string = nil
	var groups []*resourcegroups.GroupIdentifier
	for {
		resp, err := p.resourcegroups.ListGroupsWithContext(ctx, &resourcegroups.ListGroupsInput{ NextToken: nextToken})
		if err != nil {
			return nil, err
		}
        nextToken = resp.NextToken
        for i := range resp.GroupIdentifiers {
            groups = append(groups, resp.GroupIdentifiers[i])
        }
		if nextToken == nil {
			break
		}
	}

	// filter resp to only include those that match the hostResourceGroupSelector Name
	for i := range groups {
		group := groups[i]
		for x := range nodeClass.Spec.HostResourceGroupSelectorTerms {
			selector := nodeClass.Spec.HostResourceGroupSelectorTerms[x]
				logging.FromContext(ctx).
					With("group", group).
					With("selector", selector).
					Debugf("checking for host resource group match")
			if *group.GroupName == selector.Name {
				match := &v1beta1.HostResourceGroup{ARN: *group.GroupArn, Name: *group.GroupName}
				logging.FromContext(ctx).
					With("Matched hrg", match).
					Debugf("discovered host resource group configuration")

				return match, nil
			}
		}
	}
	logging.FromContext(ctx).
		With("groups", groups).
		Debugf("No hrg matched")

	// No matching groups
	return nil, nil
}
