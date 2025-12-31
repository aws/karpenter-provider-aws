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
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/aws/aws-sdk-go-v2/service/fis"
	fistypes "github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Spot Interruption experiment details partially copied from
// https://github.com/aws/amazon-ec2-spot-interrupter/blob/main/pkg/itn/itn.go
const (
	fisRoleName    = "FISInterruptionRole"
	fisTargetLimit = 5
	spotITNAction  = "aws:ec2:send-spot-instance-interruptions"
)

func (env *Environment) ExpectWindowsIPAMEnabled() {
	GinkgoHelper()
	env.ExpectConfigMapDataOverridden(types.NamespacedName{Namespace: "kube-system", Name: "amazon-vpc-cni"}, map[string]string{
		"enable-windows-ipam": "true",
	})
}

func (env *Environment) ExpectWindowsIPAMDisabled() {
	GinkgoHelper()
	env.ExpectConfigMapDataOverridden(types.NamespacedName{Namespace: "kube-system", Name: "amazon-vpc-cni"}, map[string]string{
		"enable-windows-ipam": "false",
	})
}

func (env *Environment) ExpectInstance(nodeName string) Assertion {
	return Expect(env.GetInstance(nodeName))
}

func (env *Environment) ExpectIPv6ClusterDNS() string {
	dnsService, err := env.Environment.KubeClient.CoreV1().Services("kube-system").Get(env.Context, "kube-dns", metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	kubeDNSIP := net.ParseIP(dnsService.Spec.ClusterIP)
	Expect(kubeDNSIP.To4()).To(BeNil())
	return kubeDNSIP.String()
}

func (env *Environment) ExpectSpotInterruptionExperiment(instanceIDs ...string) *fistypes.Experiment {
	GinkgoHelper()
	template := &fis.CreateExperimentTemplateInput{
		Actions:        map[string]fistypes.CreateExperimentTemplateActionInput{},
		Targets:        map[string]fistypes.CreateExperimentTemplateTargetInput{},
		StopConditions: []fistypes.CreateExperimentTemplateStopConditionInput{{Source: aws.String("none")}},
		RoleArn:        env.ExpectSpotInterruptionRole().Arn,
		Description:    aws.String(fmt.Sprintf("trigger spot ITN for instances %v", instanceIDs)),
	}
	for j, ids := range lo.Chunk(instanceIDs, fisTargetLimit) {
		key := fmt.Sprintf("itn%d", j)
		template.Actions[key] = fistypes.CreateExperimentTemplateActionInput{
			ActionId: aws.String(spotITNAction),
			Parameters: map[string]string{
				// durationBeforeInterruption is the time before the instance is terminated, so we add 2 minutes
				"durationBeforeInterruption": "PT120S",
			},
			Targets: map[string]string{"SpotInstances": key},
		}
		template.Targets[key] = fistypes.CreateExperimentTemplateTargetInput{
			ResourceType:  aws.String("aws:ec2:spot-instance"),
			SelectionMode: aws.String("ALL"),
			ResourceArns: lo.Map(ids, func(id string, _ int) string {
				return fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", env.Region, env.ExpectAccountID(), id)
			}),
		}
	}
	experimentTemplate, err := env.FISAPI.CreateExperimentTemplate(env.Context, template)
	Expect(err).ToNot(HaveOccurred())
	experiment, err := env.FISAPI.StartExperiment(env.Context, &fis.StartExperimentInput{ExperimentTemplateId: experimentTemplate.ExperimentTemplate.Id})
	Expect(err).ToNot(HaveOccurred())
	return experiment.Experiment
}

func (env *Environment) ExpectExperimentTemplateDeleted(id string) {
	GinkgoHelper()
	_, err := env.FISAPI.DeleteExperimentTemplate(env.Context, &fis.DeleteExperimentTemplateInput{
		Id: aws.String(id),
	})
	Expect(err).ToNot(HaveOccurred())
}

func (env *Environment) EventuallyExpectInstanceProfileExists(profileName string) iamtypes.InstanceProfile {
	GinkgoHelper()
	By(fmt.Sprintf("eventually expecting instance profile %s to exist", profileName))
	var instanceProfile iamtypes.InstanceProfile
	Eventually(func(g Gomega) {
		out, err := env.IAMAPI.GetInstanceProfile(env.Context, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(out.InstanceProfile).ToNot(BeNil())
		g.Expect(out.InstanceProfile.InstanceProfileName).ToNot(BeNil())
		instanceProfile = lo.FromPtr(out.InstanceProfile)
	}).WithTimeout(20 * time.Second).Should(Succeed())
	return instanceProfile
}

func (env *Environment) EventuallyExpectInstanceProfilesNotFound(profileNames ...string) {
	GinkgoHelper()
	By(fmt.Sprintf("expecting instance profiles %v to not exist", profileNames))
	Eventually(func(g Gomega) {
		for _, profileName := range profileNames {
			_, err := env.IAMAPI.GetInstanceProfile(env.Context, &iam.GetInstanceProfileInput{
				InstanceProfileName: aws.String(profileName),
			})
			g.Expect(awserrors.IsNotFound(err)).To(BeTrue())
		}
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

func (env *Environment) GetInstance(nodeName string) ec2types.Instance {
	node := env.GetNode(nodeName)
	return env.GetInstanceByID(env.ExpectParsedProviderID(node.Spec.ProviderID))
}

func (env *Environment) ExpectInstanceStopped(nodeName string) {
	GinkgoHelper()
	node := env.GetNode(nodeName)
	_, err := env.EC2API.StopInstances(env.Context, &ec2.StopInstancesInput{
		Force:       aws.Bool(true),
		InstanceIds: []string{env.ExpectParsedProviderID(node.Spec.ProviderID)},
	})
	Expect(err).To(Succeed())
}

func (env *Environment) ExpectInstanceTerminated(nodeName string) {
	GinkgoHelper()
	node := env.GetNode(nodeName)
	_, err := env.EC2API.TerminateInstances(env.Context, &ec2.TerminateInstancesInput{
		InstanceIds: []string{env.ExpectParsedProviderID(node.Spec.ProviderID)},
	})
	Expect(err).To(Succeed())
}

func (env *Environment) GetInstanceByID(instanceID string) ec2types.Instance {
	GinkgoHelper()
	instance, err := env.EC2API.DescribeInstances(env.Context, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(instance.Reservations).To(HaveLen(1))
	Expect(instance.Reservations[0].Instances).To(HaveLen(1))
	return instance.Reservations[0].Instances[0]
}

func (env *Environment) GetVolume(id string) ec2types.Volume {
	volumes := env.GetVolumes(id)
	Expect(volumes).To(HaveLen(1))
	return volumes[0]
}

func (env *Environment) GetVolumes(ids ...string) []ec2types.Volume {
	GinkgoHelper()
	dvo, err := env.EC2API.DescribeVolumes(env.Context, &ec2.DescribeVolumesInput{VolumeIds: ids})
	Expect(err).ToNot(HaveOccurred())

	return dvo.Volumes
}

func (env *Environment) GetNetworkInterface(id string) ec2types.NetworkInterface {
	networkInterfaces := env.GetNetworkInterfaces(id)
	Expect(networkInterfaces).To(HaveLen(1))
	return networkInterfaces[0]
}

func (env *Environment) GetNetworkInterfaces(ids ...string) []ec2types.NetworkInterface {
	GinkgoHelper()
	dnio, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: ids})
	Expect(err).ToNot(HaveOccurred())
	return dnio.NetworkInterfaces
}

func (env *Environment) GetSpotInstance(id string) ec2types.SpotInstanceRequest {
	GinkgoHelper()
	siro, err := env.EC2API.DescribeSpotInstanceRequests(env.Context, &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []string{id},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(siro.SpotInstanceRequests).To(HaveLen(1))
	return siro.SpotInstanceRequests[0]
}

// GetSubnets returns all subnets matching the label selector
// mapped from AZ -> {subnet-ids...}
func (env *Environment) GetSubnets(tags map[string]string) map[string][]string {
	var filters []ec2types.Filter
	for key, val := range tags {
		filters = append(filters, ec2types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []string{val},
		})
	}
	subnets := map[string][]string{}
	input := &ec2.DescribeSubnetsInput{
		Filters: filters,
	}

	paginator := ec2.NewDescribeSubnetsPaginator(env.EC2API, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(env.Context)
		if err != nil {
			Expect(err).To(BeNil())
		}

		for _, subnet := range output.Subnets {
			subnets[*subnet.AvailabilityZone] = append(subnets[*subnet.AvailabilityZone], *subnet.SubnetId)
		}
	}

	return subnets
}

// SubnetInfo is a simple struct for testing
type SubnetInfo struct {
	Name string
	ID   string
	ZoneInfo
}

// GetSubnetInfo returns all subnets matching the label selector
func (env *Environment) GetSubnetInfo(tags map[string]string) []SubnetInfo {
	var filters []ec2types.Filter
	for key, val := range tags {
		filters = append(filters, ec2types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []string{val},
		})
	}
	var subnetInfo []SubnetInfo
	input := &ec2.DescribeSubnetsInput{
		Filters: filters,
	}

	paginator := ec2.NewDescribeSubnetsPaginator(env.EC2API, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(env.Context)
		if err != nil {
			Expect(err).To(BeNil())
		}

		subnetInfo = append(subnetInfo, lo.Map(output.Subnets, func(s ec2types.Subnet, _ int) SubnetInfo {
			elem := SubnetInfo{ID: aws.ToString(s.SubnetId)}
			if tag, ok := lo.Find(s.Tags, func(t ec2types.Tag) bool { return aws.ToString(t.Key) == "Name" }); ok {
				elem.Name = aws.ToString(tag.Value)
			}
			if info, ok := lo.Find(env.ZoneInfo, func(info ZoneInfo) bool {
				return aws.ToString(s.AvailabilityZone) == info.Zone
			}); ok {
				elem.ZoneInfo = info
			}
			return elem
		})...)
	}

	return subnetInfo
}

type SecurityGroup struct {
	ec2types.GroupIdentifier
	Tags []ec2types.Tag
}

// GetSecurityGroups returns all getSecurityGroups matching the label selector
func (env *Environment) GetSecurityGroups(tags map[string]string) []SecurityGroup {
	var filters []ec2types.Filter
	for key, val := range tags {
		filters = append(filters, ec2types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []string{val},
		})
	}
	var securityGroups []SecurityGroup
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}

	paginator := ec2.NewDescribeSecurityGroupsPaginator(env.EC2API, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(env.Context)
		if err != nil {
			Expect(err).To(BeNil())
		}

		for _, sg := range output.SecurityGroups {
			securityGroups = append(securityGroups, SecurityGroup{
				Tags:            sg.Tags,
				GroupIdentifier: ec2types.GroupIdentifier{GroupId: sg.GroupId, GroupName: sg.GroupName},
			})
		}
	}

	return securityGroups
}

func (env *Environment) ExpectMessagesCreated(msgs ...any) {
	GinkgoHelper()
	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	var err error
	for _, msg := range msgs {
		wg.Add(1)
		go func(m any) {
			defer wg.Done()
			defer GinkgoRecover()
			_, e := env.SQSProvider.SendMessage(env.Context, m)
			if e != nil {
				mu.Lock()
				err = multierr.Append(err, e)
				mu.Unlock()
			}
		}(msg)
	}
	wg.Wait()
	Expect(err).To(Succeed())
}

func (env *Environment) ExpectParsedProviderID(providerID string) string {
	GinkgoHelper()
	providerIDSplit := strings.Split(providerID, "/")
	Expect(len(providerIDSplit)).ToNot(Equal(0))
	return providerIDSplit[len(providerIDSplit)-1]
}

func (env *Environment) K8sVersion() string {
	GinkgoHelper()

	return env.K8sVersionWithOffset(0)
}

func (env *Environment) K8sVersionWithOffset(offset int) string {
	GinkgoHelper()

	serverVersion, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	minorVersion, err := strconv.Atoi(strings.TrimSuffix(serverVersion.Minor, "+"))
	Expect(err).To(BeNil())
	// Choose a minor version one lesser than the server's minor version. This ensures that we choose an AMI for
	// this test that wouldn't be selected as Karpenter's SSM default (therefore avoiding false positives), and also
	// ensures that we aren't violating version skew.
	return fmt.Sprintf("%s.%d", serverVersion.Major, minorVersion-offset)
}

func (env *Environment) K8sMinorVersion() int {
	GinkgoHelper()

	version, err := strconv.Atoi(strings.Split(env.K8sVersion(), ".")[1])
	Expect(err).ToNot(HaveOccurred())
	return version
}

func (env *Environment) GetAMIBySSMPath(ssmPath string) string {
	GinkgoHelper()

	parameter, err := env.SSMAPI.GetParameter(env.Context, &ssm.GetParameterInput{
		Name: aws.String(ssmPath),
	})
	Expect(err).To(BeNil())
	return *parameter.Parameter.Value
}

func (env *Environment) GetDeprecatedAMI(amiID string, amifamily string) string {
	out, err := env.EC2API.DescribeImages(env.Context, &ec2.DescribeImagesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr(fmt.Sprintf("tag:%s", coretest.DiscoveryLabel)),
				Values: []string{env.K8sVersion()},
			},
			{
				Name:   lo.ToPtr("tag:amiFamily"),
				Values: []string{amifamily},
			},
		},
		IncludeDeprecated: lo.ToPtr(true),
	})
	Expect(err).To(BeNil())
	if len(out.Images) == 1 {
		return lo.FromPtr(out.Images[0].ImageId)
	}

	input := &ec2.CopyImageInput{
		SourceImageId: lo.ToPtr(amiID),
		Name:          lo.ToPtr(fmt.Sprintf("deprecated-%s-%s-%s", amiID, amifamily, env.K8sVersion())),
		SourceRegion:  lo.ToPtr(env.Region),
		TagSpecifications: []ec2types.TagSpecification{
			{ResourceType: ec2types.ResourceTypeImage, Tags: []ec2types.Tag{
				{
					Key:   lo.ToPtr(coretest.DiscoveryLabel),
					Value: lo.ToPtr(env.K8sVersion()),
				},
				{
					Key:   lo.ToPtr("amiFamily"),
					Value: lo.ToPtr(amifamily),
				},
			}},
		},
	}
	output, err := env.EC2API.CopyImage(env.Context, input)
	Expect(err).To(BeNil())

	deprecated, err := env.EC2API.EnableImageDeprecation(env.Context, &ec2.EnableImageDeprecationInput{
		ImageId:     output.ImageId,
		DeprecateAt: lo.ToPtr(time.Now()),
	})
	Expect(err).To(BeNil())
	Expect(lo.FromPtr(deprecated.Return)).To(BeTrue())

	return lo.FromPtr(output.ImageId)
}

