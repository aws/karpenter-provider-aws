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

package subnet

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"

	awscache "github.com/aws/karpenter/pkg/cache"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

type Provider struct {
	sync.Mutex
	ec2api ec2iface.EC2API
	cache  *cache.Cache
	cm     *pretty.ChangeMonitor
}

const TTL = 5 * time.Minute

func NewProvider(ec2api ec2iface.EC2API) *Provider {
	return &Provider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache for v1beta1, utilize resolved subnet from the AWSNodeTemplate.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache: cache.New(TTL, awscache.CleanupInterval),
	}
}

func (p *Provider) List(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) ([]*ec2.Subnet, error) {
	p.Lock()
	defer p.Unlock()
	filters := getFilters(nodeTemplate)
	hash, err := hashstructure.Hash(filters, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
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
	p.cache.SetDefault(fmt.Sprint(hash), output.Subnets)
	subnetLog := Pretty(output.Subnets)
	if p.cm.HasChanged("subnets", subnetLog) {
		logging.FromContext(ctx).With("subnets", subnetLog).Debugf("discovered subnets")
	}
	return output.Subnets, nil
}

func (p *Provider) LivenessProbe(req *http.Request) error {
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
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("subnet-id"),
				Values: aws.StringSlice(functional.SplitCommaSeparatedString(value)),
			})
		} else if value == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(key)},
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: aws.StringSlice(functional.SplitCommaSeparatedString(value)),
			})
		}
	}
	return filters
}

func Pretty(subnets []*ec2.Subnet) []string {
	names := []string{}
	for _, subnet := range subnets {
		names = append(names, fmt.Sprintf("%s (%s)", aws.StringValue(subnet.SubnetId), aws.StringValue(subnet.AvailabilityZone)))
	}
	return names
}

func (p *Provider) Reset() {
	p.cache.Flush()
}
