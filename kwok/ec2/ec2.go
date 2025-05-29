// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/kwok/apis/v1alpha1"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/aws/karpenter-provider-aws/kwok/strategy"
	"github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Client struct {
	ec2.Client
	rateLimiterProvider RateLimiterProvider
	kubeClient          client.Client
	clock               clock.Clock

	region        string
	namespace     string
	instanceTypes []ec2types.InstanceTypeInfo
	subnets       []ec2types.Subnet
	strategy      strategy.Strategy

	instances            sync.Map
	instanceLaunchCancel sync.Map

	launchTemplates        sync.Map
	launchTemplateNameToID sync.Map
}

func NewClient(region, namespace string, ec2Client *ec2.Client, rateLimiterProvider RateLimiterProvider, strategy strategy.Strategy, kubeClient client.Client, clk clock.Clock, cfg *rest.Config) *Client {
	var instanceTypes []ec2types.InstanceTypeInfo
	instanceTypesPaginator := ec2.NewDescribeInstanceTypesPaginator(ec2Client, &ec2.DescribeInstanceTypesInput{
		MaxResults: aws.Int32(100),
	})
	for instanceTypesPaginator.HasMorePages() {
		output := lo.Must(instanceTypesPaginator.NextPage(context.Background()))
		instanceTypes = append(instanceTypes, output.InstanceTypes...)
	}
	var subnets []ec2types.Subnet
	subnetsPaginator := ec2.NewDescribeSubnetsPaginator(ec2Client, &ec2.DescribeSubnetsInput{
		MaxResults: aws.Int32(100),
	})
	for subnetsPaginator.HasMorePages() {
		output := lo.Must(subnetsPaginator.NextPage(context.Background()))
		subnets = append(subnets, output.Subnets...)
	}

	c := &Client{
		Client:              *ec2Client,
		rateLimiterProvider: rateLimiterProvider,
		kubeClient:          kubeClient,
		clock:               clk,

		region:        region,
		namespace:     namespace,
		instanceTypes: instanceTypes,
		subnets:       subnets,
		strategy:      strategy,

		instances: sync.Map{},

		launchTemplates:        sync.Map{},
		launchTemplateNameToID: sync.Map{},
	}
	c.readBackup(context.Background(), cfg)
	return c
}

func (c *Client) readBackup(ctx context.Context, cfg *rest.Config) {
	configMaps := &corev1.ConfigMapList{}
	lo.Must0(client.IgnoreNotFound(lo.Must(client.New(cfg, client.Options{})).List(ctx, configMaps, client.InNamespace(c.namespace))))

	configMaps.Items = lo.Filter(configMaps.Items, func(c corev1.ConfigMap, _ int) bool {
		return strings.Contains(c.Name, "kwok-aws-instances-")
	})
	total := 0
	for _, cm := range configMaps.Items {
		if cm.Data["instances"] != "" {
			var instances []ec2types.Instance
			lo.Must0(json.Unmarshal([]byte(cm.Data["instances"]), &instances))
			for _, instance := range instances {
				c.instances.Store(lo.FromPtr(instance.InstanceId), instance)
			}
			total += len(instances)
		}
	}
	log.FromContext(ctx).WithValues("count", total).Info("loaded instances from backup")
}

