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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Tags", func() {
	It("should tag all associated resources", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				Tags:                  map[string]string{"TestTag": "TestVal"},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		volumeTags := tagMap(env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId).Tags)
		instanceTags := tagMap(instance.Tags)

		Expect(instanceTags).To(HaveKeyWithValue("TestTag", "TestVal"))
		Expect(volumeTags).To(HaveKeyWithValue("TestTag", "TestVal"))
	})
	It("should tag all associated resources with global tags", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})

		env.ExpectSettingsOverridden(map[string]string{
			"aws.tags.TestTag": "TestVal",
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		volumeTags := tagMap(env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId).Tags)
		instanceTags := tagMap(instance.Tags)

		Expect(instanceTags).To(HaveKeyWithValue("TestTag", "TestVal"))
		Expect(volumeTags).To(HaveKeyWithValue("TestTag", "TestVal"))
	})
})

func tagMap(tags []*ec2.Tag) map[string]string {
	return lo.SliceToMap(tags, func(tag *ec2.Tag) (string, string) {
		return *tag.Key, *tag.Value
	})
}
