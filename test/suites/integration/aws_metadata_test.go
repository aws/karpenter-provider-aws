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

package integration_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataOptions", func() {
	It("should use specified metadata options", func() {
		nodeClass.Spec.MetadataOptions = &v1beta1.MetadataOptions{
			HTTPEndpoint:            aws.String("enabled"),
			HTTPProtocolIPv6:        aws.String("enabled"),
			HTTPPutResponseHopLimit: aws.Int64(1),
			HTTPTokens:              aws.String("required"),
		}
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("MetadataOptions", HaveValue(Equal(ec2.InstanceMetadataOptionsResponse{
			State:                   aws.String(ec2.InstanceMetadataOptionsStateApplied),
			HttpEndpoint:            aws.String("enabled"),
			HttpProtocolIpv6:        aws.String("enabled"),
			HttpPutResponseHopLimit: aws.Int64(1),
			HttpTokens:              aws.String("required"),
			InstanceMetadataTags:    aws.String("disabled"),
		}))))
	})
})