//nolint:gocyclo
func (c *Client) backupInstances(ctx context.Context) error {
	var instances []ec2types.Instance
	c.instances.Range(func(k, v interface{}) bool {
		instances = append(instances, v.(ec2types.Instance))
		return true
	})
	sort.Slice(instances, func(i, j int) bool {
		return lo.FromPtr(instances[i].LaunchTime).Before(lo.FromPtr(instances[j].LaunchTime))
	})

	// TODO: We could consider reducing memory consumption by using nextTokens and continue
	configMaps := &corev1.ConfigMapList{}
	if err := c.kubeClient.List(ctx, configMaps, client.InNamespace(c.namespace)); err != nil {
		return fmt.Errorf("listing configmaps, %w", err)
	}
	configMaps.Items = lo.Filter(configMaps.Items, func(c corev1.ConfigMap, _ int) bool {
		return strings.Contains(c.Name, "kwok-aws-instances-")
	})
	// Sort all the ConfigMaps by their numerical value
	// This ensures that we delete the higher numerical ConfigMaps first
	sort.SliceStable(configMaps.Items, func(i, j int) bool {
		rawI := strings.Split(configMaps.Items[i].Name, "kwok-aws-instances-")
		if len(rawI) != 2 {
			return false
		}
		rawJ := strings.Split(configMaps.Items[j].Name, "kwok-aws-instances-")
		if len(rawJ) != 2 {
			return false
		}
		iNum, err := strconv.Atoi(rawI[1])
		if err != nil {
			return false
		}
		jNum, err := strconv.Atoi(rawJ[1])
		if err != nil {
			return false
		}
		return iNum < jNum
	})
	// Clean-up any ConfigMaps that don't need to be there because of the count
	// We store 500 instances per ConfigMap
	numConfigMaps := int(math.Ceil(float64(len(instances)) / float64(500)))
	if numConfigMaps < len(configMaps.Items) {
		errs := make([]error, numConfigMaps)
		workqueue.ParallelizeUntil(ctx, 10, len(configMaps.Items)-numConfigMaps, func(i int) {
			if err := c.kubeClient.Delete(ctx, &configMaps.Items[len(configMaps.Items)-i-1]); client.IgnoreNotFound(err) != nil {
				errs[i] = fmt.Errorf("deleting configmap %q, %w", configMaps.Items[len(configMaps.Items)-i-1].Name, err)
			}
		})
		if err := multierr.Combine(errs...); err != nil {
			return err
		}
	}

	errs := make([]error, numConfigMaps)
	workqueue.ParallelizeUntil(ctx, 10, numConfigMaps, func(i int) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("kwok-aws-instances-%d", i),
				Namespace: c.namespace,
			},
		}
		if err := c.kubeClient.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
			if k8serrors.IsNotFound(err) {
				if err = c.kubeClient.Create(ctx, cm); err != nil {
					errs[i] = fmt.Errorf("creating configmap %q, %w", cm.Name, err)
					return
				}
			} else {
				errs[i] = fmt.Errorf("getting configmap %q, %w", cm.Name, err)
				return
			}
		}
		stored := cm.DeepCopy()
		cm.Data = map[string]string{"instances": string(removeNullFields(lo.Must(json.Marshal(lo.Slice(instances, i*500, (i+1)*500)))))}
		if !equality.Semantic.DeepEqual(cm, stored) {
			if err := c.kubeClient.Patch(ctx, cm, client.MergeFrom(stored)); err != nil {
				errs[i] = fmt.Errorf("patching configmap %q, %w", cm.Name, err)
				return
			}
		}
	})
	return multierr.Combine(errs...)
}

// StartBackupThread initiates the thread that is responsible for storing instances in ConfigMaps on the cluster
func (c *Client) StartBackupThread(ctx context.Context) {
	for {
		if err := c.backupInstances(ctx); err != nil {
			log.FromContext(ctx).Error(err, "unable to backup instances")
			continue
		}
		select {
		case <-time.After(time.Second * 5):
		case <-ctx.Done():
			return
		}
	}
}

// StartKillNodeThread initiates the thread that is responsible for killing nodes on the cluster that no longer have an instance representation (similar to CCM)
func (c *Client) StartKillNodeThread(ctx context.Context) {
	for {
		nodes := &corev1.NodeList{}
		if err := c.kubeClient.List(ctx, nodes, client.MatchingLabels{v1alpha1.KwokLabelKey: v1alpha1.KwokLabelValue}); err != nil {
			log.FromContext(ctx).Error(err, "unable to list nodes")
			continue
		}
		for _, node := range nodes.Items {
			id, err := utils.ParseInstanceID(node.Spec.ProviderID)
			if err != nil {
				log.FromContext(ctx).Error(err, "unable to parse instance id for node %q", node.Name)
				continue
			}
			if _, ok := c.instances.Load(id); !ok {
				if err = c.kubeClient.Delete(ctx, &node); client.IgnoreNotFound(err) != nil {
					log.FromContext(ctx).Error(err, "unable to delete node %q due to gone instance", node.Name)
					continue
				}
			}
		}

		select {
		case <-time.After(time.Second * 5):
		case <-ctx.Done():
			return
		}
	}
}