func (env *Environment) EventuallyExpectRunInstances(instanceInput *ec2.RunInstancesInput) ec2types.Reservation {
	GinkgoHelper()
	// implement IMDSv2
	instanceInput.MetadataOptions = &ec2types.InstanceMetadataOptionsRequest{
		HttpEndpoint: "enabled",
		HttpTokens:   "required",
	}
	var reservation ec2types.Reservation
	Eventually(func(g Gomega) {
		out, err := env.EC2API.RunInstances(env.Context, instanceInput)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(out.Instances).ToNot(BeEmpty())
		reservation = ec2types.Reservation{
			Instances: out.Instances,
		}
	}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
	return reservation
}

func (env *Environment) ExpectSpotInterruptionRole() *iamtypes.Role {
	GinkgoHelper()
	out, err := env.IAMAPI.GetRole(env.Context, &iam.GetRoleInput{
		RoleName: aws.String(fisRoleName),
	})
	Expect(err).ToNot(HaveOccurred())
	return out.Role
}

func (env *Environment) ExpectAccountID() string {
	GinkgoHelper()
	identity, err := env.STSAPI.GetCallerIdentity(env.Context, &sts.GetCallerIdentityInput{})
	Expect(err).ToNot(HaveOccurred())
	return aws.ToString(identity.Account)
}

func (env *Environment) ExpectInstanceProfileCreated(instanceProfileName, roleName string) {
	By("creating an instance profile")
	createInstanceProfile := &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		Tags: []iamtypes.Tag{
			{
				Key:   aws.String(coretest.DiscoveryLabel),
				Value: aws.String(env.ClusterName),
			},
		},
	}
	By("adding the karpenter role to new instance profile")
	_, err := env.IAMAPI.CreateInstanceProfile(env.Context, createInstanceProfile)
	Expect(awserrors.IgnoreAlreadyExists(err)).ToNot(HaveOccurred())
	addInstanceProfile := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            aws.String(roleName),
	}
	_, err = env.IAMAPI.AddRoleToInstanceProfile(env.Context, addInstanceProfile)
	Expect(ignoreAlreadyContainsRole(err)).ToNot(HaveOccurred())
}

