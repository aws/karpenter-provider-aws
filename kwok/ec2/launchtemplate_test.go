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

package ec2

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The EC2NodeClass finalizer deletes launch templates by name with launchtemplate.DeleteAll. If this fake stores two
// templates under one name, the extra one is listed but can't be deleted by name, so the finalizer never clears.
var _ = Describe("LaunchTemplates", func() {
	const ltName = "karpenter.k8s.aws/123"
	var ctx context.Context
	var cancel context.CancelFunc
	var client *Client
	var provider *launchtemplate.DefaultProvider
	var nodeClass *v1.EC2NodeClass

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		ctx = options.ToContext(ctx, test.Options())
		readBackupCompleted := make(chan struct{})
		close(readBackupCompleted)
		client = &Client{
			rateLimiterProvider: NewNopRateLimiterProvider(),
			clock:               clock.NewFakeClock(time.Now()),
			readBackupCompleted: readBackupCompleted,
		}
		nodeClass = &v1.EC2NodeClass{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
		// startAsync never closes, so the provider doesn't hydrate its cache
		provider = launchtemplate.NewDefaultProvider(ctx, cache.New(time.Hour, time.Hour), client, nil, nil, nil, nil, nil, nil, make(chan struct{}), nil, "")
	})
	AfterEach(func() {
		cancel()
	})

	create := func() error {
		_, err := client.CreateLaunchTemplate(ctx, &ec2.CreateLaunchTemplateInput{
			LaunchTemplateName: aws.String(ltName),
			LaunchTemplateData: &ec2types.RequestLaunchTemplateData{},
			TagSpecifications: []ec2types.TagSpecification{{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags: []ec2types.Tag{
					{Key: aws.String(v1.EKSClusterNameTagKey), Value: aws.String(options.FromContext(ctx).ClusterName)},
					{Key: aws.String(v1.NodeClassTagKey), Value: aws.String(nodeClass.Name)},
				},
			}},
		})
		return err
	}
	expectNoLaunchTemplates := func() {
		GinkgoHelper()
		out, err := client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
		Expect(err).ToNot(HaveOccurred())
		Expect(out.LaunchTemplates).To(BeEmpty())
	}

	It("should reject a second create of the same name and let DeleteAll finish", func() {
		// The provider creates before describing, so a cache miss on an existing template calls create again
		Expect(create()).To(Succeed())
		err := create()

		Expect(provider.DeleteAll(ctx, nodeClass)).To(Succeed())
		expectNoLaunchTemplates()
		// Checked after DeleteAll so a regression fails with the finalizer error
		Expect(awserrors.IsAlreadyExists(err)).To(BeTrue())
	})
	It("should keep one template when the same name is created concurrently", func() {
		var wg sync.WaitGroup
		errs := make([]error, 10)
		for i := range errs {
			wg.Go(func() { errs[i] = create() })
		}
		wg.Wait()

		Expect(provider.DeleteAll(ctx, nodeClass)).To(Succeed())
		expectNoLaunchTemplates()
		Expect(lo.Count(errs, nil)).To(Equal(1))
		for _, err := range lo.Compact(errs) {
			Expect(awserrors.IsAlreadyExists(err)).To(BeTrue())
		}
	})
	It("should free the name when a template is deleted by ID", func() {
		// The provider's cache eviction deletes by ID, then a later launch creates the name again
		Expect(create()).To(Succeed())
		out, err := client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{LaunchTemplateNames: []string{ltName}})
		Expect(err).ToNot(HaveOccurred())
		Expect(out.LaunchTemplates).To(HaveLen(1))
		dupErr := create()
		_, err = client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateId: out.LaunchTemplates[0].LaunchTemplateId})
		Expect(err).ToNot(HaveOccurred())
		Expect(create()).To(Succeed())

		Expect(provider.DeleteAll(ctx, nodeClass)).To(Succeed())
		expectNoLaunchTemplates()
		Expect(awserrors.IsAlreadyExists(dupErr)).To(BeTrue())
	})
})
