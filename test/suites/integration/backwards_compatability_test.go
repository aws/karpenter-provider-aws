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
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("BackwardsCompatability", func() {
	It("should succeed to launch a node by specifying a provider in the Provisioner", func() {
		provisioner := test.Provisioner(
			test.ProvisionerOptions{
				Provider: &v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					Tags: map[string]string{
						"custom-tag":  "custom-value",
						"custom-tag2": "custom-value2",
					},
				},
			},
		)
		pod := test.Pod()
		env.ExpectCreated(pod, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodes := env.Monitor.CreatedNodes()
		Expect(nodes).To(HaveLen(1))
		Expect(env.GetInstance(nodes[0].Name).Tags).To(ContainElements(
			&ec2.Tag{Key: lo.ToPtr("custom-tag"), Value: lo.ToPtr("custom-value")},
			&ec2.Tag{Key: lo.ToPtr("custom-tag2"), Value: lo.ToPtr("custom-value2")},
		))
	})
})