func removeNullFields(bytes []byte) []byte {
	var mapSlice []map[string]interface{}
	lo.Must0(json.Unmarshal(bytes, &mapSlice))
	for _, elem := range mapSlice {
		for k, v := range elem {
			if v == nil {
				delete(elem, k)
			}
		}
	}
	return lo.Must(json.Marshal(mapSlice))
}

//nolint:gocyclo
func (c *Client) DescribeLaunchTemplates(_ context.Context, input *ec2.DescribeLaunchTemplatesInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if !c.rateLimiterProvider.DescribeLaunchTemplates().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	out := &ec2.DescribeLaunchTemplatesOutput{}
	ids := input.LaunchTemplateIds
	for _, name := range input.LaunchTemplateNames {
		raw, ok := c.launchTemplateNameToID.Load(name)
		if !ok {
			return nil, &smithy.GenericAPIError{
				Code:    "InvalidLaunchTemplateName.NotFoundException",
				Message: "At least one of the launch templates specified in the request does not exist",
			}
		}
		ids = append(ids, raw.(string))
	}

	for _, id := range ids {
		raw, ok := c.launchTemplates.Load(id)
		if !ok {
			return nil, &smithy.GenericAPIError{
				Code:    "InvalidLaunchTemplateId.NotFoundException",
				Message: "At least one of the launch templates specified in the request does not exist",
			}
		}
		lt := raw.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
		out.LaunchTemplates = append(out.LaunchTemplates, *lt.A)
	}

	for _, filter := range input.Filters {
		switch lo.FromPtr(filter.Name) {
		case "create-time":
			c.launchTemplates.Range(func(k, v interface{}) bool {
				lt := v.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
				for _, value := range filter.Values {
					if lo.FromPtr(lt.A.CreateTime).Equal(lo.Must(time.Parse(time.RFC3339, value))) {
						out.LaunchTemplates = append(out.LaunchTemplates, *lt.A)
					}
				}
				return true
			})
		case "launch-template-name":
			c.launchTemplates.Range(func(k, v interface{}) bool {
				lt := v.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
				for _, value := range filter.Values {
					if lo.FromPtr(lt.A.LaunchTemplateName) == value {
						out.LaunchTemplates = append(out.LaunchTemplates, *lt.A)
					}
				}
				return true
			})
		case "tag-key":
			c.launchTemplates.Range(func(k, v interface{}) bool {
				lt := v.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
				for _, value := range filter.Values {
					for _, t := range lt.A.Tags {
						if value == lo.FromPtr(t.Key) {
							out.LaunchTemplates = append(out.LaunchTemplates, *lt.A)
						}
					}
				}
				return true
			})
		default:
			// This looks for a tag with a specific value
			if strings.Contains(lo.FromPtr(filter.Name), "tag:") {
				key := strings.Split(lo.FromPtr(filter.Name), "tag:")[1]
				c.launchTemplates.Range(func(k, v interface{}) bool {
					lt := v.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
					for _, value := range filter.Values {
						for _, t := range lt.A.Tags {
							if key == lo.FromPtr(t.Key) && value == lo.FromPtr(t.Value) {
								out.LaunchTemplates = append(out.LaunchTemplates, *lt.A)
							}
						}
					}
					return true
				})
			}
		}
	}
	return out, nil
}