func (env *Environment) ExpectInstanceProfileDeleted(instanceProfileName, roleName string) {
	By("deleting an instance profile")
	removeRoleFromInstanceProfile := &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            aws.String(roleName),
	}
	_, err := env.IAMAPI.RemoveRoleFromInstanceProfile(env.Context, removeRoleFromInstanceProfile)
	Expect(awserrors.IgnoreNotFound(err)).To(BeNil())

	deleteInstanceProfile := &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	}
	_, err = env.IAMAPI.DeleteInstanceProfile(env.Context, deleteInstanceProfile)
	Expect(awserrors.IgnoreNotFound(err)).ToNot(HaveOccurred())
}

func ignoreAlreadyContainsRole(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "Cannot exceed quota for InstanceSessionsPerInstanceProfile") {
			return nil
		}
	}
	return err
}

func ExpectCapacityReservationCreated(
	ctx context.Context,
	ec2api *ec2.Client,
	instanceType ec2types.InstanceType,
	zone string,
	capacity int32,
	endDate *time.Time,
	tags map[string]string,
) string {
	GinkgoHelper()
	out, err := ec2api.CreateCapacityReservation(ctx, &ec2.CreateCapacityReservationInput{
		InstanceCount:         lo.ToPtr(capacity),
		InstanceType:          lo.ToPtr(string(instanceType)),
		InstancePlatform:      ec2types.CapacityReservationInstancePlatformLinuxUnix,
		AvailabilityZone:      lo.ToPtr(zone),
		EndDate:               endDate,
		InstanceMatchCriteria: ec2types.InstanceMatchCriteriaTargeted,
		TagSpecifications: lo.Ternary(len(tags) != 0, []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeCapacityReservation,
			Tags:         utils.EC2MergeTags(tags),
		}}, nil),
	})
	Expect(err).ToNot(HaveOccurred())
	return *out.CapacityReservation.CapacityReservationId
}

