package launchtemplates

import (
	"strings"
	"testing"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter/test/pkg/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var env *environment.Environment

func TestLaunchTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = environment.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "LaunchTemplates")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
})
var _ = AfterEach(func() {
	env.AfterEach()
})

// TODO: Build Custom AMI Path dynamically based on architecture, server version.
// Avoiding the use of the recommended AMI path, since those are selected by Provisioners by default
// and can result in false positive tests.
var CustomAMIPath string = "/aws/service/eks/optimized-ami/1.21/amazon-linux-2/"

var _ = Describe("Custom Launch Templates", func() {
	It("should use the AMI defined by the AMI Selector", func() {
		amiUnderTest := customAMI()
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyAL2,
		},
			AMISelector: map[string]string{"aws-ids": amiUnderTest}, // TODO - Retrieve recommended EKS AMI and use here.
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(pod, provider, provisioner)

		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		var node v1.Node
		env.Client.Get(env.Context, types.NamespacedName{Name:  pod.Spec.NodeName}, &node)
		instanceID := instanceID(node.Spec.ProviderID)
		ec2api := ec2.New(&env.AWSSession)
		instance, _ := ec2api.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice([]string{instanceID}),
		})
		Expect(*instance.Reservations[0].Instances[0].ImageId).To(Equal(amiUnderTest))
		env.ExpectDeleted(pod)
		env.EventuallyExpectScaleDown()
		env.ExpectNoCrashes()
	})
})

func instanceID(providerID string) string {
	providerIDSplit := strings.Split(providerID, "/")
	return providerIDSplit[len(providerIDSplit)-1]
}

func customAMI() string {
	ssmApi := ssm.New(&env.AWSSession)
	parameters, _ := ssmApi.GetParametersByPath(&ssm.GetParametersByPathInput{
		MaxResults: aws.Int64(1),
		Path: aws.String(CustomAMIPath),
	})
	re := regexp.MustCompile("^.*(ami-[a-z0-9]+).*$")
	return re.FindStringSubmatch(*parameters.Parameters[0].Value)[1]
}