//nolint:gocyclo
func (c *Client) CreateFleet(ctx context.Context, input *ec2.CreateFleetInput, _ ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error) {
	if !c.rateLimiterProvider.CreateFleet().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}
	if input.TargetCapacitySpecification == nil {
		return nil, fmt.Errorf("target capacity specification is required")
	}

	var fleetInstances []ec2types.CreateFleetInstance
	for range lo.FromPtr(input.TargetCapacitySpecification.TotalTargetCapacity) {
		ltConfig := input.LaunchTemplateConfigs[0]
		ltID := lo.FromPtr(ltConfig.LaunchTemplateSpecification.LaunchTemplateId)
		if ltConfig.LaunchTemplateSpecification.LaunchTemplateName != nil {
			raw, ok := c.launchTemplateNameToID.Load(lo.FromPtr(ltConfig.LaunchTemplateSpecification.LaunchTemplateName))
			if !ok {
				// TODO: Eventually we should make this a real NotFound error returned by the AWS API
				return nil, fmt.Errorf("launch template not found")
			}
			ltID = raw.(string)
		}
		raw, ok := c.launchTemplates.Load(ltID)
		if !ok {
			// TODO: Eventually we should make this a real NotFound error returned by the AWS API
			return nil, fmt.Errorf("launch template not found")
		}
		lt := raw.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])

		selectedOverride := lo.MinBy(ltConfig.Overrides, func(a, b ec2types.FleetLaunchTemplateOverridesRequest) bool {
			var capacityType string
			switch input.TargetCapacitySpecification.DefaultTargetCapacityType {
			case ec2types.DefaultTargetCapacityTypeSpot:
				capacityType = v1.CapacityTypeSpot
			case ec2types.DefaultTargetCapacityTypeOnDemand:
				capacityType = v1.CapacityTypeOnDemand
			default:
				panic(fmt.Sprintf("unknown target capacity type: %v", input.TargetCapacitySpecification.DefaultTargetCapacityType))
			}

			var aScore, bScore float64
			if subnet, subnetOk := lo.Find(c.subnets, func(s ec2types.Subnet) bool {
				return lo.FromPtr(s.SubnetId) == lo.FromPtr(a.SubnetId)
			}); subnetOk {
				aScore = c.strategy.GetScore(string(a.InstanceType), capacityType, lo.FromPtr(subnet.AvailabilityZone))
			}
			if subnet, subnetOk := lo.Find(c.subnets, func(s ec2types.Subnet) bool {
				return lo.FromPtr(s.SubnetId) == lo.FromPtr(b.SubnetId)
			}); subnetOk {
				bScore = c.strategy.GetScore(string(b.InstanceType), capacityType, lo.FromPtr(subnet.AvailabilityZone))
			}
			if lo.IsEmpty(bScore) {
				return true
			}
			if lo.IsEmpty(aScore) {
				return false
			}
			return aScore < bScore
		})
		instanceTags, _ := lo.Find(lt.B.TagSpecifications, func(t ec2types.LaunchTemplateTagSpecificationRequest) bool {
			return t.ResourceType == ec2types.ResourceTypeInstance
		})
		subnet, ok := lo.Find(c.subnets, func(s ec2types.Subnet) bool {
			return lo.FromPtr(s.SubnetId) == lo.FromPtr(selectedOverride.SubnetId)
		})
		if !ok {
			return nil, fmt.Errorf("subnet %q not found", lo.FromPtr(selectedOverride.SubnetId))
		}
		instanceTypeInfo := lo.Must(lo.Find(c.instanceTypes, func(i ec2types.InstanceTypeInfo) bool {
			return i.InstanceType == selectedOverride.InstanceType
		}))
		// TODO: We need to get the capacity and allocatable information from the userData
		it := instancetype.NewInstanceType(
			ctx,
			instanceTypeInfo,
			c.region,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			// TODO: Eventually support different AMIFamilies from userData
			"al2023",
			nil,
		)
		instance := ec2types.Instance{
			AmiLaunchIndex: nil,
			Architecture:   lo.Ternary(it.Requirements.Get(corev1.LabelArchStable).Any() == v1.ArchitectureAmd64, ec2types.ArchitectureValuesX8664, ec2types.ArchitectureValuesArm64),
			// TODO: The block device mappings here don't have any data on the ephemeral storage size
			BlockDeviceMappings: lo.Map(lt.B.BlockDeviceMappings, func(b ec2types.LaunchTemplateBlockDeviceMappingRequest, _ int) ec2types.InstanceBlockDeviceMapping {
				return ec2types.InstanceBlockDeviceMapping{
					DeviceName: b.DeviceName,
					Ebs: &ec2types.EbsInstanceBlockDevice{
						AssociatedResource:  nil,
						AttachTime:          nil,
						DeleteOnTermination: b.Ebs.DeleteOnTermination,
						Operator:            nil,
						Status:              ec2types.AttachmentStatusAttached,
						VolumeId:            lo.ToPtr(fmt.Sprintf("vol-%s", randomdata.Alphanumeric(17))),
						VolumeOwnerId:       nil,
					},
				}
			}),
			BootMode: ec2types.BootModeValuesUefi,
			// Don't support ODCR yet
			CapacityReservationId:                   nil,
			CapacityReservationSpecification:        nil,
			ClientToken:                             nil,
			CpuOptions:                              nil,
			CurrentInstanceBootMode:                 ec2types.InstanceBootModeValuesUefi,
			EbsOptimized:                            lo.ToPtr(true),
			ElasticGpuAssociations:                  nil,
			ElasticInferenceAcceleratorAssociations: nil,
			EnaSupport:                              lo.ToPtr(false),
			EnclaveOptions:                          nil,
			HibernationOptions:                      nil,
			Hypervisor:                              ec2types.HypervisorTypeXen,
			IamInstanceProfile: &ec2types.IamInstanceProfile{
				Arn: lt.B.IamInstanceProfile.Arn,
				Id:  lt.B.IamInstanceProfile.Name,
			},
			ImageId:    selectedOverride.ImageId,
			InstanceId: lo.ToPtr(fmt.Sprintf("i-%s", randomdata.Alphanumeric(17))),
			// TODO: Eventually handle LifecycleCapacityBlock
			InstanceLifecycle:  lo.Ternary(input.TargetCapacitySpecification.DefaultTargetCapacityType == ec2types.DefaultTargetCapacityTypeSpot, ec2types.InstanceLifecycleTypeSpot, ec2types.InstanceLifecycleTypeScheduled),
			InstanceType:       selectedOverride.InstanceType,
			Ipv6Address:        nil,
			KernelId:           nil,
			KeyName:            nil,
			LaunchTime:         lo.ToPtr(c.clock.Now()),
			Licenses:           nil,
			MaintenanceOptions: nil,
			MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
				HttpEndpoint:            ec2types.InstanceMetadataEndpointState(lt.B.MetadataOptions.HttpEndpoint),
				HttpProtocolIpv6:        ec2types.InstanceMetadataProtocolState(lt.B.MetadataOptions.HttpProtocolIpv6),
				HttpPutResponseHopLimit: lt.B.MetadataOptions.HttpPutResponseHopLimit,
				HttpTokens:              ec2types.HttpTokensState(lt.B.MetadataOptions.HttpTokens),
				InstanceMetadataTags:    ec2types.InstanceMetadataTagsState(lt.B.MetadataOptions.InstanceMetadataTags),
				State:                   ec2types.InstanceMetadataOptionsStateApplied,
			},
			Monitoring: &ec2types.Monitoring{
				State: lo.Ternary(lo.FromPtr(lt.B.Monitoring.Enabled), ec2types.MonitoringStateEnabled, ec2types.MonitoringStateDisabled),
			},
			// TODO: We may need to auto-gen these network interfaces too
			// TODO: We should eventually pass the network interfaces from the launch template
			NetworkInterfaces:         nil,
			NetworkPerformanceOptions: nil,
			Operator:                  nil,
			OutpostArn:                nil,
			Placement: &ec2types.Placement{
				Affinity:             nil,
				AvailabilityZone:     subnet.AvailabilityZone,
				GroupId:              nil,
				GroupName:            nil,
				HostId:               nil,
				HostResourceGroupArn: nil,
				PartitionNumber:      nil,
				SpreadDomain:         nil,
				Tenancy:              "",
			},
			Platform:        "",
			PlatformDetails: nil,
			// TODO: We may eventually need to fill-in this private DNS name
			PrivateDnsName:        nil,
			PrivateDnsNameOptions: nil,
			// TODO: We may eventually need to fill-in this private IP address
			PrivateIpAddress: nil,
			ProductCodes:     nil,
			// TODO: We may eventually need to fill-in this public DNS name
			PublicDnsName: nil,
			// TODO: We may eventually need to fill-in this public IP address
			PublicIpAddress: nil,
			RamdiskId:       nil,
			RootDeviceName:  nil,
			RootDeviceType:  ec2types.DeviceTypeEbs,
			// TODO: Pull the security groups from passed-through network interfaces
			// If we don't specify network interfaces directly, we just get it from the SecurityGroupIDs in the LT
			SecurityGroups: lo.Map(lo.Ternary(len(lt.B.NetworkInterfaces) == 0, lt.B.SecurityGroupIds, lt.B.NetworkInterfaces[0].Groups), func(s string, _ int) ec2types.GroupIdentifier {
				return ec2types.GroupIdentifier{
					GroupId: lo.ToPtr(s),
				}
			}),
			SourceDestCheck:       nil,
			SpotInstanceRequestId: lo.Ternary(input.TargetCapacitySpecification.DefaultTargetCapacityType == ec2types.DefaultTargetCapacityTypeSpot, lo.ToPtr(fmt.Sprintf("spot-%s", randomdata.Alphanumeric(17))), nil),
			SriovNetSupport:       nil,
			State: &ec2types.InstanceState{
				Code: lo.ToPtr[int32](16),
				Name: ec2types.InstanceStateNameRunning,
			},
			// TODO: We may eventually need to fill this in
			StateReason: nil,
			// TODO: We may eventually need to fill this in
			StateTransitionReason:    nil,
			SubnetId:                 selectedOverride.SubnetId,
			Tags:                     instanceTags.Tags,
			TpmSupport:               nil,
			UsageOperation:           nil,
			UsageOperationUpdateTime: nil,
			VirtualizationType:       ec2types.VirtualizationTypeHvm,
			VpcId:                    subnet.VpcId,
		}
		c.instances.Store(lo.FromPtr(instance.InstanceId), instance)
		launchCtx, cancel := context.WithCancel(ctx)
		c.instanceLaunchCancel.Store(lo.FromPtr(instance.InstanceId), cancel)

		// Create the Node through the instance launch
		// TODO: Eventually support delayed registration
		nodePoolNameTag, _ := lo.Find(instance.Tags, func(t ec2types.Tag) bool {
			return lo.FromPtr(t.Key) == v1.NodePoolLabelKey
		})
		go func() {
			select {
			case <-launchCtx.Done():
				return
			// This is meant to simulate instance startup time
			case <-c.clock.After(30 * time.Second):
			}
			if err := c.kubeClient.Create(ctx, toNode(lo.FromPtr(instance.InstanceId), lo.FromPtr(nodePoolNameTag.Value), it, lo.FromPtr(subnet.AvailabilityZone), v1.CapacityTypeOnDemand)); err != nil {
				log.FromContext(ctx).Error(err, "%q node creation failed", lo.FromPtr(instance.InstanceId))
				c.instances.Delete(lo.FromPtr(instance.InstanceId))
				c.instanceLaunchCancel.Delete(lo.FromPtr(instance.InstanceId))
			}
		}()
		fleetInstances = append(fleetInstances, ec2types.CreateFleetInstance{
			InstanceIds:  []string{lo.FromPtr(instance.InstanceId)},
			InstanceType: instance.InstanceType,
			LaunchTemplateAndOverrides: &ec2types.LaunchTemplateAndOverridesResponse{
				LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecification{
					LaunchTemplateId:   lt.A.LaunchTemplateId,
					LaunchTemplateName: lt.A.LaunchTemplateName,
					Version:            lo.ToPtr(fmt.Sprint(lo.FromPtr(lt.A.LatestVersionNumber))),
				},
				Overrides: &ec2types.FleetLaunchTemplateOverrides{
					AvailabilityZone:    subnet.AvailabilityZone,
					BlockDeviceMappings: nil, // For now, we don't support blockDeviceMapping overrides
					ImageId:             selectedOverride.ImageId,
					InstanceType:        lt.B.InstanceType,
					MaxPrice:            selectedOverride.MaxPrice,
					Placement:           nil,
					Priority:            selectedOverride.Priority,
					SubnetId:            selectedOverride.SubnetId,
					WeightedCapacity:    selectedOverride.WeightedCapacity,
				},
			},
			Lifecycle: lo.Ternary(instance.InstanceLifecycle == ec2types.InstanceLifecycleTypeSpot, ec2types.InstanceLifecycleSpot, ec2types.InstanceLifecycleOnDemand),
		})
	}
	return &ec2.CreateFleetOutput{
		// TODO: We can eventually send back ICE errors through this section
		Errors:    nil,
		FleetId:   lo.ToPtr(fmt.Sprintf("fleet-%s", randomdata.Alphanumeric(17))),
		Instances: fleetInstances,
	}, nil
}

