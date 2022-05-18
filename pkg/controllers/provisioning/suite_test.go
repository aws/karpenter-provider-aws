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

package provisioning_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/controllers/state"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/test"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var controller *provisioning.Controller
var env *test.Environment
var recorder *test.EventRecorder
var cfg *test.Config

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Provisioning")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		recorder = test.NewEventRecorder()
		cfg = test.NewConfig()
		controller = provisioning.NewController(ctx, cfg, e.Client, corev1.NewForConfigOrDie(e.Config), recorder, cloudProvider, state.NewCluster(ctx, e.Client))
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Provisioning", func() {
	It("should provision nodes", func() {
		ExpectApplied(ctx, env.Client, test.Provisioner())
		pods := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		for _, pod := range pods {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes for pods with supported node selectors", func() {
		provisioner := test.Provisioner()
		schedulable := []*v1.Pod{
			// Constrained by provisioner
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}}),
			// Constrained by zone
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}),
			// Constrained by instanceType
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"}}),
			// Constrained by architecture
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "arm64"}}),
			// Constrained by operatingSystem
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "linux"}}),
		}
		unschedulable := []*v1.Pod{
			// Ignored, matches another provisioner
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: "unknown"}}),
			// Ignored, invalid zone
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}}),
			// Ignored, invalid instance type
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "unknown"}}),
			// Ignored, invalid architecture
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "unknown"}}),
			// Ignored, invalid operating system
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "unknown"}}),
			// Ignored, invalid capacity type
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: "unknown"}}),
			// Ignored, label selector does not match
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
		}
		ExpectApplied(ctx, env.Client, provisioner)
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller, schedulable...) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller, unschedulable...) {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes for accelerators", func() {
		ExpectApplied(ctx, env.Client, test.Provisioner())
		for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")}},
			}),
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1alpha1.ResourceAMDGPU: resource.MustParse("1")}},
			}),
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{v1alpha1.ResourceAWSNeuron: resource.MustParse("1")}},
			}),
		) {
			ExpectScheduled(ctx, env.Client, pod)
		}
	})
	Context("Resource Limits", func() {
		It("should not schedule when limits are exceeded", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20")},
				Status: v1alpha5.ProvisionerStatus{
					Resources: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("100"),
					},
				},
			}))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule if limits would be met", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						// requires a 2 CPU node, but leaves room for overhead
						v1.ResourceCPU: resource.MustParse("1.75"),
					},
				}}))[0]
			// A 2 CPU node can be launched
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should partially schedule if limits would be exceeded", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3")},
			}))

			// prevent these pods from scheduling on the same node
			opts := test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "foo"},
				},
				PodAntiRequirements: []v1.PodAffinityTerm{
					{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "foo",
							},
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("1.5"),
					}}}
			pods := ExpectProvisioned(ctx, env.Client, controller,
				test.UnschedulablePod(opts),
				test.UnschedulablePod(opts),
			)
			scheduledPodCount := 0
			unscheduledPodCount := 0
			pod0 := ExpectPodExists(ctx, env.Client, pods[0].Name, pods[0].Namespace)
			pod1 := ExpectPodExists(ctx, env.Client, pods[1].Name, pods[1].Namespace)
			if pod0.Spec.NodeName == "" {
				unscheduledPodCount++
			} else {
				scheduledPodCount++
			}
			if pod1.Spec.NodeName == "" {
				unscheduledPodCount++
			} else {
				scheduledPodCount++
			}
			Expect(scheduledPodCount).To(Equal(1))
			Expect(unscheduledPodCount).To(Equal(1))
		})
		It("should not schedule if limits would be exceeded", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{
				Limits: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("2.1"),
					},
				}}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if limits would be exceeded (GPU)", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{
				Limits: v1.ResourceList{v1.ResourcePods: resource.MustParse("1")},
			}))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1"),
					},
				}}))[0]
			// only available instance type has 2 GPUs which would exceed the limit
			ExpectNotScheduled(ctx, env.Client, pod)
		})
	})
	Context("Daemonsets and Node Overhead", func() {
		It("should account for overhead", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should account for overhead (with startup taint)", func() {
			provisioner := test.Provisioner(test.ProvisionerOptions{
				StartupTaints: []v1.Taint{{Key: "foo.com/taint", Effect: v1.TaintEffectNoSchedule}},
			})

			ExpectApplied(ctx, env.Client, provisioner, test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should not schedule if overhead is too large", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("10000Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if resource requests are not defined and limits (requests) are too large", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("10000Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule based on the max resource requests of containers and initContainers", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("2"), v1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
					},
					InitResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("2Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
		It("should not schedule if combined max resources are too large for any node", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
					InitResourceRequirements: v1.ResourceRequirements{
						Limits:   v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("10000Gi")},
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
					},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if initContainer resources are too large", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					InitResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("10000Gi")},
					},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should be able to schedule pods if resource requests and limits are not defined", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should ignore daemonsets without matching tolerations", func() {
			ExpectApplied(ctx, env.Client,
				test.Provisioner(test.ProvisionerOptions{Taints: []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoSchedule}}}),
				test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{
					Tolerations:          []v1.Toleration{{Operator: v1.TolerationOperator(v1.NodeSelectorOpExists)}},
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("2")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
		})
		It("should ignore daemonsets with an invalid selector", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodeSelector:         map[string]string{"node": "invalid"},
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("2")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
		})
		It("should account daemonsets with NotIn operator and unspecified key", func() {
			ExpectApplied(ctx, env.Client, test.Provisioner(), test.DaemonSet(
				test.DaemonSetOptions{PodOptions: test.PodOptions{
					NodeRequirements:     []v1.NodeSelectorRequirement{{Key: "foo", Operator: v1.NodeSelectorOpNotIn, Values: []string{"bar"}}},
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements:     []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}},
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
	})
	Context("Labels", func() {
		It("should label nodes", func() {
			provisioner := test.Provisioner(test.ProvisionerOptions{Labels: map[string]string{"test-key": "test-value", "test-key-2": "test-value-2"}})
			ExpectApplied(ctx, env.Client, provisioner)
			for _, pod := range ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod()) {
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.ProvisionerNameLabelKey, provisioner.Name))
				Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
				Expect(node.Labels).To(HaveKeyWithValue("test-key-2", "test-value-2"))
				Expect(node.Labels).To(HaveKey(v1.LabelTopologyZone))
				Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
			}
		})
	})
	Context("Taints", func() {
		It("should schedule pods that tolerate taints", func() {
			provisioner := test.Provisioner(test.ProvisionerOptions{Taints: []v1.Taint{{Key: "nvidia.com/gpu", Value: "true", Effect: v1.TaintEffectNoSchedule}}})
			ExpectApplied(ctx, env.Client, provisioner)
			for _, pod := range ExpectProvisioned(ctx, env.Client, controller,
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpEqual,
							Value:    "true",
							Effect:   v1.TaintEffectNoSchedule,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
						},
					}}),
				test.UnschedulablePod(
					test.PodOptions{Tolerations: []v1.Toleration{
						{
							Operator: v1.TolerationOpExists,
						},
					}}),
			) {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})
	})
})