func ExpectCapacityReservationsCanceled(ctx context.Context, ec2api *ec2.Client, reservationIDs ...string) {
	GinkgoHelper()
	for _, id := range reservationIDs {
		_, err := ec2api.CancelCapacityReservation(ctx, &ec2.CancelCapacityReservationInput{
			CapacityReservationId: &id,
		})
		Expect(err).ToNot(HaveOccurred())
	}
}

// Creates a role with the provided name. The appropriate policies and trust policy are configured to ensure nodes with
// this role may join the cluster. Additionally, an AccessEntry is created for the role to ensure nodes are authorized
// to join the cluster.
func (env *Environment) EventuallyExpectNodeRoleCreated(roleName string) {
	GinkgoHelper()
	if _, err := env.IAMAPI.GetRole(env.Context, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	}); err == nil {
		return
	}

	// Create new role with trust policy
	trustPolicy := `{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }`

	// Create new role with same trust policy
	_, err := env.IAMAPI.CreateRole(env.Context, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
		Tags: []iamtypes.Tag{
			{
				Key:   aws.String(coretest.DiscoveryLabel),
				Value: aws.String(env.ClusterName),
			},
		},
	})
	Expect(awserrors.IgnoreAlreadyExists(err)).ToNot(HaveOccurred())

	requiredPolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
	}

	// Attach all required policies
	for _, policyArn := range requiredPolicies {
		_, err = env.IAMAPI.AttachRolePolicy(env.Context, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: aws.String(policyArn),
		})
		Expect(err).ToNot(HaveOccurred())
	}

	// Verify role exists and has access entry
	Eventually(func(g Gomega) {
		// Verify role exists
		verifyRole, err := env.IAMAPI.GetRole(env.Context, &iam.GetRoleInput{
			RoleName: aws.String(roleName),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(verifyRole.Role).ToNot(BeNil())
	}).Should(Succeed())

	Eventually(func(g Gomega) {
		_, err = env.EKSAPI.CreateAccessEntry(env.Context, &eks.CreateAccessEntryInput{
			ClusterName:  aws.String(env.ClusterName),
			PrincipalArn: aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s", env.ExpectAccountID(), roleName)),
			Type:         aws.String("EC2_LINUX"),
		})
		g.Expect(err).ToNot(HaveOccurred())
	}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
}

// Deletes a role and cleans up associated resources. This includes removing the EKS access entry that authorizes nodes
// to join the cluster, detaching standard node policies (container registry, CNI, worker node, and SSM policies), and
// finally deleting the IAM role itself.
func (env *Environment) ExpectNodeRoleDeleted(roleName string) {
	GinkgoHelper()

	_, err := env.EKSAPI.DeleteAccessEntry(env.Context, &eks.DeleteAccessEntryInput{
		ClusterName:  aws.String(env.ClusterName),
		PrincipalArn: aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s", env.ExpectAccountID(), roleName)),
	})
	Expect(awserrors.IgnoreNotFound(err)).ToNot(HaveOccurred())

	requiredPolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
	}
	// Detach all policies
	for _, policyArn := range requiredPolicies {
		_, err = env.IAMAPI.DetachRolePolicy(env.Context, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: &policyArn,
		})
		Expect(awserrors.IgnoreNotFound(err)).ToNot(HaveOccurred())
	}

	// Delete role
	_, err = env.IAMAPI.DeleteRole(env.Context, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	Expect(awserrors.IgnoreNotFound(err)).ToNot(HaveOccurred())
}
