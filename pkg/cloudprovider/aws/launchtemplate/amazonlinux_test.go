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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate"
	f "github.com/aws/karpenter/pkg/cloudprovider/fake"
)

var _ = Describe("Amazonlinux", func() {
	input := &launchtemplate.Amazonlinux{}
	constraints := &v1alpha1.Constraints{}
	builder := launchtemplate.NewBuilder(fake.DefaultK8sClient(), fake.DefaultSSMClient(), fake.NewDefaultAMIResolver(), fake.DefaultSecurityGroupResolver())
	version, err := builder.K8sClient.ServerVersion(context.Background())
	Expect(err).To(BeNil())
	configuration := &launchtemplate.Configuration{
		Constraints:            constraints,
		ClusterName:            "cluster",
		ClusterEndpoint:        "https://localhost",
		DefaultInstanceProfile: "profile",
		KubernetesVersion:      *version,
		NodeLabels:             constraints.Labels,
		CABundle:               pointer.String("CADATA"),
	}

	Context("x86_64", func() {
		instanceType := f.NewInstanceType(f.InstanceTypeOptions{
			Architecture: "x86_64",
		})
		Describe("GetImageID", func() {
			It("must return the corresponding ImageID", func() {
				id, err := input.GetImageID(context.Background(), builder, configuration, instanceType)
				Expect(err).To(BeNil())
				Expect(id).To(Equal("ami-015c52b52fe1c5990"))

			})
		})
	})

	Context("arm64", func() {
		instanceType := f.NewInstanceType(f.InstanceTypeOptions{
			Architecture: "arm64",
		})
		Describe("GetImageID", func() {
			It("must return the corresponding ImageID", func() {
				id, err := input.GetImageID(context.Background(), builder, configuration, instanceType)
				Expect(err).To(BeNil())
				Expect(id).To(Equal("ami-002a052abdc5fff1c"))

			})
		})
	})

})
