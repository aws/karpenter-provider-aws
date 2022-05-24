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
	"encoding/base64"
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
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/amifamily"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/project"
	"github.com/aws/karpenter/pkg/utils/sets"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	// CacheTTL restricts QPS to AWS APIs to this interval for verifying setup
	// resources. This value represents the maximum eventual consistency between
	// AWS actual state and the controller's ability to provision those
	// resources. Cache hits enable faster provisioning and reduced API load on
	// AWS APIs, which can have a serious impact on performance and scalability.
	// DO NOT CHANGE THIS VALUE WITHOUT DUE CONSIDERATION
	CacheTTL = 60 * time.Second
	// CacheCleanupInterval triggers cache cleanup (lazy eviction) at this interval.
	CacheCleanupInterval = 10 * time.Minute
	// MaxInstanceTypes defines the number of instance type options to pass to CreateFleet
	MaxInstanceTypes = 20
)

func init() {
	v1alpha5.NormalizedLabels = functional.UnionStringMaps(v1alpha5.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": v1.LabelTopologyZone})
}

type CloudProvider struct {
	instanceTypeProvider *InstanceTypeProvider
	subnetProvider       *SubnetProvider
	instanceProvider     *InstanceProvider
}

func NewCloudProvider(ctx context.Context, options cloudprovider.Options) *CloudProvider {
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
	ec2api := ec2.New(sess)
	subnetProvider := NewSubnetProvider(ec2api)
	instanceTypeProvider := NewInstanceTypeProvider(ec2api, subnetProvider)
	return &CloudProvider{
		instanceTypeProvider: instanceTypeProvider,
		subnetProvider:       subnetProvider,
		instanceProvider: &InstanceProvider{ec2api, instanceTypeProvider, subnetProvider,
			NewLaunchTemplateProvider(
				ctx,
				ec2api,
				options.ClientSet,
				amifamily.New(ssm.New(sess), cache.New(CacheTTL, CacheCleanupInterval)),
				NewSecurityGroupProvider(ec2api),
				getCABundle(ctx),
			),
		},
	}
}

// Create a node given the constraints.
func (c *CloudProvider) Create(ctx context.Context, nodeRequest *cloudprovider.NodeRequest) (*v1.Node, error) {
	vendorConstraints, err := v1alpha1.Deserialize(nodeRequest.Template.Provider)
	if err != nil {
		return nil, err
	}
	return c.instanceProvider.Create(ctx, vendorConstraints, nodeRequest)
}

// GetInstanceTypes returns all available InstanceTypes
func (c *CloudProvider) GetInstanceTypes(ctx context.Context) ([]cloudprovider.InstanceType, error) {
	return c.instanceTypeProvider.Get(ctx)
}

func (c *CloudProvider) Delete(ctx context.Context, node *v1.Node) error {
	return c.instanceProvider.Terminate(ctx, node)
}

func (c *CloudProvider) GetRequirements(ctx context.Context, provider *v1alpha5.Provider) (scheduling.Requirements, error) {
	instanceTypes, err := c.GetInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting instance types, %w", err)
	}
	requirements := cloudprovider.InstanceTypeRequirements(instanceTypes)

	awsprovider, err := v1alpha1.Deserialize(provider)
	if err != nil {
		return nil, apis.ErrGeneric(err.Error())
	}
	// Constrain AZs from subnets
	subnets, err := c.subnetProvider.Get(ctx, awsprovider)
	if err != nil {
		return nil, err
	}
	zones := sets.NewSet()
	for _, subnet := range subnets {
		zones.Insert(aws.StringValue(subnet.AvailabilityZone))
	}
	requirements.Add(scheduling.Requirements{v1.LabelTopologyZone: zones})
	return requirements, nil
}

// Validate the provisioner
func (c *CloudProvider) Validate(ctx context.Context, provisioner *v1alpha5.Provisioner) *apis.FieldError {
	provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
	if err != nil {
		return apis.ErrGeneric(err.Error())
	}
	return provider.Validate()
}

// Default the provisioner
func (c *CloudProvider) Default(ctx context.Context, provisioner *v1alpha5.Provisioner) {
	defaultLabels(provisioner)
}

