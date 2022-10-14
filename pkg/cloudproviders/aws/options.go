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

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/logging"

	awscache "github.com/aws/karpenter/pkg/cloudproviders/aws/cache"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/project"
)

const (
	// cacheTTL restricts QPS to AWS APIs to this interval for verifying setup
	// resources. This value represents the maximum eventual consistency between
	// AWS actual state and the controller's ability to provision those
	// resources. Cache hits enable faster provisioning and reduced API load on
	// AWS APIs, which can have a serious impact on performance and scalability.
	// DO NOT CHANGE THIS VALUE WITHOUT DUE CONSIDERATION
	cacheTTL = 60 * time.Second
	// cacheCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	cacheCleanupInterval = 10 * time.Minute
)

type Options struct {
	cloudprovider.Options

	Session                   *session.Session
	UnavailableOfferingsCache *awscache.UnavailableOfferings

	CacheCleanupInterval time.Duration
	CacheTTL             time.Duration
}

func NewOptionsOrDie(ctx context.Context, options cloudprovider.Options) Options {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws"))
	sess := withUserAgent(session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			client.DefaultRetryer{NumMaxRetries: client.DefaultRetryerMaxNumRetries},
		),
	)))
	if *sess.Config.Region == "" {
		logging.FromContext(ctx).Debug("AWS region not configured, asking EC2 Instance Metadata Service")
		*sess.Config.Region = getRegionFromIMDS(sess)
	}
	logging.FromContext(ctx).Debugf("Using AWS region %s", *sess.Config.Region)
	return Options{
		Options:                   options,
		Session:                   sess,
		UnavailableOfferingsCache: awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, cacheCleanupInterval)),
		CacheCleanupInterval:      cacheCleanupInterval,
		CacheTTL:                  cacheTTL,
	}
}

// get the current region from EC2 IMDS
func getRegionFromIMDS(sess *session.Session) string {
	region, err := ec2metadata.New(sess).Region()
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err))
	}
	return region
}

// withUserAgent adds a karpenter specific user-agent string to AWS session
func withUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", project.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}
