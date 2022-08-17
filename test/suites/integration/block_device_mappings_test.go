package integration_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/resources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BlockDeviceMappings", func() {
	BeforeEach(func() {

	})

	It("should use specified block device mappings", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				LaunchTemplate: awsv1alpha1.LaunchTemplate{
					BlockDeviceMappings: []*awsv1alpha1.BlockDeviceMapping{
						{
							DeviceName: aws.String("/dev/xvda"),
							EBS: &awsv1alpha1.BlockDevice{
								VolumeSize:          resources.Quantity("10G"),
								VolumeType:          aws.String("io2"),
								IOPS:                aws.Int64(1000),
								Encrypted:           aws.Bool(true),
								DeleteOnTermination: aws.Bool(true),
							},
						},
					},
				},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(len(instance.BlockDeviceMappings)).To(Equal(1))
		Expect(instance.BlockDeviceMappings[0]).ToNot(BeNil())
		Expect(instance.BlockDeviceMappings[0]).To(HaveField("DeviceName", HaveValue(Equal("/dev/xvda"))))
		Expect(instance.BlockDeviceMappings[0].Ebs).To(HaveField("DeleteOnTermination", HaveValue(BeTrue())))
		volume := env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId)
		Expect(volume).To(HaveField("Encrypted", HaveValue(BeTrue())))
		Expect(volume).To(HaveField("Size", HaveValue(Equal(int64(10)))))
		Expect(volume).To(HaveField("Iops", HaveValue(Equal(int64(1000)))))
		Expect(volume).To(HaveField("VolumeType", HaveValue(Equal("io2"))))
	})

})