var _ = Describe("Volume Topology Requirements", func() {
	var storageClass *storagev1.StorageClass
	BeforeEach(func() {
		storageClass = test.StorageClass(test.StorageClassOptions{Zones: []string{"test-zone-2", "test-zone-3"}})
	})
	It("should not schedule if invalid pvc", func() {
		ExpectApplied(ctx, env.Client, test.Provisioner())
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{"invalid"},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should schedule valid pods when a pod with an invalid pvc is encountered", func() {
		ExpectApplied(ctx, env.Client, test.Provisioner())
		invalidPod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{"invalid"},
		}))[0]
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{}))[0]
		ExpectNotScheduled(ctx, env.Client, invalidPod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should schedule to storage class zones if volume does not exist", func() {
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
		ExpectApplied(ctx, env.Client, test.Provisioner(), storageClass, persistentVolumeClaim)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"},
			}},
		}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
	})
	It("should not schedule if storage class zones are incompatible", func() {
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
		ExpectApplied(ctx, env.Client, test.Provisioner(), storageClass, persistentVolumeClaim)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
			}},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should schedule to volume zones if volume already bound", func() {
		persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
		ExpectApplied(ctx, env.Client, test.Provisioner(), storageClass, persistentVolumeClaim, persistentVolume)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
		}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
	})
	It("should not schedule if volume zones are incompatible", func() {
		persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
		ExpectApplied(ctx, env.Client, test.Provisioner(), storageClass, persistentVolumeClaim, persistentVolume)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
			}},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
})

var _ = Describe("Preferential Fallback", func() {
	Context("Required", func() {
		It("should not relax the final term", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}}, // Should not be relaxed
				}},
			}}}}
			// Don't relax
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{Requirements: []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}}))
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should relax multiple terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}, // OR operator, never get to this one
				}},
			}}}}
			// Success
			ExpectApplied(ctx, env.Client, test.Provisioner())
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1"))
		})
	})
	Context("Preferences", func() {
		It("should relax all node affinity terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
			}}}
			// Success
			ExpectApplied(ctx, env.Client, test.Provisioner())
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should relax to use lighter weights", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 100, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-3"}},
					}},
				},
				{
					Weight: 50, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{ // OR operator, never get to this one
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
					}},
				},
			}}}
			// Success
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{Requirements: []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}))
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should tolerate PreferNoSchedule taint only after trying to relax Affinity terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
			}}}
			// Success
			ExpectApplied(ctx, env.Client, test.Provisioner(test.ProvisionerOptions{Taints: []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectPreferNoSchedule}}}))
			pod = ExpectProvisioned(ctx, env.Client, controller, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Spec.Taints).To(ContainElement(v1.Taint{Key: "foo", Value: "bar", Effect: v1.TaintEffectPreferNoSchedule}))
		})
	})
})

var _ = Describe("Multiple Provisioners", func() {
	It("should schedule to an explicitly selected provisioner", func() {
		provisioner := test.Provisioner()
		ExpectApplied(ctx, env.Client, provisioner, test.Provisioner())
		pod := ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner.Name))
	})
	It("should schedule to a provisioner by labels", func() {
		provisioner := test.Provisioner(test.ProvisionerOptions{Labels: map[string]string{"foo": "bar"}})
		ExpectApplied(ctx, env.Client, provisioner, test.Provisioner())
		ExpectProvisioned(ctx, env.Client, controller)
		pod := ExpectProvisioned(ctx, env.Client, controller,
			test.UnschedulablePod(test.PodOptions{NodeSelector: provisioner.Spec.Labels}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner.Name))
	})
	It("should not match provisioner with PreferNoSchedule taint when other provisioner match", func() {
		provisioner := test.Provisioner(test.ProvisionerOptions{Taints: []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectPreferNoSchedule}}})
		ExpectApplied(ctx, env.Client, provisioner, test.Provisioner())
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).ToNot(Equal(provisioner.Name))
	})
})
