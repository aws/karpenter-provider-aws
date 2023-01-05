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

package drift_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clock "k8s.io/utils/clock/testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/apis"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/controllers/drift"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var provisioner *corev1alpha5.Provisioner
var nodeTemplate *v1alpha1.AWSNodeTemplate
var unavailableOfferingsCache *awscache.UnavailableOfferings
var recorder *coretest.EventRecorder
var fakeClock *clock.FakeClock
var controller corecontroller.Controller
var settingsStore coretest.SettingsStore

// var nodeStateController corecontroller.Controller
var cloudProvider corecloudprovider.CloudProvider
var validAMIs []string

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSDrift")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	settingsStore = coretest.SettingsStore{
		settings.ContextKey:    coretest.Settings(),
		awssettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)
	ctx, stop = context.WithCancel(ctx)
	fakeClock = &clock.FakeClock{}
	recorder = coretest.NewEventRecorder()
	validAMIs = []string{"ami-123"}
	cloudProvider = fake.NewCloudProvider(validAMIs...)
	unavailableOfferingsCache = awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, awscache.CleanupInterval))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	settingsStore = coretest.SettingsStore{
		settings.ContextKey:    coretest.Settings(),
		awssettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)
	controller = drift.NewController(env.Client, cloudProvider)
	nodeTemplate = test.AWSNodeTemplate()
	nodeTemplate = &v1alpha1.AWSNodeTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: coretest.RandomName(),
		},
		Spec: v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
				SubnetSelector:        map[string]string{"*": "*"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			},
		},
	}
	nodeTemplate.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "AWSNodeTemplate",
	})
	provisioner = test.Provisioner(coretest.ProvisionerOptions{
		Requirements: []v1.NodeSelectorRequirement{{
			Key:      v1alpha1.LabelInstanceCategory,
			Operator: v1.NodeSelectorOpExists,
		}},
		ProviderRef: &corev1alpha5.ProviderRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		},
	})
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("AWSDrift", func() {
	Context("AMIs", func() {
		It("should not detect drift if the feature flag is disabled", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey:    coretest.Settings(coretest.SettingsOptions{DriftEnabled: false}),
				awssettings.ContextKey: test.Settings(),
			}
			ctx = settingsStore.InjectSettings(ctx)
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha1.LabelInstanceAMIID:          "ami-invalid",
						v1.LabelInstanceTypeStable:           coretest.RandomName(),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeTemplate, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKeyWithValue(corev1alpha5.VoluntaryDisruptionAnnotationKey, corev1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		})
		It("should not detect drift if the provisioner does not exist", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey:    coretest.Settings(coretest.SettingsOptions{DriftEnabled: true}),
				awssettings.ContextKey: test.Settings(),
			}
			ctx = settingsStore.InjectSettings(ctx)
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha1.LabelInstanceAMIID:          "ami-invalid",
						v1.LabelInstanceTypeStable:           coretest.RandomName(),
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodeTemplate, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKeyWithValue(corev1alpha5.VoluntaryDisruptionAnnotationKey, corev1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		})
		It("should detect drift when the AMI is not valid", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey:    coretest.Settings(coretest.SettingsOptions{DriftEnabled: true}),
				awssettings.ContextKey: test.Settings(),
			}
			ctx = settingsStore.InjectSettings(ctx)
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha1.LabelInstanceAMIID:          "ami-invalid",
						v1.LabelInstanceTypeStable:           coretest.RandomName(),
					},
				},
			})
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(node.Annotations).To(HaveKeyWithValue(corev1alpha5.VoluntaryDisruptionAnnotationKey, corev1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		})
		It("should not detect drift when the AMI is valid", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey:    coretest.Settings(coretest.SettingsOptions{DriftEnabled: true}),
				awssettings.ContextKey: test.Settings(),
			}
			ctx = settingsStore.InjectSettings(ctx)
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha1.LabelInstanceAMIID:          validAMIs[0],
						v1.LabelInstanceTypeStable:           coretest.RandomName(),
					},
				},
			})
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(node))
			node = ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(node.Annotations).ToNot(HaveKeyWithValue(corev1alpha5.VoluntaryDisruptionAnnotationKey, corev1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		})
	})
})