func (c *Client) TerminateInstances(_ context.Context, input *ec2.TerminateInstancesInput, _ ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	if !c.rateLimiterProvider.TerminateInstances().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request was rate limited",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	for _, id := range input.InstanceIds {
		c.instances.Delete(id)
		if cancel, ok := c.instanceLaunchCancel.LoadAndDelete(id); ok {
			cancel.(context.CancelFunc)()
		}
	}
	return &ec2.TerminateInstancesOutput{
		TerminatingInstances: lo.Map(input.InstanceIds, func(id string, _ int) ec2types.InstanceStateChange {
			return ec2types.InstanceStateChange{
				CurrentState: &ec2types.InstanceState{
					Code: lo.ToPtr[int32](48),
					Name: ec2types.InstanceStateNameTerminated,
				},
				InstanceId: lo.ToPtr(id),
				PreviousState: &ec2types.InstanceState{
					Code: lo.ToPtr[int32](16),
					Name: ec2types.InstanceStateNameRunning,
				},
			}
		}),
	}, nil
}

func (c *Client) DescribeInstances(_ context.Context, input *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if !c.rateLimiterProvider.DescribeInstances().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	// TODO: Eventually we can consider supporting DescribeInstances filters
	var instances []ec2types.Instance
	if len(input.InstanceIds) > 0 {
		for _, id := range input.InstanceIds {
			raw, ok := c.instances.Load(id)
			if !ok {
				return nil, &smithy.GenericAPIError{
					Code: "InvalidInstanceID.NotFound",
					// TODO: we can eventually expand this to list out every id
					Message: fmt.Sprintf("The instance IDs '%s' do not exist", id),
				}
			}
			instances = append(instances, raw.(ec2types.Instance))
		}
	} else {
		c.instances.Range(func(k, v interface{}) bool {
			instances = append(instances, v.(ec2types.Instance))
			return true
		})
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{
				Groups:    nil,
				Instances: instances,
				// TODO: Consider adding these values but they aren't necessary
				OwnerId:       nil,
				RequesterId:   nil,
				ReservationId: nil,
			},
		},
	}, nil
}

