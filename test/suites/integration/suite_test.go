package integration

import (
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var env *environment.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = environment.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "Integration Suite")
}

var _ = AfterEach(func() {
	env.ExpectCleaned()
})

var _ = Describe("Sanity Checks", func() {
	It("should provision nodes", func() {
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
	})
})
