package integration_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataOptions", func() {
	BeforeEach(func() {

	})

	It("should use specified metadata options", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				LaunchTemplate: awsv1alpha1.LaunchTemplate{
					MetadataOptions: &awsv1alpha1.MetadataOptions{
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