func (c *Client) RunInstances(_ context.Context, input *ec2.RunInstancesInput, _ ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	if !c.rateLimiterProvider.RunInstances().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	// TODO: Implement RunInstances completely
	// For now, this is only used for validation
	panic("implement me")
}

//nolint:gocyclo
func (c *Client) CreateTags(_ context.Context, input *ec2.CreateTagsInput, _ ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	if !c.rateLimiterProvider.CreateTags().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	for _, resource := range input.Resources {
		switch {
		case strings.Contains(resource, "i-"):
			raw, ok := c.instances.Load(resource)
			if !ok {
				// For now, we just ignore if the resource doesn't exist
				continue
			}
			instance := raw.(ec2types.Instance)
			instance.Tags = lo.Reject(instance.Tags, func(t ec2types.Tag, _ int) bool {
				for _, tag := range instance.Tags {
					if tag.Key == t.Key {
						return true
					}
				}
				return false
			})
			instance.Tags = append(instance.Tags, input.Tags...)
			c.instances.Store(resource, instance)
		case strings.Contains(resource, "lt-"):
			raw, ok := c.launchTemplates.Load(resource)
			if !ok {
				// For now, we just ignore if the resource doesn't exist
				continue
			}
			lt := raw.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
			lt.A.Tags = lo.Reject(lt.A.Tags, func(t ec2types.Tag, _ int) bool {
				for _, tag := range lt.A.Tags {
					if tag.Key == t.Key {
						return true
					}
				}
				return false
			})
			lt.A.Tags = append(lt.A.Tags, input.Tags...)
			c.launchTemplates.Store(resource, lt)
		default:
			return nil, fmt.Errorf("unknown resource type %q", resource)
		}
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (c *Client) CreateLaunchTemplate(_ context.Context, input *ec2.CreateLaunchTemplateInput, _ ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error) {
	if !c.rateLimiterProvider.CreateLaunchTemplate().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	launchTemplateID := fmt.Sprintf("lt-%s", randomdata.Alphanumeric(17))
	ltTags, _ := lo.Find(input.TagSpecifications, func(t ec2types.TagSpecification) bool {
		return t.ResourceType == ec2types.ResourceTypeLaunchTemplate
	})
	lt := &ec2types.LaunchTemplate{
		CreateTime:           lo.ToPtr(c.clock.Now()),
		DefaultVersionNumber: lo.ToPtr[int64](0),
		LatestVersionNumber:  lo.ToPtr[int64](0),
		LaunchTemplateId:     lo.ToPtr(launchTemplateID),
		LaunchTemplateName:   input.LaunchTemplateName,
		Tags:                 ltTags.Tags,
	}
	c.launchTemplates.Store(launchTemplateID, lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData]{A: lt, B: input.LaunchTemplateData})
	c.launchTemplateNameToID.Store(lo.FromPtr(input.LaunchTemplateName), launchTemplateID)
	return &ec2.CreateLaunchTemplateOutput{LaunchTemplate: lt}, nil
}

func (c *Client) DeleteLaunchTemplate(_ context.Context, input *ec2.DeleteLaunchTemplateInput, _ ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error) {
	if !c.rateLimiterProvider.DeleteLaunchTemplate().TryAccept() {
		return nil, &smithy.GenericAPIError{
			Code:    errors.RateLimitingErrorCode,
			Message: "Request limit exceeded.",
		}
	}
	// TODO: Eventually do more rigorous validations and auth checks for dry-run
	if lo.FromPtr(input.DryRun) {
		return nil, &smithy.GenericAPIError{
			Code:    errors.DryRunOperationErrorCode,
			Message: "Request would have succeeded, but DryRun flag is set",
		}
	}

	launchTemplateID := lo.FromPtr(input.LaunchTemplateId)
	if input.LaunchTemplateName != nil {
		raw, ok := c.launchTemplateNameToID.Load(lo.FromPtr(input.LaunchTemplateName))
		if !ok {
			return nil, &smithy.GenericAPIError{
				Code:    "InvalidLaunchTemplateName.NotFoundException",
				Message: fmt.Sprintf("The specified launch template, with template name %s, does not exist.", lo.FromPtr(input.LaunchTemplateName)),
			}
		}
		launchTemplateID = raw.(string)
	}
	raw, ok := c.launchTemplates.LoadAndDelete(launchTemplateID)
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "InvalidLaunchTemplateId.NotFoundException",
			Message: fmt.Sprintf("The specified launch template, with template id %s, does not exist.", launchTemplateID),
		}
	}
	lt := raw.(lo.Tuple2[*ec2types.LaunchTemplate, *ec2types.RequestLaunchTemplateData])
	c.launchTemplateNameToID.Delete(lo.FromPtr(lt.A.LaunchTemplateName))
	return &ec2.DeleteLaunchTemplateOutput{
		LaunchTemplate: lt.A,
	}, nil
}

