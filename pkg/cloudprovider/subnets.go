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

package cloudprovider

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscontext "github.com/aws/karpenter/pkg/context"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type SubnetProvider struct {
	sync.Mutex
	ec2api ec2iface.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

func NewSubnetProvider(ec2api ec2iface.EC2API) *SubnetProvider {
	return &SubnetProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		cache:  cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval),
	}
}

func (p *SubnetProvider) Get(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]*ec2.Subnet, error) {
	p.Lock()
	defer p.Unlock()
	filters := getFilters(nodeTemplate)
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	if subnets, ok := p.cache.Get(fmt.Sprint(hash)); ok {
		return subnets.([]*ec2.Subnet), nil
	}
	output, err := p.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing subnets %s, %w", pretty.Concise(filters), err)
	}
	if len(output.Subnets) == 0 {
		return nil, fmt.Errorf("no subnets matched selector %v", nodeTemplate.Spec.SubnetSelector)
	}
	p.cache.SetDefault(fmt.Sprint(hash), output.Subnets)
	subnetLog := prettySubnets(output.Subnets)
	if p.cm.HasChanged("subnets", subnetLog) {
		logging.FromContext(ctx).With("subnets", subnetLog).Debugf("discovered subnets")
	}
	return output.Subnets, nil
}

func (p *SubnetProvider) LivenessProbe(req *http.Request) error {
	p.Lock()
	//nolint: staticcheck
	p.Unlock()
	return nil
}

func getFilters(nodeTemplate *v1alpha1.AWSNodeTemplate) []*ec2.Filter {
	filters := []*ec2.Filter{}
	// Filter by subnet
	for key, value := range nodeTemplate.Spec.SubnetSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("subnet-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else if value == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(key)},
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: []*string{aws.String(value)},
			})
		}
	}
	return filters
}

func prettySubnets(subnets []*ec2.Subnet) []string {
	names := []string{}
	for _, subnet := range subnets {
		names = append(names, fmt.Sprintf("%s (%s)", aws.StringValue(subnet.SubnetId), aws.StringValue(subnet.AvailabilityZone)))
	}
	return names
}
