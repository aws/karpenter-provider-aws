package license

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"

	"github.com/aws/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type Provider struct {
	sync.RWMutex
	ec2api      ec2iface.EC2API
	cache       *cache.Cache
	cm          *pretty.ChangeMonitor
}

func NewProvider(ec2api ec2iface.EC2API, cache *cache.Cache) *Provider {
	return &Provider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache for v1beta1, utilize resolved subnet from the AWSNodeTemplate.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache: cache,
	}
}

func (p *Provider) List(ctx context.Context, nodeClass *v1beta1.NodeClass) ([]*ec2.LicenseConfiguration, error) {
    return nil, nil
}
