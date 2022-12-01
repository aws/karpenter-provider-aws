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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("MetadataOptions", func() {
	It("should use specified metadata options", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					MetadataOptions: &v1alpha1.MetadataOptions{
						HTTPEndpoint:            aws.String("enabled"),
						HTTPProtocolIPv6:        aws.String("enabled"),
						HTTPPutResponseHopLimit: aws.Int64(1),
						HTTPTokens:              aws.String("required"),
					},
				},
			},
		})

		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
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
