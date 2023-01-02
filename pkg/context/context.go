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

package context

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"

	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	"github.com/aws/karpenter/pkg/utils/project"

	cloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
)

// Context is injected into the AWS CloudProvider's factories
type Context struct {
	cloudprovider.Context

	Session                   *session.Session
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	EC2API                    ec2iface.EC2API
	SubnetProvider            *subnet.Provider
	SecurityGroupProvider     *securitygroup.Provider
}

func NewOrDie(ctx cloudprovider.Context) Context {
	ctx.Context = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws"))
	sess := withUserAgent(session.Must(session.NewSession(
		request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			client.DefaultRetryer{NumMaxRetries: client.DefaultRetryerMaxNumRetries},
		),
	)))
	if *sess.Config.Region == "" {
		logging.FromContext(ctx).Debug("retrieving region from IMDS")
		region, err := ec2metadata.New(sess).Region()
		*sess.Config.Region = lo.Must(region, err, "failed to get region from metadata server")
	}
	logging.FromContext(ctx).With("region", *sess.Config.Region).Debugf("discovered region")

	ec2api := ec2.New(sess)
	if err := checkEC2Connectivity(ctx, ec2api); err != nil {
		logging.FromContext(ctx).Fatalf("checking EC2 API connectivity, %s", err)
	}
	subnetProvider := subnet.NewProvider(ec2api)
	securityGroupProvider := securitygroup.NewProvider(ec2api)
	return Context{
		Context:                   ctx,
		Session:                   sess,
		UnavailableOfferingsCache: awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, awscache.CleanupInterval)),
		EC2API:                    ec2api,
		SubnetProvider:            subnetProvider,
		SecurityGroupProvider:     securityGroupProvider,
	}
}

// withUserAgent adds a karpenter specific user-agent string to AWS session
func withUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", project.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}

// checkEC2Connectivity makes a dry-run call to DescribeInstanceTypes.  If it fails, we provide an early indicator that we
// are having issues connecting to the EC2 API.
func checkEC2Connectivity(ctx context.Context, api *ec2.EC2) error {
	_, err := api.DescribeInstanceTypesWithContext(ctx, &ec2.DescribeInstanceTypesInput{DryRun: aws.Bool(true)})
	var aerr awserr.Error
	if errors.As(err, &aerr) && aerr.Code() == "DryRunOperation" {
		return nil
	}
	return err
}
