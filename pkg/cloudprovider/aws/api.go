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
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/utils"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/project"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
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

var (
	SupportedOperatingSystems = []string{
		v1alpha1.OperatingSystemLinux,
	}
	SupportedArchitectures = []string{
		v1alpha1.ArchitectureAmd64,
		v1alpha1.ArchitectureArm64,
	}
)

type API struct {
	nodeAPI                *NodeAPI
	launchTemplateProvider *LaunchTemplateProvider
	subnetProvider         *SubnetProvider
	instanceTypeProvider   *InstanceTypeProvider
	instanceProvider       *InstanceProvider
}

func NewAPI(options cloudprovider.Options) *API {
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
	return &API{
		nodeAPI: &NodeAPI{ec2api: ec2api},
		launchTemplateProvider: &LaunchTemplateProvider{
			ec2api:                ec2api,
			cache:                 cache.New(CacheTTL, CacheCleanupInterval),
			securityGroupProvider: NewSecurityGroupProvider(ec2api),
			ssm:                   ssm.New(sess),
			clientSet:             options.ClientSet,
		},
		subnetProvider:       NewSubnetProvider(ec2api),
		instanceTypeProvider: NewInstanceTypeProvider(ec2api),
		instanceProvider:     &InstanceProvider{ec2api: ec2api},
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

// Create a set of nodes given the constraints.
func (a *API) Create(ctx context.Context, packings []*cloudprovider.Packing, provisioner *v1alpha1.Provisioner) ([]*cloudprovider.PackedNode, error) {
	instanceIDs := []*string{}
	instancePackings := map[string]*cloudprovider.Packing{}
	for _, packing := range packings {
		constraints := Constraints(*packing.Constraints)
		// 1. Get Subnets and constrain by zones
		zonalSubnets, err := a.subnetProvider.GetZonalSubnets(ctx, provisioner.Spec.Cluster.Name)
		if err != nil {
			return nil, fmt.Errorf("getting zonal subnets, %w", err)
		}
		zonalSubnetOptions := map[string][]*ec2.Subnet{}
		for zone, subnets := range zonalSubnets {
			if len(constraints.Zones) == 0 || functional.ContainsString(constraints.Zones, zone) {
				zonalSubnetOptions[zone] = subnets
			}
		}
		// 2. Get Launch Template
		launchTemplate, err := a.launchTemplateProvider.Get(ctx, provisioner, &constraints)
		if err != nil {
			return nil, fmt.Errorf("getting launch template, %w", err)
		}
		// 3. Create instance
		instanceID, err := a.instanceProvider.Create(ctx, launchTemplate, packing.InstanceTypeOptions, zonalSubnets, constraints.GetCapacityType())
		if err != nil {
			// TODO Aggregate errors and continue
			return nil, fmt.Errorf("creating capacity %w", err)
		}
		instancePackings[*instanceID] = packing
		instanceIDs = append(instanceIDs, instanceID)
	}

	// 4. Convert to Nodes
	nodes, err := a.nodeAPI.For(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("determining nodes, %w", err)
	}
	// 5. Convert to PackedNodes, TODO: move this logic into NodeAPI
	packedNodes := []*cloudprovider.PackedNode{}
	for instanceID, node := range nodes {
		packing := instancePackings[instanceID]
		node.Labels = packing.Constraints.Labels
		node.Spec.Taints = packing.Constraints.Taints
		packedNodes = append(packedNodes, &cloudprovider.PackedNode{
			Node: node,
			Pods: packing.Pods,
		})
	}
	return packedNodes, nil
}

func (a *API) GetInstanceTypes(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	return a.instanceTypeProvider.Get(ctx)
}

func (a *API) Terminate(ctx context.Context, nodes []*v1.Node) error {
	return a.instanceProvider.Terminate(ctx, nodes)
}
