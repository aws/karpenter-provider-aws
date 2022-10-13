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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/gomega" //nolint:revive,stylecheck
)

func (env *Environment) ExpectInstance(nodeName string) Assertion {
	return Expect(env.GetInstance(nodeName))
}

func (env *Environment) GetInstance(nodeName string) ec2.Instance {
	node := env.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, instance.Reservations).To(HaveLen(1))
	ExpectWithOffset(1, instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) ExpectInstanceStopped(nodeName string) {
	node := env.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	_, err := env.EC2API.StopInstances(&ec2.StopInstancesInput{
		Force:       aws.Bool(true),
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) ExpectInstanceTerminated(nodeName string) {
	node := env.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).To(Succeed())
}

func (env *Environment) GetVolume(volumeID *string) ec2.Volume {
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{volumeID}})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, len(dvo.Volumes)).To(Equal(1))
	return *dvo.Volumes[0]
}
