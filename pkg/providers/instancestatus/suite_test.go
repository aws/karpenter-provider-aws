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

package instancestatus_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceStatusProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = Describe("Instance Status Provider", func() {

	BeforeEach(func() {
		awsEnv.Clock.SetTime(time.Time{})
		statuses := []ec2types.InstanceStatus{
			{
				InstanceId: lo.ToPtr("i-0123456789"),
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
					Details: []ec2types.InstanceStatusDetails{
						{
							Status:        ec2types.StatusTypeFailed,
							Name:          ec2types.StatusNameReachability,
							ImpairedSince: lo.ToPtr(awsEnv.Clock.Now()),
						},
					},
				},
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
					Details: []ec2types.InstanceStatusDetails{
						{
							Status:        ec2types.StatusTypeFailed,
							Name:          ec2types.StatusNameReachability,
							ImpairedSince: lo.ToPtr(awsEnv.Clock.Now()),
						},
					},
				},
				AttachedEbsStatus: &ec2types.EbsStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
					Details: []ec2types.EbsStatusDetails{
						{
							Status:        ec2types.StatusTypeFailed,
							Name:          ec2types.StatusNameReachability,
							ImpairedSince: lo.ToPtr(awsEnv.Clock.Now()),
						},
					},
				},
				Events: []ec2types.InstanceStatusEvent{
					{
						Code: ec2types.EventCodeInstanceRetirement,
					},
				},
			},
		}
		awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
			InstanceStatuses: statuses,
		})
	})
	Context("List and Aggregate", func() {
		It("should return all impairment details", func() {
			impairedTime := awsEnv.Clock.Now()
			awsEnv.Clock.Step(1 * time.Hour)
			statuses, err := awsEnv.InstanceStatusProvider.List(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(statuses).To(HaveLen(1))
			Expect(statuses[0].InstanceID).To(Equal("i-0123456789"))
			Expect(statuses[0].Overall).To(Equal(ec2types.SummaryStatusImpaired))
			Expect(statuses[0].Details).To(ContainElements([]instancestatus.Details{
				{
					Category:      instancestatus.InstanceStatus,
					Name:          string(ec2types.StatusNameReachability),
					Status:        ec2types.StatusTypeFailed,
					ImpairedSince: impairedTime,
				},
				{
					Category:      instancestatus.SystemStatus,
					Name:          string(ec2types.StatusNameReachability),
					Status:        ec2types.StatusTypeFailed,
					ImpairedSince: impairedTime,
				},
			}))
		})
		It("should not return healthy statuses", func() {
			awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId: lo.ToPtr("i-0123456789"),
						SystemStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusInitializing,
						},
						InstanceStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusInsufficientData,
						},
						AttachedEbsStatus: &ec2types.EbsStatusSummary{
							Status: ec2types.SummaryStatusInitializing,
						},
						Events: []ec2types.InstanceStatusEvent{
							{
								Code: ec2types.EventCodeInstanceRetirement,
							},
						},
					},
				},
			})
			statuses, err := awsEnv.InstanceStatusProvider.List(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(statuses).To(HaveLen(0))
		})
	})
})
