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

package v1alpha1

import (
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Provisioning",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Validation", func() {
	var provisioner *Provisioner

	BeforeEach(func() {
		provisioner = &Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(randomdata.SillyName()),
			},
			Spec: ProvisionerSpec{
				Cluster: &ClusterSpec{
					Name:     "test-cluster",
					Endpoint: "https://test-cluster",
					CABundle: "dGVzdC1jbHVzdGVyCg==",
				},
			},
		}
	})

	It("should fail for empty cluster specification", func() {
		for _, cluster := range []*ClusterSpec{
			nil,
			{Endpoint: "https://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			{Name: "test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			{Name: "test-cluster", Endpoint: "https://test-cluster"},
		} {
			provisioner.Spec.Cluster = cluster
			Expect(provisioner.ValidateCreate()).ToNot(Succeed())
		}
	})

	It("should fail for restricted labels", func() {
		for _, label := range RestrictedLabels {
			provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
			Expect(provisioner.ValidateCreate()).ToNot(Succeed())
		}
	})

	It("should fail for invalid architecture", func() {
		// Nil is ok
		Expect(provisioner.ValidateCreate()).To(Succeed())

		// Not supported (unregistered)
		provisioner.Spec.Architecture = &ArchitectureAmd64
		Expect(provisioner.ValidateCreate()).ToNot(Succeed())

		// Supported (registered)
		for _, architecture := range []*Architecture{
			&ArchitectureAmd64,
			&ArchitectureArm64,
		} {
			AddSupportedArchitectures(*architecture)
			provisioner.Spec.Architecture = architecture
			Expect(provisioner.ValidateCreate()).To(Succeed())
		}
	})

	It("should fail for invalid operating system", func() {
		// Nil is ok
		Expect(provisioner.ValidateCreate()).To(Succeed())

		// Not supported (unregistered)
		provisioner.Spec.OperatingSystem = &OperatingSystemLinux
		Expect(provisioner.ValidateCreate()).ToNot(Succeed())

		// Supported (registered)
		for _, operatingSystem := range []*OperatingSystem{
			&OperatingSystemLinux,
		} {
			AddSupportedOperatingSystems(*operatingSystem)
			provisioner.Spec.OperatingSystem = operatingSystem
			Expect(provisioner.ValidateCreate()).To(Succeed())
		}
	})
})
