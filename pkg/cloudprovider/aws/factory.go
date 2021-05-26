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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/utils"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"github.com/awslabs/karpenter/pkg/utils/project"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	// CacheTTL restricts QPS to AWS APIs to this interval for verifying setup resources.
	CacheTTL = 5 * time.Minute
	// CacheCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	CacheCleanupInterval = 10 * time.Minute
	// ClusterTagKeyFormat is set on all Kubernetes owned resources.
	ClusterTagKeyFormat = "kubernetes.io/cluster/%s"
	// KarpenterTagKeyFormat is set on all Karpenter owned resources.
	KarpenterTagKeyFormat = "karpenter.sh/cluster/%s"
)

type Factory struct {
	nodeFactory            *NodeFactory
	launchTemplateProvider *LaunchTemplateProvider
	subnetProvider         *SubnetProvider
	instanceTypeProvider   *InstanceTypeProvider
	instanceProvider       *InstanceProvider
}

func NewFactory(options cloudprovider.Options) *Factory {
	sess := withUserAgent(session.Must(
		session.NewSession(request.WithRetryer(
			&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint},
			utils.NewRetryer()))))
	if *sess.Config.Region == "" {
		zap.S().Debug("AWS region not configured, asking EC2 Instance Metadata Service")
		*sess.Config.Region = getRegionFromIMDS(sess)
	}
	zap.S().Debugf("Using AWS region %s", *sess.Config.Region)
	ec2api := ec2.New(sess)
	launchTemplateProvider := &LaunchTemplateProvider{
		ec2api: ec2api,
		cache:  cache.New(CacheTTL, CacheCleanupInterval),
		securityGroupProvider: &SecurityGroupProvider{
			ec2api: ec2api,
			cache:  cache.New(CacheTTL, CacheCleanupInterval),
		},
		ssm:       ssm.New(sess),
		clientSet: options.ClientSet,
	}
	return &Factory{
		nodeFactory:            &NodeFactory{ec2api: ec2api},
		launchTemplateProvider: launchTemplateProvider,
		subnetProvider:         NewSubnetProvider(ec2api),
		instanceTypeProvider:   NewInstanceTypeProvider(ec2api),
		instanceProvider:       &InstanceProvider{ec2api: ec2api},
	}
}

func (f *Factory) CapacityFor(provisioner *v1alpha1.Provisioner) cloudprovider.Capacity {
	return &Capacity{
		provisioner:            provisioner,
		nodeFactory:            f.nodeFactory,
		instanceProvider:       f.instanceProvider,
		launchTemplateProvider: f.launchTemplateProvider,
		instanceTypeProvider:   f.instanceTypeProvider,
		subnetProvider:         f.subnetProvider,
	}
}

// get the current region from EC2 IMDS
func getRegionFromIMDS(sess *session.Session) string {
	region, err := ec2metadata.New(sess).Region()
	log.PanicIfError(err, "failed to call the metadata server's region API")
	return region
}

// withUserAgent adds a karpenter specific user-agent string to AWS session
func withUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", project.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}
