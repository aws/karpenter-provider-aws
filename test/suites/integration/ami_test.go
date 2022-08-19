package integration_test

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var customAMI string

var _ = Describe("LaunchTemplates", func() {
	BeforeEach(func() {
		customAMI = selectCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id")
	})

	It("should use the AMI defined by the AMI Selector", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				AMIFamily:             &awsv1alpha1.AMIFamilyAL2,
			},
			AMISelector: map[string]string{"aws-ids": customAMI},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should support Custom AMIFamily with AMI Selectors", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			AMIFamily:             &awsv1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": customAMI},
			UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should merge UserData contents for AL2 AMIFamily", func() {
		content, err := ioutil.ReadFile("testdata/al2_userdata_input.golden")
		Expect(err).ToNot(HaveOccurred())
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			AMIFamily:             &awsv1alpha1.AMIFamilyAL2,
		},
			UserData: aws.String(string(content)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
		Expect(err).ToNot(HaveOccurred())
		// Since the node has joined the cluster, we know our bootstrapping was correct.
		// Just verify if the UserData contains our custom content too, rather than doing a byte-wise comparison.
		Expect(string(actualUserData)).To(ContainSubstring("Running custom user data script"))
	})
	It("should merge UserData contents for Bottlerocket AMIFamily", func() {
		content, err := ioutil.ReadFile("testdata/br_userdata_input.golden")
		Expect(err).ToNot(HaveOccurred())
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			AMIFamily:             &awsv1alpha1.AMIFamilyBottlerocket,
		},
			UserData: aws.String(string(content)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(actualUserData)).To(ContainSubstring("kube-api-qps = 30"))
	})
})

func getInstanceAttribute(nodeName string, attribute string) *ec2.DescribeInstanceAttributeOutput {
	var node v1.Node
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instanceAttribute, err := env.EC2API.DescribeInstanceAttribute(&ec2.DescribeInstanceAttributeInput{
		InstanceId: aws.String(instanceID),
		Attribute:  aws.String(attribute),
	})
	Expect(err).ToNot(HaveOccurred())
	return instanceAttribute
}

func selectCustomAMI(amiPath string) string {
	serverVersion, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	minorVersion, err := strconv.Atoi(strings.TrimSuffix(serverVersion.Minor, "+"))
	Expect(err).To(BeNil())
	// Choose a minor version one lesser than the server's minor version. This ensures that we choose an AMI for
	// this test that wouldn't be selected as Karpenter's SSM default (therefore avoiding false positives), and also
	// ensures that we aren't violating version skew.
	version := fmt.Sprintf("%s.%d", serverVersion.Major, minorVersion-1)
	parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf(amiPath, version)),
	})
	Expect(err).To(BeNil())
	return *parameter.Parameter.Value
}
