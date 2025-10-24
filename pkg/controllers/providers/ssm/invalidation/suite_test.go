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

package invalidation_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/providers/ssm/invalidation"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var invalidationController *invalidation.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSM Invalidation Controller")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)

	invalidationController = invalidation.NewController(awsEnv.SSMCache, awsEnv.AMIProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv.Reset()
})

var _ = Describe("SSM Invalidation Controller", func() {
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = &v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				AMIFamily: &v1beta1.AMIFamilyAL2023,
			},
		}
	})
	It("shouldn't invalidate cache entries for non-deprecated AMIs", func() {
		_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		currentEntries := getSSMCacheEntries()
		Expect(len(currentEntries)).To(Equal(2))
		awsEnv.AMICache.Flush()
		ExpectReconcileSucceeded(ctx, invalidationController, client.ObjectKey{})
		awsEnv.SSMAPI.Reset()
		_, err = awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		updatedEntries := getSSMCacheEntries()
		Expect(len(updatedEntries)).To(Equal(2))
		for parameter, amiID := range currentEntries {
			updatedAMIID, ok := updatedEntries[parameter]
			Expect(ok).To(BeTrue())
			Expect(updatedAMIID).To(Equal(amiID))
		}
	})
	It("should invalidate cache entries for deprecated AMIs when the SSM parameter is mutable", func() {
		_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		currentEntries := getSSMCacheEntries()
		deprecateAMIs(lo.Values(currentEntries)...)
		Expect(len(currentEntries)).To(Equal(2))
		awsEnv.AMICache.Flush()
		ExpectReconcileSucceeded(ctx, invalidationController, client.ObjectKey{})
		awsEnv.SSMAPI.Reset()
		_, err = awsEnv.AMIProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		updatedEntries := getSSMCacheEntries()
		Expect(len(updatedEntries)).To(Equal(2))
		for parameter, amiID := range currentEntries {
			updatedAMIID, ok := updatedEntries[parameter]
			Expect(ok).To(BeTrue())
			Expect(updatedAMIID).ToNot(Equal(amiID))
		}
	})
})

func getSSMCacheEntries() map[string]string {
	entries := map[string]string{}
	for _, item := range awsEnv.SSMCache.Items() {
		entry := item.Object.(ssm.CacheEntry)
		entries[entry.Parameter.Name] = entry.Value
	}
	return entries
}

func deprecateAMIs(amiIDs ...string) {
	awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{
		Images: lo.Map(amiIDs, func(amiID string, _ int) *ec2.Image {
			return &ec2.Image{
				Name:            lo.ToPtr(coretest.RandomName()),
				ImageId:         lo.ToPtr(amiID),
				CreationDate:    lo.ToPtr(awsEnv.Clock.Now().Add(-24 * time.Hour).Format(time.RFC3339)),
				Architecture:    lo.ToPtr("x86_64"),
				DeprecationTime: lo.ToPtr(awsEnv.Clock.Now().Add(-12 * time.Hour).Format(time.RFC3339)),
			}
		}),
	})
}
