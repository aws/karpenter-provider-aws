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
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"knative.dev/pkg/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation")
}

var _ = Describe("Validation", func() {
	var ctx context.Context
	var provisioner *Provisioner

	BeforeEach(func() {
		ctx = context.Background()
		provisioner = &Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
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
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		}
	})

	Context("Labels", func() {
		It("should fail for invalid label keys", func() {
			provisioner.Spec.Labels = map[string]string{"spaces are not allowed": randomdata.SillyName()}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for invalid label values", func() {
			provisioner.Spec.Labels = map[string]string{randomdata.SillyName(): "/ is not allowed"}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for restricted labels", func() {
			for _, label := range []string{
				ArchitectureLabelKey,
				OperatingSystemLabelKey,
				ProvisionerNameLabelKey,
				ProvisionerNamespaceLabelKey,
				ProvisionerPhaseLabel,
				ZoneLabelKey,
				InstanceTypeLabelKey,
			} {
				provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
	})
	Context("Zones", func() {
		SupportedZones = append(SupportedZones, "test-zone-1")
		It("should succeed if unspecified", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.Zones = []string{"unknown"}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.Zones = []string{"test-zone-1"}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})

	Context("InstanceTypes", func() {
		SupportedInstanceTypes = append(SupportedInstanceTypes, "test-instance-type")
		It("should succeed if unspecified", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.InstanceTypes = []string{"unknown"}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.InstanceTypes = []string{
				"test-instance-type",
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})

	Context("Architecture", func() {
		SupportedArchitectures = append(SupportedArchitectures, "test-architecture")
		It("should succeed if unspecified", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.Architecture = ptr.String("unknown")
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.Architecture = ptr.String("test-architecture")
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})

	Context("OperatingSystem", func() {
		SupportedOperatingSystems = append(SupportedArchitectures, "test-operating-system")
		It("should succeed if unspecified", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.OperatingSystem = ptr.String("unknown")
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.OperatingSystem = ptr.String("test-operating-system")
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})
})
