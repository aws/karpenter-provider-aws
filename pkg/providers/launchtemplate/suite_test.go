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

package launchtemplate_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"

	. "github.com/aws/karpenter-core/pkg/test/expectations"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options
var env *coretest.Environment
var awsEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)

	fakeClock = &clock.FakeClock{}
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.SubnetProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, env.KubernetesInterface.CoreV1(), events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	cluster.Reset()
	awsEnv.Reset()

	awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("10.0.100.10")
	awsEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(tags []*ec2.Tag, expected map[string]string) {
	GinkgoHelper()
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		Expect(ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		Expect(foundValue).To(Equal(expValue))
	}
}

func ExpectTagsNotFound(tags []*ec2.Tag, expectNotFound map[string]string) {
	GinkgoHelper()
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for k, v := range expectNotFound {
		elem, ok := existingTags[k]
		Expect(!ok || v != elem).To(BeTrue())
	}
}

func ExpectLaunchTemplatesCreatedWithUserDataContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).To(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserDataNotContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).ToNot(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserData(expected string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		// Newlines are always added for missing TOML fields, so strip them out before comparisons.
		actualUserData := strings.Replace(string(userData), "\n", "", -1)
		expectedUserData := strings.Replace(expected, "\n", "", -1)
		ExpectWithOffset(2, actualUserData).To(Equal(expectedUserData))
	})
}
