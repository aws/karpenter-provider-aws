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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/project"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const (
	// CreationQPS limits the number of requests per second to CreateFleet
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
	CreationQPS = 2
	// CreationBurst limits the additional burst requests.
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
	CreationBurst = 100
	// CacheTTL restricts QPS to AWS APIs to this interval for verifying setup
	// resources. This value represents the maximum eventual consistency between
	// AWS actual state and the controller's ability to provision those
	// resources. Cache hits enable faster provisioning and reduced API load on
	// AWS APIs, which can have a serious impact on performance and scalability.
	// DO NOT CHANGE THIS VALUE WITHOUT DUE CONSIDERATION
	CacheTTL = 60 * time.Second
	// CacheCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	CacheCleanupInterval = 10 * time.Minute
)

type CloudProvider struct {
	instanceTypeProvider *InstanceTypeProvider
	subnetProvider       *SubnetProvider
	instanceProvider     *InstanceProvider
}

func NewCloudProvider(ctx context.Context, options cloudprovider.Options) *CloudProvider {
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
	ec2api := ec2.New(sess)
	subnetProvider := NewSubnetProvider(ec2api)
	instanceTypeProvider := NewInstanceTypeProvider(ec2api, subnetProvider)
	return &CloudProvider{
		instanceTypeProvider: instanceTypeProvider,
		subnetProvider:       subnetProvider,
		instanceProvider: &InstanceProvider{ec2api, instanceTypeProvider, subnetProvider,
			NewLaunchTemplateProvider(
				ec2api,
				NewAMIProvider(ssm.New(sess), options.ClientSet),
				NewSecurityGroupProvider(ec2api),
			),
		},
	}
}

// get the current region from EC2 IMDS
func getRegionFromIMDS(sess *session.Session) string {
	region, err := ec2metadata.New(sess).Region()
	if err != nil {
		panic(fmt.Sprintf("Failed to call the metadata server's region API, %s", err.Error()))
	}
	return region
}

// withUserAgent adds a karpenter specific user-agent string to AWS session
func withUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", project.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}

// Create a node given the constraints.
func (c *CloudProvider) Create(ctx context.Context, constraints *v1alpha5.Constraints, instanceTypes []cloudprovider.InstanceType, quantity int, callback func(*v1.Node) error) error {
	vendorConstraints, err := v1alpha1.Deserialize(constraints)
	if err != nil {
		return err
	}
	// Create will only return an error if zero nodes could be launched.
	// Partial fulfillment will be logged
	nodes, err := c.instanceProvider.Create(ctx, vendorConstraints, instanceTypes, quantity)
	if err != nil {
		return fmt.Errorf("launching instances, %w", err)
	}
	var errs error
	for _, node := range nodes {
		errs = multierr.Append(errs, callback(node))
	}
	return errs
}

// GetInstanceTypes returns all available InstanceTypes despite accepting a Constraints struct (note that it does not utilize Requirements)
func (c *CloudProvider) GetInstanceTypes(ctx context.Context, constraints *v1alpha5.Constraints) ([]cloudprovider.InstanceType, error) {
	vendorConstraints, err := v1alpha1.Deserialize(constraints)
	if err != nil {
		return nil, apis.ErrGeneric(err.Error())
	}
	return c.instanceTypeProvider.Get(ctx, vendorConstraints.AWS)
}

func (c *CloudProvider) Delete(ctx context.Context, node *v1.Node) error {
	return c.instanceProvider.Terminate(ctx, node)
}

// Validate the provisioner
func (c *CloudProvider) Validate(ctx context.Context, constraints *v1alpha5.Constraints) *apis.FieldError {
	vendorConstraints, err := v1alpha1.Deserialize(constraints)
	if err != nil {
		return apis.ErrGeneric(err.Error())
	}
	return vendorConstraints.AWS.Validate()
}

// Default the provisioner
func (c *CloudProvider) Default(ctx context.Context, constraints *v1alpha5.Constraints) {
	vendorConstraints, err := v1alpha1.Deserialize(constraints)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to deserialize provider, %s", err.Error())
		return
	}
	vendorConstraints.Default(ctx)
	if err := vendorConstraints.Serialize(constraints); err != nil {
		logging.FromContext(ctx).Errorf("Failed to serialize provider, %s", err.Error())
	}
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
}
