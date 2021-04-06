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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhooks",
		[]Reporter{printer.NewlineReporter{}})
}

var env = test.NewEnvironment(func(e *test.Environment) {
	e.Manager.RegisterWebhooks(
		&Validator{CloudProvider: fake.NewFactory(cloudprovider.Options{})},
		&Defaulter{},
	)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Validation", func() {
	var provisioner *v1alpha1.Provisioner

	BeforeEach(func() {
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
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

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	It("should fail for empty cluster specification", func() {
		for _, cluster := range []*v1alpha1.ClusterSpec{
			nil,
			{Endpoint: "https://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			{Name: "test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			{Name: "test-cluster", Endpoint: "https://test-cluster"},
		} {
			provisioner.Spec.Cluster = cluster
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		}
	})

	It("should fail for restricted labels", func() {
		for _, label := range []string{
			v1alpha1.ArchitectureLabelKey,
			v1alpha1.OperatingSystemLabelKey,
			v1alpha1.ProvisionerNameLabelKey,
			v1alpha1.ProvisionerNamespaceLabelKey,
			v1alpha1.ProvisionerPhaseLabel,
			v1alpha1.ProvisionerTTLKey,
			v1alpha1.ZoneLabelKey,
			v1alpha1.InstanceTypeLabelKey,
		} {
			provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		}
	})

	Context("Zones", func() {
		It("should succeed if unspecified", func() {
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.Zones = []string{"unknown"}
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.Zones = []string{"test-zone-1"}
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
	})

	Context("InstanceTypes", func() {
		It("should succeed if unspecified", func() {
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.InstanceTypes = []string{"unknown"}
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.InstanceTypes = []string{
				"test-instance-type-1",
			}
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
	})

	Context("Architecture", func() {
		It("should succeed if unspecified", func() {
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.Architecture = ptr.String("unknown")
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.Architecture = ptr.String("test-architecture-1")
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
	})

	Context("OperatingSystem", func() {
		It("should succeed if unspecified", func() {
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
		It("should fail if not supported", func() {
			provisioner.Spec.OperatingSystem = ptr.String("unknown")
			Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
		})
		It("should succeed if supported", func() {
			provisioner.Spec.OperatingSystem = ptr.String("test-operating-system-1")
			Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
		})
	})
})
