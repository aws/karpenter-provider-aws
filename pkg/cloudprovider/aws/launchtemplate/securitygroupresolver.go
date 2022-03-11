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

package launchtemplate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"
)

type SecurityGroupResolver interface {
	Get(ctx context.Context, filter map[string]string) ([]string, error)
}

type NativeSecurityGroupResolver struct {
	ec2api ec2iface.EC2API
}

func NewNativeSecurityGroupResolver(ec2api ec2iface.EC2API) *NativeSecurityGroupResolver {
	return &NativeSecurityGroupResolver{
		ec2api: ec2api,
	}
}

func (s *NativeSecurityGroupResolver) Get(ctx context.Context, filter map[string]string) ([]string, error) {
	// Get SecurityGroups
	securityGroups, err := s.getSecurityGroups(ctx, s.getFilters(filter))
	if err != nil {
		return nil, err
	}
	// Fail if no security groups found
	if len(securityGroups) == 0 {
		return nil, fmt.Errorf("no security groups exist given constraints")
	}
	// Convert to IDs
	securityGroupIds := []string{}
	for _, securityGroup := range securityGroups {
		securityGroupIds = append(securityGroupIds, aws.StringValue(securityGroup.GroupId))
	}
	return securityGroupIds, nil
}

func (s *NativeSecurityGroupResolver) getFilters(filter map[string]string) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range filter {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}
	return filters
}

func (s *NativeSecurityGroupResolver) getSecurityGroups(ctx context.Context, filters []*ec2.Filter) ([]*ec2.SecurityGroup, error) {
	output, err := s.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("describing security groups %+v, %w", filters, err)
	}
	return output.SecurityGroups, nil
}

type CachingSecurityGroupResolver struct {
	inner SecurityGroupResolver
	cache *cache.Cache
}

func NewCachingSecurityGroupResolver(inner SecurityGroupResolver) *CachingSecurityGroupResolver {
	return &CachingSecurityGroupResolver{
		inner: inner,
		cache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func getHash(filters map[string]string) string {
	hasher := sha256.New()
	for key, value := range filters {
		hasher.Write([]byte(key))
		hasher.Write([]byte{0})
		hasher.Write([]byte(value))
	}
	return hex.EncodeToString(hasher.Sum(make([]byte, 0)))
}

func (s *CachingSecurityGroupResolver) Get(ctx context.Context, filters map[string]string) ([]string, error) {
	hash := getHash(filters)
	if cachedValue, ok := s.cache.Get(hash); ok {
		if cachedGroups, ok := cachedValue.([]string); ok {
			return cachedGroups, nil
		}
		if cachedError, ok := cachedValue.(error); ok {
			return nil, cachedError
		}
	}
	output, err := s.inner.Get(ctx, filters)
	if err != nil {
		err = fmt.Errorf("describing security groups %+v, %w", filters, err)
		s.cache.SetDefault(hash, err)
		return nil, err
	}
	s.cache.SetDefault(hash, output)
	logging.FromContext(ctx).Debugf("Discovered security groups: %s", output)
	return output, nil
}
