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
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataOptions", func() {
	It("should use specified metadata options", func() {
		nodeClass.Spec.MetadataOptions = &v1.MetadataOptions{
			HTTPEndpoint:            aws.String("enabled"),
			HTTPProtocolIPv6:        aws.String("enabled"),
			HTTPPutResponseHopLimit: aws.Int64(1),
			HTTPTokens:              aws.String("required"),
		}
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("MetadataOptions", HaveValue(Equal(ec2types.InstanceMetadataOptionsResponse{
			State:                   ec2types.InstanceMetadataOptionsStateApplied,
			HttpEndpoint:            ec2types.InstanceMetadataEndpointStateEnabled,
			HttpProtocolIpv6:        ec2types.InstanceMetadataProtocolStateEnabled,
			HttpPutResponseHopLimit: aws.Int32(1),
			HttpTokens:              ec2types.HttpTokensStateRequired,
			InstanceMetadataTags:    ec2types.InstanceMetadataTagsStateDisabled,
		}))))
	})
})