func defaultLabels(provisioner *v1alpha5.Provisioner) {
	for key, value := range map[string]string{
		v1alpha5.LabelCapacityType: ec2.DefaultTargetCapacityTypeOnDemand,
		v1.LabelArchStable:         v1alpha5.ArchitectureAmd64,
	} {
		hasLabel := false
		if _, ok := provisioner.Spec.Labels[key]; ok {
			hasLabel = true
		}
		for _, requirement := range provisioner.Spec.Requirements {
			if requirement.Key == key {
				hasLabel = true
			}
		}
		if !hasLabel {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key: key, Operator: v1.NodeSelectorOpIn, Values: []string{value},
			})
		}
	}
}

// GetMachine by name
func (c *CloudProvider) GetMachine(ctx context.Context, name string) (*v1.Node, error) {
	instance, err := c.getInstance(ctx, name)
	if err != nil {
		return nil, err
	}
	instanceTypes, err := c.GetInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting instance types")
	}

	c.instanceProvider.instanceToNode(ctx, instance, instanceTypes)
	return nil, nil
}

// CreateMachine and return a corresponding node object
func (c *CloudProvider) CreateMachine(ctx context.Context, machine *v1alpha5.Machine) (*v1.Node, error) {
	provider, err := v1alpha1.Deserialize(machine.Spec.Provider)
	if err != nil {
		return nil, apis.ErrGeneric(err.Error())
	}

	instanceTypes, err := c.GetInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting instance types, %w", err)
	}

	requirements := scheduling.NewNodeSelectorRequirements(machine.Spec.Requirements...)

	node, err := c.instanceProvider.Create(ctx, provider, &cloudprovider.NodeRequest{
		Template: &scheduling.NodeTemplate{
			Labels:               machine.Spec.Labels,
			Requirements:         requirements,
			Taints:               machine.Spec.Taints,
			StartupTaints:        machine.Spec.StartupTaints,
			KubeletConfiguration: machine.Spec.KubeletConfiguration,
		},
		InstanceTypeOptions: cloudprovider.FilterInstanceTypes(instanceTypes, requirements, v1.ResourceList{}),
	})
	if err != nil {
		return nil, fmt.Errorf("creating instance, %w", err)
	}
	return node, nil
}

// DeleteMachine by name
func (c *CloudProvider) DeleteMachine(ctx context.Context, name string) error {
	// Get instance
	instance, err := c.getInstance(ctx, name)
	if err != nil {
		return err
	}
	// Idempotent delete
	if instance == nil {
		return nil
	}
	// Delete instance. It violates our invariant for have multiple with the same name
	if _, err := c.instanceProvider.ec2api.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{InstanceIds: []*string{instance.InstanceId}}); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("terminating instance %s, %w", aws.StringValue(instance.InstanceId), err)
	}
	return nil
}

func (c *CloudProvider) getInstance(ctx context.Context, name string) (*ec2.Instance, error) {
	describeInstanceOutput, err := c.instanceProvider.ec2api.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{{Name: aws.String(fmt.Sprintf("tag:%s", v1alpha5.MachineNameLabelKey)), Values: []*string{aws.String(name)}}},
	})
	if err != nil {
		return nil, fmt.Errorf("describing instances, %w", err)
	}
	instances := lo.FlatMap(describeInstanceOutput.Reservations, func(reservation *ec2.Reservation, _ int) []*ec2.Instance { return reservation.Instances })
	// Not found
	if len(instances) == 0 {
		return nil, nil
	}
	// Multiple found, invariant violated
	if len(instances) > 1 {
		return nil, fmt.Errorf("invariant violated, multiple instances with machine name %s, %v", name,
			lo.Map(instances, func(instance *ec2.Instance, _ int) *string { return instance.InstanceId }),
		)
	}
	return instances[0], nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "aws"
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

func getCABundle(ctx context.Context) *string {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	restConfig := injection.GetConfig(ctx)
	if restConfig == nil {
		return nil
	}
	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		logging.FromContext(ctx).Fatalf("Unable to discover caBundle, loading transport config, %v", err)
		return nil
	}
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	if err != nil {
		logging.FromContext(ctx).Fatalf("Unable to discover caBundle, loading TLS config, %v", err)
		return nil
	}
	logging.FromContext(ctx).Debugf("Discovered caBundle, length %d", len(transportConfig.TLS.CAData))
	return ptr.String(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData))
}
