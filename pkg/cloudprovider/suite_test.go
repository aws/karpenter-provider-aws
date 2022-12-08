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

package cloudprovider

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis"
	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/operator/controller"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter-core/pkg/utils/pretty"

	"github.com/aws/karpenter/pkg/fake"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options
var env *coretest.Environment
var launchTemplateCache *cache.Cache
var securityGroupCache *cache.Cache
var subnetCache *cache.Cache
var ssmCache *cache.Cache
var ec2Cache *cache.Cache
var internalUnavailableOfferingsCache *cache.Cache
var unavailableOfferingsCache *awscache.UnavailableOfferings
var instanceTypeCache *cache.Cache
var instanceTypeProvider *InstanceTypeProvider
var fakeEC2API *fake.EC2API
var fakePricingAPI *fake.PricingAPI
var prov *provisioning.Provisioner
var provisioningController controller.Controller
var cloudProvider *CloudProvider
var cluster *state.Cluster
var recorder *coretest.EventRecorder
var fakeClock *clock.FakeClock
var provisioner *corev1alpha5.Provisioner
var nodeTemplate *v1alpha1.AWSNodeTemplate
var pricingProvider *PricingProvider
var settingsStore coretest.SettingsStore

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, apis.CRDs...)
	settingsStore = coretest.SettingsStore{
		settings.ContextKey:    coretest.Settings(),
		awssettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)
	ctx, stop = context.WithCancel(ctx)

	launchTemplateCache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	internalUnavailableOfferingsCache = cache.New(awscache.UnavailableOfferingsTTL, awscontext.CacheCleanupInterval)
	unavailableOfferingsCache = awscache.NewUnavailableOfferings(internalUnavailableOfferingsCache)
	securityGroupCache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	subnetCache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	ssmCache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	ec2Cache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	instanceTypeCache = cache.New(InstanceTypesAndZonesCacheTTL, awscontext.CacheCleanupInterval)
	fakeEC2API = &fake.EC2API{}
	fakePricingAPI = &fake.PricingAPI{}
	pricingProvider = NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
	subnetProvider := &SubnetProvider{
		ec2api: fakeEC2API,
		cache:  subnetCache,
		cm:     pretty.NewChangeMonitor(),
	}
	instanceTypeProvider = &InstanceTypeProvider{
		ec2api:               fakeEC2API,
		subnetProvider:       subnetProvider,
		cache:                instanceTypeCache,
		pricingProvider:      pricingProvider,
		unavailableOfferings: unavailableOfferingsCache,
		cm:                   pretty.NewChangeMonitor(),
	}
	securityGroupProvider := &SecurityGroupProvider{
		ec2api: fakeEC2API,
		cache:  securityGroupCache,
		cm:     pretty.NewChangeMonitor(),
	}
	cloudProvider = &CloudProvider{
		instanceTypeProvider: instanceTypeProvider,
		instanceProvider: NewInstanceProvider(ctx, fakeEC2API, instanceTypeProvider, subnetProvider, &LaunchTemplateProvider{
			ec2api:                fakeEC2API,
			amiFamily:             amifamily.New(env.Client, fake.SSMAPI{}, fakeEC2API, ssmCache, ec2Cache),
			kubernetesInterface:   env.KubernetesInterface,
			securityGroupProvider: securityGroupProvider,
			cache:                 launchTemplateCache,
			caBundle:              ptr.String("ca-bundle"),
			cm:                    pretty.NewChangeMonitor(),
		}),
		kubeClient: env.Client,
	}
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(ctx, fakeClock, env.Client, cloudProvider)
	recorder = coretest.NewEventRecorder()
	prov = provisioning.NewProvisioner(ctx, env.Client, env.KubernetesInterface.CoreV1(), recorder, cloudProvider, cluster)
	provisioningController = provisioning.NewController(env.Client, prov, recorder)

	env.CRDDirectoryPaths = append(env.CRDDirectoryPaths, RelativeToRoot("charts/karpenter/crds"))
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	settingsStore = coretest.SettingsStore{
		settings.ContextKey:    coretest.Settings(),
		awssettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)
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

	// Reset the pricing provider, so we don't cross-pollinate pricing data
	pricingProvider = NewPricingProvider(ctx, fakePricingAPI, fakeEC2API, "", false, make(chan struct{}))
	instanceTypeProvider.pricingProvider = pricingProvider

	recorder.Reset()
	fakeEC2API.Reset()
	fakePricingAPI.Reset()
	launchTemplateCache.Flush()
	securityGroupCache.Flush()
	subnetCache.Flush()
	internalUnavailableOfferingsCache.Flush()
	ssmCache.Flush()
	ec2Cache.Flush()
	instanceTypeCache.Flush()
	cloudProvider.instanceProvider.launchTemplateProvider.kubeDNSIP = net.ParseIP("10.0.100.10")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Allocation", func() {
	Context("Defaulting", func() {
		// Intent here is that if updates occur on the provisioningController, the Provisioner doesn't need to be recreated
		It("should not set the InstanceProfile with the default if none provided in Provisioner", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.InstanceProfile).To(BeNil())
		})
		It("should default requirements", func() {
			provisioner.SetDefaults(ctx)
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      corev1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1alpha5.CapacityTypeOnDemand},
			}))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1alpha5.ArchitectureAmd64},
			}))
		})
	})
	Context("EC2 Context", func() {
		It("should set context on the CreateFleet request if specified on the Provisioner", func() {
			provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
			Expect(err).ToNot(HaveOccurred())
			provider.Context = aws.String("context-1234")
			provisioner = coretest.Provisioner(coretest.ProvisionerOptions{Provider: provider})
			provisioner.SetDefaults(ctx)
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.StringValue(createFleetInput.Context)).To(Equal("context-1234"))
		})
		It("should default to no EC2 Context", func() {
			provisioner.SetDefaults(ctx)
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.Context).To(BeNil())
		})
	})
	Context("Provider Backwards Compatibility", func() {
		It("should launch a node using provider defaults", func() {
			provisioner := test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)

			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			firstLt := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))

			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			launchTemplate := createFleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification
			Expect(createFleetInput.LaunchTemplateConfigs).To(HaveLen(1))

			Expect(*createFleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName).
				To(Equal(*firstLt.LaunchTemplateName))
			Expect(firstLt.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
			Expect(*launchTemplate.Version).To(Equal("$Latest"))
		})
		It("should discover security groups by ID", func() {
			provisioner := test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"aws-ids": "sg-test1"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
				"sg-test1",
			))
		})
		It("should discover subnets by ID", func() {
			provisioner := test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"aws-ids": "subnet-test1"},
					SecurityGroupSelector: map[string]string{"*": "*"},
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("subnet-test1"))
		})
		It("should use the instance profile on the Provisioner when specified", func() {
			provisioner := test.Provisioner(coretest.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
					SubnetSelector:        map[string]string{"*": "*"},
					SecurityGroupSelector: map[string]string{"*": "*"},
					InstanceProfile:       aws.String("overridden-profile"),
				},
				Requirements: []v1.NodeSelectorRequirement{{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				}},
			})
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(*input.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("overridden-profile"))
		})
	})
})

func RelativeToRoot(path string) string {
	_, file, _, _ := runtime.Caller(0)
	manifestsRoot := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(manifestsRoot, path)
}
