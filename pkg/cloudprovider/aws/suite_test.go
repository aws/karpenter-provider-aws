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

package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/vpc"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/amifamily"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/patrickmn/go-cache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
)

var ctx context.Context
var opts options.Options
var env *test.Environment
var launchTemplateCache *cache.Cache
var securityGroupCache *cache.Cache
var subnetCache *cache.Cache
var amiCache *cache.Cache
var unavailableOfferingsCache *cache.Cache
var fakeEC2API *fake.EC2API
var provisioners *provisioning.Controller
var selectionController *selection.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		opts = options.Options{
			ClusterName:               "test-cluster",
			ClusterEndpoint:           "https://test-cluster",
			AWSNodeNameConvention:     string(options.IPName),
			AWSENILimitedPodDensity:   true,
			AWSDefaultInstanceProfile: "test-instance-profile",
		}
		Expect(opts.Validate()).To(Succeed(), "Failed to validate options")
		ctx = injection.WithOptions(ctx, opts)
		launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
		unavailableOfferingsCache = cache.New(InsufficientCapacityErrorCacheTTL, InsufficientCapacityErrorCacheCleanupInterval)
		securityGroupCache = cache.New(CacheTTL, CacheCleanupInterval)
		subnetCache = cache.New(CacheTTL, CacheCleanupInterval)
		amiCache = cache.New(CacheTTL, CacheCleanupInterval)
		fakeEC2API = &fake.EC2API{}
		subnetProvider := &SubnetProvider{
			ec2api: fakeEC2API,
			cache:  subnetCache,
		}
		instanceTypeProvider := &InstanceTypeProvider{
			ec2api:               fakeEC2API,
			subnetProvider:       subnetProvider,
			cache:                cache.New(InstanceTypesAndZonesCacheTTL, CacheCleanupInterval),
			unavailableOfferings: unavailableOfferingsCache,
		}
		securityGroupProvider := &SecurityGroupProvider{
			ec2api: fakeEC2API,
			cache:  securityGroupCache,
		}
		clientSet := kubernetes.NewForConfigOrDie(e.Config)
		cloudProvider := &CloudProvider{
			subnetProvider:       subnetProvider,
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider: &InstanceProvider{
				fakeEC2API, instanceTypeProvider, subnetProvider, &LaunchTemplateProvider{
					ec2api:                fakeEC2API,
					amiFamily:             amifamily.New(fake.SSMAPI{}, amiCache),
					clientSet:             clientSet,
					securityGroupProvider: securityGroupProvider,
					cache:                 launchTemplateCache,
					caBundle:              ptr.String("ca-bundle"),
				},
			},
		}
		registry.RegisterOrDie(ctx, cloudProvider)
		provisioners = provisioning.NewController(ctx, e.Client, clientSet.CoreV1(), cloudProvider)
		selectionController = selection.NewController(e.Client, provisioners)
	})

	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha5.Provisioner
	var provider *v1alpha1.AWS

	BeforeEach(func() {
		provider = &v1alpha1.AWS{
			SubnetSelector:        map[string]string{"foo": "bar"},
			SecurityGroupSelector: map[string]string{"foo": "bar"},
		}
		provisioner = ProvisionerWithProvider(&v1alpha5.Provisioner{ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())}}, provider)
		fakeEC2API.Reset()
		launchTemplateCache.Flush()
		securityGroupCache.Flush()
		subnetCache.Flush()
		unavailableOfferingsCache.Flush()
		amiCache.Flush()
	})

	AfterEach(func() {
		ExpectProvisioningCleanedUp(ctx, env.Client, provisioners)
	})

	Context("Reconciliation", func() {
		Context("Specialized Hardware", func() {
			It("should not launch AWS Pod ENI on a t3", func() {
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						NodeSelector: map[string]string{
							v1.LabelInstanceTypeStable: "t3.large",
						},
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSPodENI: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSPodENI: resource.MustParse("1")},
						},
					})) {
					ExpectNotScheduled(ctx, env.Client, pod)
				}
			})
			It("should launch AWS Pod ENI on a compatible instance type", func() {
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSPodENI: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSPodENI: resource.MustParse("1")},
						},
					})) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
					supportsPodENI := func() bool {
						limits, ok := vpc.Limits[node.Labels[v1.LabelInstanceTypeStable]]
						return ok && limits.IsTrunkingCompatible
					}
					Expect(supportsPodENI()).To(Equal(true))
				}
			})
			// TODO(todd): this set of tests should move to scheduler once resource handling is made more generic
			It("should launch instances for Nvidia GPU resource requests", func() {
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						},
					}),
					// Should pack onto same instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
						},
					}),
					// Should pack onto a separate instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
						},
					})) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"))
					Expect(node.Status.Capacity).To(HaveKeyWithValue(nvidiaGPUResourceName, resource.MustParse("4")))
					nodeNames.Insert(node.Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
			It("should launch pods with Nvidia and Neuron resources on different instances", func() {
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						},
					}),
					// Should pack onto a different instance since no type has both Nvidia and Neuron
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					})) {
					node := ExpectScheduled(ctx, env.Client, pod)
					nodeNames.Insert(node.Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
			It("should fail to schedule a pod with both Nvidia and Neuron resources requests", func() {
				pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1"), resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1"), resources.AWSNeuron: resource.MustParse("1")},
						},
					}))
				ExpectNotScheduled(ctx, env.Client, pods[0])
			})
			It("should not schedule a non-GPU workload on a node w/GPU", func() {
				Skip("enable after scheduling and binpacking are merged into the same process")
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								resources.NvidiaGPU: resource.MustParse("1"),
								"cpu":               resource.MustParse("31"),
							},
							Limits: v1.ResourceList{
								resources.NvidiaGPU: resource.MustParse("1"),
								"cpu":               resource.MustParse("31"),
							},
						},
					}),
					// Can't pack onto the same instance due to consuming too much CPU
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								"cpu": resource.MustParse("3"),
							},
							Limits: v1.ResourceList{
								"cpu": resource.MustParse("3"),
							},
						},
					}),
				) {
					node := ExpectScheduled(ctx, env.Client, pod)

					// This test has a GPU workload that nearly maxes out the test instance type.  It's intended to ensure
					// that the second pod won't get a GPU node since it doesn't require one, even though it's compatible
					// with the first pod that does require a GPU.
					if _, isGpuPod := pod.Spec.Containers[0].Resources.Requests[resources.NvidiaGPU]; isGpuPod {
						Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"))
					} else {
						Expect(node.Labels).ToNot(HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"))
					}
					nodeNames.Insert(node.Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})

			It("should launch instances for AWS Neuron resource requests", func() {
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
					// Should pack onto same instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						},
					}),
					// Should pack onto a separate instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
						},
					}),
				) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.6xlarge"))
					Expect(node.Status.Capacity).To(HaveKeyWithValue(awsNeuronResourceName, resource.MustParse("4")))
					nodeNames.Insert(node.Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
		})
		Context("Insufficient Capacity Error Cache", func() {
			It("should launch instances of different type on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{{CapacityType: v1alpha1.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}}
				pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
					test.UnschedulablePod(test.PodOptions{
						NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"},
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
				)
				// it should've tried to pack them on a single inf1.6xlarge then hit an insufficient capacity error
				for _, pod := range pods {
					ExpectNotScheduled(ctx, env.Client, pod)
				}
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.2xlarge"))
					nodeNames.Insert(node.Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
			It("should launch instances in a different zone on second reconciliation attempt with Insufficient Capacity Error Cache fallback", func() {
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{{CapacityType: v1alpha1.CapacityTypeOnDemand, InstanceType: "p3.8xlarge", Zone: "test-zone-1a"}}
				pod := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
					},
				})
				pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
					{
						Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
							{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
						}},
					},
				}}}
				pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
				// it should've tried to pack them in test-zone-1a on a p3.8xlarge then hit insufficient capacity, the next attempt will try test-zone-1b
				ExpectNotScheduled(ctx, env.Client, pod)

				pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(SatisfyAll(
					HaveKeyWithValue(v1.LabelInstanceTypeStable, "p3.8xlarge"),
					HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1b")))
			})
			It("should launch smaller instances than optimal if larger instance launch results in Insufficient Capacity Error", func() {
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{
					{CapacityType: v1alpha1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
				}
				twoInstanceProvisioner := provisioner.DeepCopy()
				twoInstanceProvisioner.Spec.Constraints.Requirements = twoInstanceProvisioner.Spec.Constraints.Requirements.Add(
					v1.NodeSelectorRequirement{
						Key:      v1.LabelInstanceType,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"m5.large", "m5.xlarge"},
					})
				pods := []*v1.Pod{}
				for i := 0; i < 2; i++ {
					pods = append(pods, test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
						},
						NodeSelector: map[string]string{
							v1.LabelTopologyZone: "test-zone-1a",
						},
					}))
				}

				// The first provision will fail with an Insufficient Capacity Exception on 1 m5.xlarge
				pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, twoInstanceProvisioner, pods...)
				ExpectNotScheduled(ctx, env.Client, pods[0])
				ExpectNotScheduled(ctx, env.Client, pods[1])
				// Provisions 2 m5.large instances since m5.xlarge was ICE'd
				pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, twoInstanceProvisioner, pods...)
				ExpectScheduled(ctx, env.Client, pods[0])
				ExpectScheduled(ctx, env.Client, pods[1])
			})
			It("should launch instances on later reconciliation attempt with Insufficient Capacity Error Cache expiry", func() {
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{{CapacityType: v1alpha1.CapacityTypeOnDemand, InstanceType: "inf1.6xlarge", Zone: "test-zone-1a"}}
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "inf1.6xlarge"},
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						},
					}),
				)[0]
				ExpectNotScheduled(ctx, env.Client, pod)
				// capacity shortage is over - expire the item from the cache and try again
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{}
				unavailableOfferingsCache.Delete(UnavailableOfferingsCacheKey(v1alpha1.CapacityTypeOnDemand, "inf1.6xlarge", "test-zone-1a"))
				pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "inf1.6xlarge"))
			})
			It("should launch on-demand capacity if flexible to both spot and on demand, but spot if unavailable", func() {
				fakeEC2API.InsufficientCapacityPools = []fake.CapacityPool{{CapacityType: v1alpha1.CapacityTypeSpot, InstanceType: "m5.large", Zone: "test-zone-1a"}}
				provisioner.Spec.Requirements = v1alpha5.NewRequirements(
					v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}},
					v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1a"}},
					v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"m5.large"}},
				)
				// Spot Unavailable
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				ExpectNotScheduled(ctx, env.Client, pod)
				// Fallback to OD
				pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeOnDemand))
			})
		})
		Context("CapacityType", func() {
			It("should default to on-demand", func() {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeOnDemand))
			})
			It("should launch spot capacity if flexible to both spot and on demand", func() {
				provisioner.Spec.Requirements = v1alpha5.NewRequirements(
					v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}})
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeSpot))
			})
		})
		Context("LaunchTemplates", func() {
			It("should use same launch template for equivalent constraints", func() {
				t1 := v1.Toleration{
					Key:      "Abacus",
					Operator: "Equal",
					Value:    "Zebra",
					Effect:   "NoSchedule",
				}
				t2 := v1.Toleration{
					Key:      "Zebra",
					Operator: "Equal",
					Value:    "Abacus",
					Effect:   "NoSchedule",
				}
				t3 := v1.Toleration{
					Key:      "Boar",
					Operator: "Equal",
					Value:    "Abacus",
					Effect:   "NoSchedule",
				}

				pod1 := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						Tolerations: []v1.Toleration{t1, t2, t3},
					}),
				)[0]
				ExpectScheduled(ctx, env.Client, pod1)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				name1 := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName

				pod2 := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						Tolerations: []v1.Toleration{t2, t3, t1},
					}),
				)[0]

				ExpectScheduled(ctx, env.Client, pod2)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				name2 := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName
				Expect(name1).To(Equal(name2))
			})

			It("should request that tags be applied to both instances and volumes", func() {
				provider.Tags = map[string]string{
					"tag1": "tag1value",
					"tag2": "tag2value",
				}
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(createFleetInput.TagSpecifications).To(HaveLen(2))

				// tags should be included in both the instance and volume tag specification
				Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
				ExpectTags(createFleetInput.TagSpecifications[0].Tags, provider.Tags)

				Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
				ExpectTags(createFleetInput.TagSpecifications[1].Tags, provider.Tags)
			})

			It("should override default tag names", func() {
				// these tags are defaulted, so ensure users can override them
				provider.Tags = map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "myprovisioner",
					"Name":                           "myname",
				}

				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(createFleetInput.TagSpecifications).To(HaveLen(2))

				// tags should be included in both the instance and volume tag specification
				Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
				ExpectTags(createFleetInput.TagSpecifications[0].Tags, provider.Tags)

				Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
				ExpectTags(createFleetInput.TagSpecifications[1].Tags, provider.Tags)
			})

			It("should default to a generated launch template", func() {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				createLaunchTemplateInput := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(createFleetInput.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := createFleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(*createLaunchTemplateInput.LaunchTemplateName))
				Expect(createLaunchTemplateInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
				Expect(*launchTemplate.Version).To(Equal("$Latest"))
			})
			It("should allow a launch template to be specified", func() {
				provider.LaunchTemplateName = aws.String("test-launch-template")
				provider.SecurityGroupSelector = nil
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal("test-launch-template"))
				Expect(*launchTemplate.Version).To(Equal("$Latest"))
			})
		})
		Context("Subnets", func() {
			It("should default to the cluster's subnets", func() {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod(
					test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureAmd64}}))[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ContainElements(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-1"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1a")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1b")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1c")},
				))
			})
			It("should launch instances into subnet with the most available IP addresses", func() {
				fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
					{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
						Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
					{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(100),
						Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
				}}

				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}}))[0]
				ExpectScheduled(ctx, env.Client, pod)
				createFleetInput := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(aws.StringValue(createFleetInput.LaunchTemplateConfigs[0].Overrides[0].SubnetId)).To(Equal("test-subnet-2"))
			})
		})
		Context("Security Groups", func() {
			It("should default to the clusters security groups", func() {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-1"),
					aws.String("test-security-group-2"),
					aws.String("test-security-group-3"),
				))
			})
			It("should discover security groups by tag", func() {
				fakeEC2API.DescribeSecurityGroupsOutput = &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{
					{GroupId: aws.String("test-sg-1"), Tags: []*ec2.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-1")}}},
					{GroupId: aws.String("test-sg-2"), Tags: []*ec2.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-2")}}},
				}}
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-sg-1"),
					aws.String("test-sg-2"),
				))
			})
		})
		Context("User Data", func() {
			It("should not specify --use-max-pods=false when using ENI-based pod density", func() {
				opts.AWSENILimitedPodDensity = true
				localCtx := injection.WithOptions(ctx, opts)
				pod := ExpectProvisioned(localCtx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(localCtx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				Expect(string(userData)).NotTo(ContainSubstring("--use-max-pods false"))
			})
			It("should specify --use-max-pods=false when not using ENI-based pod density", func() {
				opts.AWSENILimitedPodDensity = false
				localCtx := injection.WithOptions(ctx, opts)
				pod := ExpectProvisioned(localCtx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(localCtx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				Expect(string(userData)).To(ContainSubstring("--use-max-pods false"))
				Expect(string(userData)).To(ContainSubstring("--max-pods=110"))
			})
			Context("Kubelet Args", func() {
				It("should specify the --dns-cluster-ip flag when clusterDNSIP is set", func() {
					provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{ClusterDNS: []string{"10.0.10.100"}}
					pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
					ExpectScheduled(ctx, env.Client, pod)
					Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
					input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
					userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
					Expect(string(userData)).To(ContainSubstring("--dns-cluster-ip '10.0.10.100'"))
				})
			})
			Context("Instance Profile", func() {
				It("should use the default instance profile if none specified on the Provisioner", func() {
					provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{ClusterDNS: []string{"10.0.10.100"}}
					pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
					ExpectScheduled(ctx, env.Client, pod)
					Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
					input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
					Expect(*input.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("test-instance-profile"))
				})
				It("should use the instance profile on the Provisioner when specified", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.InstanceProfile = aws.String("overridden-profile")
					ProvisionerWithProvider(&v1alpha5.Provisioner{ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())}}, provider)
					pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
					ExpectScheduled(ctx, env.Client, pod)
					Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
					input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
					Expect(*input.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("overridden-profile"))
				})
			})
		})
		Context("Metadata Options", func() {
			It("should default metadata options on generated launch template", func() {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit).To(Equal(int64(2)))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2.LaunchTemplateHttpTokensStateRequired))
			})
			It("should set metadata options on generated launch template from provisioner configuration", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateDisabled),
					HTTPProtocolIPv6:        aws.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled),
					HTTPPutResponseHopLimit: aws.Int64(1),
					HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateOptional),
				}
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2.LaunchTemplateInstanceMetadataEndpointStateDisabled))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit).To(Equal(int64(1)))
				Expect(*input.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2.LaunchTemplateHttpTokensStateOptional))
			})
		})
		Context("Block Device Mappings", func() {
			It("should default AL2 block device mappings", func() {
				provider, _ := ProviderFromProvisioner(provisioner)
				provider.AMIFamily = &v1alpha1.AMIFamilyAL2
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(20)))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
				Expect(input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
			})
			It("should use custom block device mapping", func() {
				provider, _ := ProviderFromProvisioner(provisioner)
				provider.AMIFamily = &v1alpha1.AMIFamilyAL2
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							DeleteOnTermination: aws.Bool(true),
							Encrypted:           aws.Bool(true),
							VolumeType:          aws.String("io2"),
							VolumeSize:          resource.NewScaledQuantity(40, resource.Giga),
							IOPS:                aws.Int64(10_000),
							KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
						},
					},
				}
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(40)))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("io2"))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(Equal(int64(10_000)))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.DeleteOnTermination).To(BeTrue())
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(BeTrue())
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.KmsKeyId).To(Equal("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"))
			})
			It("should default bottlerocket second volume with root volume size", func() {
				provider, _ := ProviderFromProvisioner(provisioner)
				//provider.BlockDeviceMappings = nil
				provider.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(2))
				// Bottlerocket control volume
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(4)))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
				Expect(input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
				// Bottlerocket user volume
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize).To(Equal(int64(20)))
				Expect(*input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeType).To(Equal("gp3"))
				Expect(input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.Iops).To(BeNil())
			})
		})
	})
	Context("Defaulting", func() {
		// Intent here is that if updates occur on the controller, the Provisioner doesn't need to be recreated
		It("should not set the InstanceProfile with the default if none provided in Provisioner", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.Deserialize(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.InstanceProfile).To(BeNil())
		})

		It("should default requirements", func() {
			provisioner.SetDefaults(ctx)
			Expect(provisioner.Spec.Requirements.CapacityTypes().UnsortedList()).To(ConsistOf(v1alpha1.CapacityTypeOnDemand))
			Expect(provisioner.Spec.Requirements.Architectures().UnsortedList()).To(ConsistOf(v1alpha5.ArchitectureAmd64))
		})
	})
	Context("Validation", func() {
		It("should validate", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should not panic if provider undefined", func() {
			provisioner.Spec.Provider = nil
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})

		Context("SubnetSelector", func() {
			It("should not allow empty string keys or values", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SubnetSelector = map[string]string{key: value}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("SecurityGroupSelector", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.SecurityGroupSelector = map[string]string{"key": "value"}
				provisioner := ProvisionerWithProvider(provisioner, provider)
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should not allow empty string keys or values", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SecurityGroupSelector = map[string]string{key: value}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("Labels", func() {
			It("should not allow unrecognized labels with the aws label prefix", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support a capacity type label", func() {
				for _, value := range []string{v1alpha1.CapacityTypeOnDemand, v1alpha1.CapacityTypeSpot} {
					provisioner.Spec.Labels = map[string]string{v1alpha5.LabelCapacityType: value}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
		})
		Context("MetadataOptions", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.MetadataOptions = &v1alpha1.MetadataOptions{}
				provisioner := ProvisionerWithProvider(provisioner, provider)
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should allow missing values", func() {
				provider, err := ProviderFromProvisioner(provisioner)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{}
				provisioner := ProvisionerWithProvider(provisioner, provider)
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			Context("HTTPEndpoint", func() {
				It("should allow enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					for _, value := range ec2.LaunchTemplateInstanceMetadataEndpointState_Values() {
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPEndpoint: &value,
						}
						provisioner := ProvisionerWithProvider(provisioner, provider)
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPEndpoint: aws.String(randomdata.SillyName()),
					}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPProtocolIpv6", func() {
				It("should allow enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					for _, value := range ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values() {
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPProtocolIPv6: &value,
						}
						provisioner := ProvisionerWithProvider(provisioner, provider)
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPProtocolIPv6: aws.String(randomdata.SillyName()),
					}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPPutResponseHopLimit", func() {
				It("should validate inside accepted range", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPPutResponseHopLimit: aws.Int64(int64(randomdata.Number(1, 65))),
					}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should not validate outside accepted range", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{}
					// We expect to be able to invalidate any hop limit between
					// [math.MinInt64, 1). But, to avoid a panic here, we can't
					// exceed math.MaxInt for the difference between bounds of
					// the random number range. So we divide the range
					// approximately in half and test on both halves.
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64, math.MinInt64/2)))
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64/2, 1)))
					provisioner = ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())

					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(65, math.MaxInt64)))
					provisioner = ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("HTTPTokens", func() {
				It("should allow enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					for _, value := range ec2.LaunchTemplateHttpTokensState_Values() {
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPTokens: aws.String(value),
						}
						provisioner := ProvisionerWithProvider(provisioner, provider)
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPTokens: aws.String(randomdata.SillyName()),
					}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
			Context("BlockDeviceMappings", func() {
				It("should not allow with a custom launch template", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.LaunchTemplateName = aws.String("my-lt")
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should validate minimal device mapping", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should validate ebs device mapping with snapshotID only", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							SnapshotID: aws.String("snap-0123456789"),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should not allow volume size below minimum", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(100, resource.Mega),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow volume size above max", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow nil device name", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow nil volume size", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS:        &v1alpha1.BlockDevice{},
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should not allow empty ebs block", func() {
					provider, err := ProviderFromProvisioner(provisioner)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
					}}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
		})
	})
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(tags []*ec2.Tag, expected map[string]string) {
	existingTags := map[string]string{}
	for _, tag := range tags {
		existingTags[*tag.Key] = *tag.Value
	}
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		Expect(ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		Expect(foundValue).To(Equal(expValue))
	}
}

func ProvisionerWithProvider(provisioner *v1alpha5.Provisioner, provider *v1alpha1.AWS) *v1alpha5.Provisioner {
	raw, err := json.Marshal(provider)
	Expect(err).ToNot(HaveOccurred())
	provisioner.Spec.Constraints.Provider = &runtime.RawExtension{Raw: raw}
	provisioner.Spec.Limits = &v1alpha5.Limits{Resources: v1.ResourceList{}}
	provisioner.Spec.Limits.Resources[v1.ResourceCPU] = *resource.NewScaledQuantity(10, 0)
	return provisioner
}

func ProviderFromProvisioner(provisioner *v1alpha5.Provisioner) (*v1alpha1.AWS, error) {
	constraints, err := v1alpha1.Deserialize(&provisioner.Spec.Constraints)
	return constraints.AWS, err
}

func InstancesLaunchedFrom(createFleetInputIter <-chan interface{}) int {
	instancesLaunched := 0
	for input := range createFleetInputIter {
		createFleetInput := input.(*ec2.CreateFleetInput)
		instancesLaunched += int(*createFleetInput.TargetCapacitySpecification.TotalTargetCapacity)
	}
	return instancesLaunched
}
