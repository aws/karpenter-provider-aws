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
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/fis"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/mitchellh/hashstructure/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter/pkg/apis/v1beta1"
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

func (env *Environment) ExpectSpotInterruptionExperiment(instanceIDs ...string) *fis.Experiment {
	GinkgoHelper()
	template := &fis.CreateExperimentTemplateInput{
		Actions:        map[string]*fis.CreateExperimentTemplateActionInput{},
		Targets:        map[string]*fis.CreateExperimentTemplateTargetInput{},
		StopConditions: []*fis.CreateExperimentTemplateStopConditionInput{{Source: aws.String("none")}},
		RoleArn:        env.ExpectSpotInterruptionRole().Arn,
		Description:    aws.String(fmt.Sprintf("trigger spot ITN for instances %v", instanceIDs)),
	}
	for j, ids := range lo.Chunk(instanceIDs, fisTargetLimit) {
		key := fmt.Sprintf("itn%d", j)
		template.Actions[key] = &fis.CreateExperimentTemplateActionInput{
			ActionId: aws.String(spotITNAction),
			Parameters: map[string]*string{
				// durationBeforeInterruption is the time before the instance is terminated, so we add 2 minutes
				"durationBeforeInterruption": aws.String("PT120S"),
			},
			Targets: map[string]*string{"SpotInstances": aws.String(key)},
		}
		template.Targets[key] = &fis.CreateExperimentTemplateTargetInput{
			ResourceType:  aws.String("aws:ec2:spot-instance"),
			SelectionMode: aws.String("ALL"),
			ResourceArns: aws.StringSlice(lo.Map(ids, func(id string, _ int) string {
				return fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", env.Region, env.ExpectAccountID(), id)
			})),
		}
	}
	experimentTemplate, err := env.FISAPI.CreateExperimentTemplateWithContext(env.Context, template)
	Expect(err).ToNot(HaveOccurred())
	experiment, err := env.FISAPI.StartExperimentWithContext(env.Context, &fis.StartExperimentInput{ExperimentTemplateId: experimentTemplate.ExperimentTemplate.Id})
	Expect(err).ToNot(HaveOccurred())
	return experiment.Experiment
}

func (env *Environment) ExpectExperimentTemplateDeleted(id string) {
	GinkgoHelper()
	_, err := env.FISAPI.DeleteExperimentTemplateWithContext(env.Context, &fis.DeleteExperimentTemplateInput{
		Id: aws.String(id),
	})
	Expect(err).ToNot(HaveOccurred())
}

func (env *Environment) EventuallyExpectInstanceProfileExists(profileName string) iam.InstanceProfile {
	GinkgoHelper()
	By(fmt.Sprintf("eventually expecting instance profile %s to exist", profileName))
	var instanceProfile iam.InstanceProfile
	Eventually(func(g Gomega) {
		out, err := env.IAMAPI.GetInstanceProfileWithContext(env.Context, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(out.InstanceProfile).ToNot(BeNil())
		g.Expect(out.InstanceProfile.InstanceProfileName).ToNot(BeNil())
		instanceProfile = lo.FromPtr(out.InstanceProfile)
	}).WithTimeout(20 * time.Second).Should(Succeed())
	return instanceProfile
}

// GetInstanceProfileName gets the string for the profile name based on the cluster name, region and the NodeClass name.
// The length of this string can never exceed the maximum instance profile name limit of 128 characters.
func (env *Environment) GetInstanceProfileName(nodeClass *v1beta1.EC2NodeClass) string {
	return fmt.Sprintf("%s_%d", env.ClusterName, lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s", env.Region, nodeClass.Name), hashstructure.FormatV2, nil)))
}

func (env *Environment) GetInstance(nodeName string) ec2.Instance {
	node := env.Environment.GetNode(nodeName)
	return env.GetInstanceByID(env.ExpectParsedProviderID(node.Spec.ProviderID))
}

func (env *Environment) ExpectInstanceStopped(nodeName string) {
	GinkgoHelper()
	node := env.Environment.GetNode(nodeName)
	_, err := env.EC2API.StopInstances(&ec2.StopInstancesInput{
		Force:       aws.Bool(true),
		InstanceIds: aws.StringSlice([]string{env.ExpectParsedProviderID(node.Spec.ProviderID)}),
	})
	Expect(err).To(Succeed())
}

func (env *Environment) ExpectInstanceTerminated(nodeName string) {
	GinkgoHelper()
	node := env.Environment.GetNode(nodeName)
	_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{env.ExpectParsedProviderID(node.Spec.ProviderID)}),
	})
	Expect(err).To(Succeed())
}

func (env *Environment) GetInstanceByID(instanceID string) ec2.Instance {
	GinkgoHelper()
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(instance.Reservations).To(HaveLen(1))
	Expect(instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) GetVolume(id *string) *ec2.Volume {
	volumes := env.GetVolumes(id)
	Expect(volumes).To(HaveLen(1))
	return volumes[0]
}

func (env *Environment) GetVolumes(ids ...*string) []*ec2.Volume {
	GinkgoHelper()
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: ids})
	Expect(err).ToNot(HaveOccurred())
	return dvo.Volumes
}

func (env *Environment) GetNetworkInterface(id *string) *ec2.NetworkInterface {
	networkInterfaces := env.GetNetworkInterfaces(id)
	Expect(networkInterfaces).To(HaveLen(1))
	return networkInterfaces[0]
}

func (env *Environment) GetNetworkInterfaces(ids ...*string) []*ec2.NetworkInterface {
	GinkgoHelper()
	dnio, err := env.EC2API.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: ids})
	Expect(err).ToNot(HaveOccurred())
	return dnio.NetworkInterfaces
}

