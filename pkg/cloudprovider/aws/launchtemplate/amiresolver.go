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
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
)

// AMIResolver minimal abstraction over ec2iface.EC2API needed to lookup AMI images by ID.
type AMIResolver interface {
	GetImage(ctx context.Context, imageID string) (*ec2.Image, error)
}

// EC2AMIResolver native EC2 AMIResolver.
type EC2AMIResolver struct {
	ec2Client ec2iface.EC2API
}

func NewAWSAMIResolver(ec2Client ec2iface.EC2API) *EC2AMIResolver {
	return &EC2AMIResolver{
		ec2Client: ec2Client,
	}
}

func (c *EC2AMIResolver) GetImage(ctx context.Context, imageID string) (*ec2.Image, error) {
	output, err := c.ec2Client.DescribeImages(&ec2.DescribeImagesInput{ImageIds: []*string{&imageID}})
	if err != nil {
		return nil, fmt.Errorf("getting AMI image '%s', %w", imageID, err)
	}
	if len(output.Images) == 0 {
		err := fmt.Errorf("AMI image '%s' not found", imageID)
		return nil, err
	}
	return output.Images[0], nil
}

// CachingAMIResolver wrapper around an AMIResolver, caching image lookup results.
type CachingAMIResolver struct {
	cache *cache.Cache
	inner AMIResolver
}

func NewCachingAMIResolver(inner AMIResolver) *CachingAMIResolver {
	return &CachingAMIResolver{
		inner: inner,
		cache: cache.New(CacheTTL, CacheCleanupInterval),
	}
}

func (c *CachingAMIResolver) GetImage(ctx context.Context, imageID string) (*ec2.Image, error) {
	if image, ok := c.cache.Get(imageID); ok {
		if cachedValue, ok := image.(*ec2.Image); ok {
			return cachedValue, nil
		}
		if cachedError, ok := image.(error); ok {
			return nil, cachedError
		}
	}
	image, err := c.inner.GetImage(ctx, imageID)
	if err != nil {
		// Cache lookup errors to avoid running into API rate limits.
		c.cache.SetDefault(imageID, err)
		return nil, err
	}
	c.cache.SetDefault(imageID, image)
	return image, nil
}
