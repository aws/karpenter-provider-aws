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
	"math"
	"net"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	awscache "github.com/aws/karpenter/pkg/cloudproviders/aws/cache"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/cloudprovider/amifamily"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/operator/injection"
	"github.com/aws/karpenter/pkg/operator/options"
	. "github.com/aws/karpenter/pkg/test/expectations"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"

	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/fake"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/pretty"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options
var env *test.Environment
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
var controller *provisioning.Controller
var cloudProvider *CloudProvider
var clientSet *kubernetes.Clientset
var cluster *state.Cluster
var recorder *test.EventRecorder
var cfg *test.Config
var fakeClock *clock.FakeClock
var provisioner *v1alpha5.Provisioner
var provider *awsv1alpha1.AWS
var pricingProvider *PricingProvider

var defaultOpts = options.Options{
	ClusterName:               "test-cluster",
	ClusterEndpoint:           "https://test-cluster",
	AWSNodeNameConvention:     string(options.IPName),
	AWSENILimitedPodDensity:   true,
	AWSEnablePodENI:           true,
	AWSDefaultInstanceProfile: "test-instance-profile",
}

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		opts = defaultOpts
		Expect(opts.Validate()).To(Succeed(), "Failed to validate options")
		ctx = injection.WithOptions(ctx, opts)
		ctx, stop = context.WithCancel(ctx)
		launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
		internalUnavailableOfferingsCache = cache.New(awscache.UnavailableOfferingsTTL, CacheCleanupInterval)
		unavailableOfferingsCache = awscache.NewUnavailableOfferings(internalUnavailableOfferingsCache)
		securityGroupCache = cache.New(CacheTTL, CacheCleanupInterval)
		subnetCache = cache.New(CacheTTL, CacheCleanupInterval)
		ssmCache = cache.New(CacheTTL, CacheCleanupInterval)
		ec2Cache = cache.New(CacheTTL, CacheCleanupInterval)
		instanceTypeCache = cache.New(InstanceTypesAndZonesCacheTTL, CacheCleanupInterval)
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
		clientSet = kubernetes.NewForConfigOrDie(e.Config)
		cloudProvider = &CloudProvider{
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider: NewInstanceProvider(ctx, fakeEC2API, instanceTypeProvider, subnetProvider, &LaunchTemplateProvider{
				ec2api:                fakeEC2API,
				amiFamily:             amifamily.New(ctx, fake.SSMAPI{}, fakeEC2API, ssmCache, ec2Cache, e.Client),
				clientSet:             clientSet,
				securityGroupProvider: securityGroupProvider,
				cache:                 launchTemplateCache,
				caBundle:              ptr.String("ca-bundle"),
				cm:                    pretty.NewChangeMonitor(),
			}),
			kubeClient: e.Client,
		}
		v1alpha5.DefaultHook = cloudProvider.Default
		v1alpha5.ValidateHook = cloudProvider.Validate
		cfg = test.NewConfig()
		fakeClock = clock.NewFakeClock(time.Now())
		cluster = state.NewCluster(fakeClock, cfg, e.Client, cloudProvider)
		recorder = test.NewEventRecorder()
		prov := provisioning.NewProvisioner(ctx, cfg, e.Client, corev1.NewForConfigOrDie(e.Config), recorder, cloudProvider, cluster)
		controller = provisioning.NewController(e.Client, prov, recorder)
	})

	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	provider = &awsv1alpha1.AWS{
		AMIFamily:             aws.String(awsv1alpha1.AMIFamilyAL2),
		SubnetSelector:        map[string]string{"*": "*"},
		SecurityGroupSelector: map[string]string{"*": "*"},
	}
	provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
	opts = defaultOpts
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
		// Intent here is that if updates occur on the controller, the Provisioner doesn't need to be recreated
		It("should not set the InstanceProfile with the default if none provided in Provisioner", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.InstanceProfile).To(BeNil())
		})
		It("should default requirements", func() {
			provisioner.SetDefaults(ctx)
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.CapacityTypeOnDemand},
			}))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			}))
		})
		It("should default requirements hooks in webhook mode", func() {
			// clear our hook to ensure that creating the cloud provider in webhook mode sets it
			v1alpha5.DefaultHook = func(ctx context.Context, provisoner *v1alpha5.Provisioner) {}
			NewCloudProvider(ctx, cloudprovider.Options{WebhookOnly: true})
			v1alpha5.DefaultHook(ctx, provisioner)
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.CapacityTypeOnDemand},
			}))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			}))
		})
	})
	Context("Validation", func() {
		It("should validate", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should succeed if provider undefined", func() {
			provisioner.Spec.Provider = nil
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})

		Context("SubnetSelector", func() {
			It("should not allow empty string keys or values", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SubnetSelector = map[string]string{key: value}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("SecurityGroupSelector", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.SecurityGroupSelector = map[string]string{"key": "value"}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should not allow empty string keys or values", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SecurityGroupSelector = map[string]string{key: value}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("EC2 Context", func() {
			It("should set context on the CreateFleet request if specified on the Provisioner", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.Context = aws.String("context-1234")
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				provisioner.SetDefaults(ctx)
				ExpectApplied(ctx, env.Client, provisioner)
				pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Len()).To(Equal(1))
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop()
				Expect(aws.StringValue(createFleetInput.Context)).To(Equal("context-1234"))
			})
			It("should default to no EC2 Context", func() {
				provisioner.SetDefaults(ctx)
				ExpectApplied(ctx, env.Client, provisioner)
				pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Len()).To(Equal(1))
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop()
				Expect(createFleetInput.Context).To(BeNil())
			})
		})
		Context("Labels", func() {
			It("should not allow unrecognized labels with the aws label prefix", func() {
				provisioner.Spec.Labels = map[string]string{awsv1alpha1.LabelDomain + "/" + randomdata.SillyName(): randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support well known labels", func() {
				for _, label := range []string{
					awsv1alpha1.LabelInstanceHypervisor,
					awsv1alpha1.LabelInstanceFamily,
					awsv1alpha1.LabelInstanceSize,
					awsv1alpha1.LabelInstanceCPU,
					awsv1alpha1.LabelInstanceMemory,
					awsv1alpha1.LabelInstanceGPUName,
					awsv1alpha1.LabelInstanceGPUManufacturer,
					awsv1alpha1.LabelInstanceGPUCount,
					awsv1alpha1.LabelInstanceGPUMemory,
				} {
					provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
		})
		Context("MetadataOptions", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.MetadataOptions = &awsv1alpha1.MetadataOptions{}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should allow missing values", func() {
				provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &awsv1alpha1.MetadataOptions{}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			Context("HTTPEndpoint", func() {
				It("should allow enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					for i := range ec2.LaunchTemplateInstanceMetadataEndpointState_Values() {
						value := ec2.LaunchTemplateInstanceMetadataEndpointState_Values()[i]
						provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
							HTTPEndpoint: &value,
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
						HTTPEndpoint: aws.String(randomdata.SillyName()),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPProtocolIpv6", func() {
				It("should allow enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					for i := range ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values() {
						value := ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values()[i]
						provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
							HTTPProtocolIPv6: &value,
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
						HTTPProtocolIPv6: aws.String(randomdata.SillyName()),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPPutResponseHopLimit", func() {
				It("should validate inside accepted range", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
						HTTPPutResponseHopLimit: aws.Int64(int64(randomdata.Number(1, 65))),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should not validate outside accepted range", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &awsv1alpha1.MetadataOptions{}
					// We expect to be able to invalidate any hop limit between
					// [math.MinInt64, 1). But, to avoid a panic here, we can't
					// exceed math.MaxInt for the difference between bounds of
					// the random number range. So we divide the range
					// approximately in half and test on both halves.
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64, math.MinInt64/2)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64/2, 1)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())

					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(65, math.MaxInt64)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPTokens", func() {
				It("should allow enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					for _, value := range ec2.LaunchTemplateHttpTokensState_Values() {
						provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
							HTTPTokens: aws.String(value),
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &awsv1alpha1.MetadataOptions{
						HTTPTokens: aws.String(randomdata.SillyName()),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("BlockDeviceMappings", func() {
				It("should not allow with a custom launch template", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.LaunchTemplateName = aws.String("my-lt")
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &awsv1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should validate minimal device mapping", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &awsv1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should validate ebs device mapping with snapshotID only", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &awsv1alpha1.BlockDevice{
							SnapshotID: aws.String("snap-0123456789"),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should not allow volume size below minimum", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &awsv1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(100, resource.Mega),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow volume size above max", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &awsv1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow nil device name", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						EBS: &awsv1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow nil volume size", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS:        &awsv1alpha1.BlockDevice{},
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow empty ebs block", func() {
					provider, err := awsv1alpha1.Deserialize(provisioner.Spec.Provider)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*awsv1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
					}}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
		})
	})

	Context("Webhook", func() {
		It("should validate when in webhook mode", func() {
			cp := NewCloudProvider(ctx, cloudprovider.Options{WebhookOnly: true})
			// just ensures that validation doesn't depend on anything as when created for the webhook
			// we don't fully initialize the cloud provider
			Expect(cp.Validate(ctx, provisioner)).To(Succeed())
		})
		It("should default when in webhookmode", func() {
			cp := NewCloudProvider(ctx, cloudprovider.Options{WebhookOnly: true})
			// just ensures that validation doesn't depend on anything as when created for the webhook
			// we don't fully initialize the cloud provider
			cp.Default(ctx, provisioner)
		})
	})
})