func (env *Environment) GetSpotInstanceRequest(id *string) *ec2.SpotInstanceRequest {
	GinkgoHelper()
	siro, err := env.EC2API.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{SpotInstanceRequestIds: []*string{id}})
	Expect(err).ToNot(HaveOccurred())
	Expect(siro.SpotInstanceRequests).To(HaveLen(1))
	return siro.SpotInstanceRequests[0]
}

// GetSubnets returns all subnets matching the label selector
// mapped from AZ -> {subnet-ids...}
func (env *Environment) GetSubnets(tags map[string]string) map[string][]string {
	var filters []*ec2.Filter
	for key, val := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(val)},
		})
	}
	subnets := map[string][]string{}
	err := env.EC2API.DescribeSubnetsPages(&ec2.DescribeSubnetsInput{Filters: filters}, func(dso *ec2.DescribeSubnetsOutput, _ bool) bool {
		for _, subnet := range dso.Subnets {
			subnets[*subnet.AvailabilityZone] = append(subnets[*subnet.AvailabilityZone], *subnet.SubnetId)
		}
		return true
	})
	Expect(err).To(BeNil())
	return subnets
}

// SubnetInfo is a simple struct for testing
type SubnetInfo struct {
	Name string
	ID   string
}

// GetSubnetNameAndIds returns all subnets matching the label selector
func (env *Environment) GetSubnetNameAndIds(tags map[string]string) []SubnetInfo {
	var filters []*ec2.Filter
	for key, val := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(val)},
		})
	}
	var subnetInfo []SubnetInfo
	err := env.EC2API.DescribeSubnetsPages(&ec2.DescribeSubnetsInput{Filters: filters}, func(dso *ec2.DescribeSubnetsOutput, _ bool) bool {
		subnetInfo = lo.Map(dso.Subnets, func(s *ec2.Subnet, _ int) SubnetInfo {
			elem := SubnetInfo{ID: aws.StringValue(s.SubnetId)}
			if tag, ok := lo.Find(s.Tags, func(t *ec2.Tag) bool { return aws.StringValue(t.Key) == "Name" }); ok {
				elem.Name = aws.StringValue(tag.Value)
			}
			return elem
		})
		return true
	})
	Expect(err).To(BeNil())
	return subnetInfo
}

type SecurityGroup struct {
	ec2.GroupIdentifier
	Tags []*ec2.Tag
}

// GetSecurityGroups returns all getSecurityGroups matching the label selector
func (env *Environment) GetSecurityGroups(tags map[string]string) []SecurityGroup {
	var filters []*ec2.Filter
	for key, val := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(val)},
		})
	}
	var securityGroups []SecurityGroup
	err := env.EC2API.DescribeSecurityGroupsPages(&ec2.DescribeSecurityGroupsInput{Filters: filters}, func(dso *ec2.DescribeSecurityGroupsOutput, _ bool) bool {
		for _, sg := range dso.SecurityGroups {
			securityGroups = append(securityGroups, SecurityGroup{
				Tags:            sg.Tags,
				GroupIdentifier: ec2.GroupIdentifier{GroupId: sg.GroupId, GroupName: sg.GroupName},
			})
		}
		return true
	})
	Expect(err).To(BeNil())
	return securityGroups
}

func (env *Environment) ExpectMessagesCreated(msgs ...interface{}) {
	GinkgoHelper()
	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	var err error
	for _, msg := range msgs {
		wg.Add(1)
		go func(m interface{}) {
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

func (env *Environment) GetK8sVersion(offset int) string {
	serverVersion, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	minorVersion, err := strconv.Atoi(strings.TrimSuffix(serverVersion.Minor, "+"))
	Expect(err).To(BeNil())
	// Choose a minor version one lesser than the server's minor version. This ensures that we choose an AMI for
	// this test that wouldn't be selected as Karpenter's SSM default (therefore avoiding false positives), and also
	// ensures that we aren't violating version skew.
	return fmt.Sprintf("%s.%d", serverVersion.Major, minorVersion-offset)
}

func (env *Environment) GetCustomAMI(amiPath string, versionOffset int) string {
	version := env.GetK8sVersion(versionOffset)
	parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf(amiPath, version)),
	})
	Expect(err).To(BeNil())
	return *parameter.Parameter.Value
}

func (env *Environment) EventuallyExpectRunInstances(instanceInput *ec2.RunInstancesInput) *ec2.Reservation {
	GinkgoHelper()
	// implement IMDSv2
	instanceInput.MetadataOptions = &ec2.InstanceMetadataOptionsRequest{
		HttpEndpoint: aws.String("enabled"),
		HttpTokens:   aws.String("required"),
	}
	var out *ec2.Reservation
	var err error
	Eventually(func(g Gomega) {
		out, err = env.EC2API.RunInstances(instanceInput)
		g.Expect(err).ToNot(HaveOccurred())
	}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
	return out
}

func (env *Environment) ExpectSpotInterruptionRole() *iam.Role {
	GinkgoHelper()
	out, err := env.IAMAPI.GetRoleWithContext(env.Context, &iam.GetRoleInput{
		RoleName: aws.String(fisRoleName),
	})
	Expect(err).ToNot(HaveOccurred())
	return out.Role
}

func (env *Environment) ExpectAccountID() string {
	GinkgoHelper()
	identity, err := env.STSAPI.GetCallerIdentityWithContext(env.Context, &sts.GetCallerIdentityInput{})
	Expect(err).ToNot(HaveOccurred())
	return aws.StringValue(identity.Account)
}
