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

package test

import (
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Validation", func() {
	var provisioner *v1alpha1.Provisioner

	BeforeEach(func() {
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(randomdata.SillyName()),
			},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{
					Name:     "test-cluster",
					Endpoint: "https://test-cluster",
					CABundle: "dGVzdC1jbHVzdGVyCg==",
				},
			},
		}
	})

	It("should support architectures", func() {
		for _, architecture := range []*v1alpha1.Architecture{
			&v1alpha1.ArchitectureAmd64,
			&v1alpha1.ArchitectureArm64,
		} {
			provisioner.Spec.Architecture = architecture
			Expect(provisioner.ValidateCreate()).To(Succeed())
		}
	})

	It("should support operating systems", func() {
		for _, operatingSystem := range []*v1alpha1.OperatingSystem{
			&v1alpha1.OperatingSystemLinux,
		} {
			provisioner.Spec.OperatingSystem = operatingSystem
			Expect(provisioner.ValidateCreate()).To(Succeed())
		}
	})
})
