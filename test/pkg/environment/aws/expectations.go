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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func (env *Environment) GetInstance(nodeName string) ec2.Instance {
	node := env.Environment.GetNode(nodeName)
	return env.GetInstanceByIDWithOffset(1, env.ExpectParsedProviderID(node.Spec.ProviderID))
}

func (env *Environment) ExpectInstanceStopped(nodeName string) {
	node := env.Environment.GetNode(nodeName)
	_, err := env.EC2API.StopInstances(&ec2.StopInstancesInput{
		Force:       aws.Bool(true),
		InstanceIds: aws.StringSlice([]string{env.ExpectParsedProviderID(node.Spec.ProviderID)}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) ExpectInstanceTerminated(nodeName string) {
	node := env.Environment.GetNode(nodeName)
	_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{env.ExpectParsedProviderID(node.Spec.ProviderID)}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) GetInstanceByID(instanceID string) ec2.Instance {
	return env.GetInstanceByIDWithOffset(1, instanceID)
}

func (env *Environment) GetInstanceByIDWithOffset(offset int, instanceID string) ec2.Instance {
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(offset+1, err).ToNot(HaveOccurred())
	ExpectWithOffset(offset+1, instance.Reservations).To(HaveLen(1))
	ExpectWithOffset(offset+1, instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) GetVolume(volumeID *string) ec2.Volume {
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{volumeID}})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, len(dvo.Volumes)).To(Equal(1))
	return *dvo.Volumes[0]
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

func (env *Environment) ExpectQueueExists() {
	exists, err := env.SQSProvider.QueueExists(env.Context)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, exists).To(BeTrue())
}

func (env *Environment) ExpectMessagesCreated(msgs ...interface{}) {
	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	var err error
	for _, msg := range msgs {
		wg.Add(1)
		go func(m interface{}) {
			defer wg.Done()
			defer GinkgoRecover()
			_, e := env.SQSProvider.SendMessage(env.Environment.Context, m)
			if e != nil {
				mu.Lock()
				err = multierr.Append(err, e)
				mu.Unlock()
			}
		}(msg)
	}
	wg.Wait()
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) ExpectParsedProviderID(providerID string) string {
	providerIDSplit := strings.Split(providerID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	return providerIDSplit[len(providerIDSplit)-1]
}

func (env *Environment) GetCustomAMI(amiPath string, versionOffset int) string {
	serverVersion, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	minorVersion, err := strconv.Atoi(strings.TrimSuffix(serverVersion.Minor, "+"))
	Expect(err).To(BeNil())
	// Choose a minor version one lesser than the server's minor version. This ensures that we choose an AMI for
	// this test that wouldn't be selected as Karpenter's SSM default (therefore avoiding false positives), and also
	// ensures that we aren't violating version skew.
	version := fmt.Sprintf("%s.%d", serverVersion.Major, minorVersion-versionOffset)
	parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf(amiPath, version)),
	})
	Expect(err).To(BeNil())
	return *parameter.Parameter.Value
}

func (env *Environment) ExpectRunInstances(instanceInput *ec2.RunInstancesInput) *ec2.Reservation {
	GinkgoHelper()
	// implement IMDSv2
	instanceInput.MetadataOptions = &ec2.InstanceMetadataOptionsRequest{
		HttpEndpoint: aws.String("enabled"),
		HttpTokens:   aws.String("required"),
	}

	out, err := env.EC2API.RunInstances(instanceInput)
	Expect(err).ToNot(HaveOccurred())

	return out
}