func toNode(instanceID, nodePoolName string, instanceType *cloudprovider.InstanceType, zone, capacityType string) *corev1.Node {
	nodeName := fmt.Sprintf("%s-%d", strings.ReplaceAll(namesgenerator.GetRandomName(0), "_", "-"), rand.Uint32()) //nolint:gosec
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Annotations: map[string]string{
				v1alpha1.KwokLabelKey: v1alpha1.KwokLabelValue,
			},
			// TODO: We can eventually add all the labels from the userData but for now we just add the NodePool labels
			Labels: map[string]string{
				corev1.LabelInstanceTypeStable: instanceType.Name,
				corev1.LabelHostname:           nodeName,
				corev1.LabelTopologyRegion:     instanceType.Requirements.Get(corev1.LabelTopologyRegion).Any(),
				corev1.LabelTopologyZone:       zone,
				v1.CapacityTypeLabelKey:        capacityType,
				corev1.LabelArchStable:         instanceType.Requirements.Get(corev1.LabelArchStable).Any(),
				corev1.LabelOSStable:           string(corev1.Linux),
				v1.NodePoolLabelKey:            nodePoolName,
				v1alpha1.KwokLabelKey:          v1alpha1.KwokLabelValue,
				v1alpha1.KwokPartitionLabelKey: "a",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: fmt.Sprintf("kwok-aws:///%s/%s", zone, instanceID),
			Taints:     []corev1.Taint{v1.UnregisteredNoExecuteTaint},
		},
		Status: corev1.NodeStatus{
			Capacity:    instanceType.Capacity,
			Allocatable: instanceType.Allocatable(),
			Phase:       corev1.NodePending,
		},
	}
}
