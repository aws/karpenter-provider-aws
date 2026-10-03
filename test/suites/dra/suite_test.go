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

package dra_test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/drametadata"
	"github.com/aws/karpenter-provider-aws/pkg/providers/efadra"
	"github.com/aws/karpenter-provider-aws/pkg/providers/nvidiadra"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gpuDeviceClass is the DeviceClass the NVIDIA DRA driver chart ships, so this is the name a real
// workload references. efaDeviceClass has no such counterpart: dranet's install manifest ships no
// DeviceClass, so the suite creates one.
const (
	gpuDeviceClass = "gpu.nvidia.com"
	efaDeviceClass = "dra.net"
)

// attributePCIeRoot is the standardized attribute both drivers publish, and the one a topology
// colocation constraint is written against.
const attributePCIeRoot = "resource.kubernetes.io/pcieRoot"

// testableFamilies limits every spec to GPU families that are cheap and hold whole GPUs. The scraped
// metadata covers far more: p-family capacity generally needs a reservation, and the g6f/gr6f
// fractional-L4 types report a partial GPU whose behavior under a multi-device or shared claim isn't
// what these specs are about.
//
// g5g is Graviton and included deliberately. Every x86 multi-GPU type in this list scores 1/10 on spot
// placement in us-west-2 and routinely fails on-demand with InsufficientInstanceCapacity, whereas
// g5g.16xlarge (2 T4Gs behind one PCIe root) has real capacity. It works because the driver image is
// multi-arch and AL2023 publishes an arm64 NVIDIA variant -- see amifamily/al2023.go.
var testableFamilies = []string{"g4dn", "g5", "g5g", "g6", "g6e", "gr6"}

var env *aws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestDRA(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "DRA")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	if env.K8sMinorVersion() < 34 {
		Skip("DRA requires the resource.k8s.io/v1 API, which is only served on EKS 1.34+")
	}
	// Both switches are needed: the AWS gate for Karpenter to contribute DRA templates at all, and the
	// core option to stop ignoring DRA requests. aws.Environment.AfterEach restores the chart's
	// settings, so there is nothing to undo here.
	env.ExpectSettingsOverridden(
		corev1.EnvVar{Name: "AWS_FEATURE_GATES", Value: "NodeClassCEL=false,DRA=true"},
		corev1.EnvVar{Name: "IGNORE_DRA_REQUESTS", Value: "false"},
	)

	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("DRA", func() {
	// These are true end-to-end: the drivers are installed, so Karpenter sizes the node from its own
	// ResourceSliceTemplates, the driver then publishes the real ResourceSlice on that node, and
	// kube-scheduler allocates the claim against it. A pod that goes healthy means both halves agreed.
	//
	// No spec pins a single instance type -- GPU capacity is too patchy for that to be reliable. Each one
	// offers every type in the scraped metadata that has the property under test, plus cheaper decoys
	// that lack it, and asserts the launched node landed in the former set. That keeps Karpenter free to
	// fall back on capacity errors while still failing if it picks a type that can't serve the claim.
	It("should launch a multi-GPU node for a two-device claim", func() {
		installNVIDIADRADriver("")
		// The decoys are single-GPU types, all of them cheaper than anything that can hold two. If the
		// per-device count in the GPU template were wrong or ignored, one of those would win on price.
		//
		// Two rather than four devices: both discriminate against the single-GPU decoys identically, but two
		// also admits g5g.16xlarge, the only multi-GPU type in these families with dependable capacity.
		multiGPU := gpuTypes(atLeastGPUs(2))
		pinInstanceTypes(append(multiGPU, gpuTypes(singleGPU)...)...)
		pod := podClaiming(nil, coretest.ExactDeviceRequest("gpu", gpuDeviceClass, 2))

		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		nodes := env.ExpectCreatedNodeCount("==", 1)
		Expect(multiGPU).To(ContainElement(instanceTypeOf(nodes[0])))
	})
	It("should launch a node whose GPUs share a PCIe root", func() {
		installNVIDIADRADriver("")
		// Cleanly generational: every g4dn/g5 puts multiple GPUs behind one root, every g6/g6e gives each
		// GPU its own. The g6 decoys are the cheaper half, so Karpenter only avoids them because no pair of
		// their GPUs can satisfy a same-root constraint.
		colocatable := gpuTypes(sharesAPCIeRoot)
		pinInstanceTypes(append(colocatable, gpuTypes(rootPerGPU)...)...)
		pod := podClaiming(
			[]resourcev1.DeviceConstraint{coretest.MatchAttributeConstraint(attributePCIeRoot)},
			coretest.ExactDeviceRequest("gpu", gpuDeviceClass, 2),
		)

		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		nodes := env.ExpectCreatedNodeCount("==", 1)
		Expect(colocatable).To(ContainElement(instanceTypeOf(nodes[0])))
	})
	It("should not launch when no instance type can satisfy the PCIe root constraint", func() {
		// The non-vacuity half of the spec above. Every type on offer here has enough GPUs for the claim's
		// count but puts each one on its own root, so the constraint is the only thing that can reject them.
		//
		// Both halves are asserted here rather than relying on the spec above, because the suite runs with
		// --ginkgo.randomize-all and because "no node appeared" is the same observation you would get from
		// a missing DeviceClass or a gate that didn't take effect. The unconstrained pod at the end rules
		// those out: it shares everything with the constrained one except the constraint, so it launching
		// proves the only reason the first one didn't was the pcieRoot values. No driver is installed, so
		// neither pod ever binds -- nodes are the signal.
		env.ExpectCreatedOrUpdated(coretest.DeviceClassWithSelector(gpuDeviceClass, nvidiadra.DriverName))
		pinInstanceTypes(gpuTypes(rootPerGPU)...)
		constrained := podClaiming(
			[]resourcev1.DeviceConstraint{coretest.MatchAttributeConstraint(attributePCIeRoot)},
			coretest.ExactDeviceRequest("gpu", gpuDeviceClass, 2),
		)

		env.ExpectCreated(nodeClass, nodePool, constrained)
		env.ConsistentlyExpectPendingPods(time.Minute, constrained)
		Expect(env.Monitor.CreatedNodeCount()).To(BeZero())

		unconstrained := podClaiming(nil, coretest.ExactDeviceRequest("gpu", gpuDeviceClass, 2))
		env.ExpectCreated(unconstrained)
		env.EventuallyExpectCreatedNodeCount("==", 1)
	})
	It("should run a pod claiming both a GPU and an EFA device", func() {
		installNVIDIADRADriver("")
		installDRANet()
		pinInstanceTypes(gpuTypes(hasEFA)...)
		// Karpenter attaches EFA interfaces only when the NodeClaim requests vpc.amazonaws.com/efa or the
		// EC2NodeClass pins spec.networkInterfaces (pkg/providers/instance/instance.go). A dra.net claim
		// does neither, so without this extended resource the instance launches with a plain ENI, dranet
		// publishes no EFA device, and the claim can never be satisfied.
		pod := podClaiming(nil,
			coretest.ExactDeviceRequest("gpu", gpuDeviceClass, 1),
			coretest.ExactDeviceRequest("efa", efaDeviceClass, 1),
		)
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")}
		pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{v1.ResourceEFA: resource.MustParse("1")}

		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should time-slice one GPU across four claims and launch a second node for the fifth", func() {
		// Time slicing: the driver publishes each GPU with a "shares" capacity of 4, and the annotation
		// tells Karpenter to size nodes the same way. Restricting to single-GPU types is what makes this
		// unambiguous -- with one GPU per node there is no second device for the four claims to spread
		// across, so they only fit on one node if the shared capacity is being modeled.
		installNVIDIADRADriver("4")
		nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
			v1.AnnotationNVIDIAConsumableCapacity: "4",
		})
		singleGPUTypes := gpuTypes(singleGPU)
		pinInstanceTypes(singleGPUTypes...)

		oneShare := coretest.ExactDeviceRequestWithCapacity("gpu", gpuDeviceClass, 1,
			map[resourcev1.QualifiedName]resource.Quantity{nvidiadra.CapacityShares: resource.MustParse("1")})
		pods := lo.Times(4, func(int) *corev1.Pod { return podClaiming(nil, oneShare) })

		env.ExpectCreated(lo.Map(pods, func(p *corev1.Pod, _ int) client.Object { return p })...)
		env.ExpectCreated(nodeClass, nodePool)
		env.EventuallyExpectHealthy(pods...)
		nodes := env.ExpectCreatedNodeCount("==", 1)
		Expect(singleGPUTypes).To(ContainElement(instanceTypeOf(nodes[0])))

		// The ceiling is the other half of the claim: a fifth share doesn't fit on a 4-share GPU, so it has
		// to cost a second node. Without this, a template that advertised unlimited sharing would pass.
		fifth := podClaiming(nil, oneShare)
		env.ExpectCreated(fifth)
		env.EventuallyExpectHealthy(fifth)
		env.EventuallyExpectCreatedNodeCount("==", 2)
	})
})

// gpuTypes returns every scraped instance type in testableFamilies matching predicate. Driving the
// NodePool off the metadata rather than a hard-coded list means a spec offers as many fallbacks as the
// fleet allows, and picks up new instance types as the scrape grows.
func gpuTypes(predicate func(string, *drametadata.DeviceMetadata) bool) []string {
	GinkgoHelper()
	matches := lo.FilterMap(lo.Entries(drametadata.GPUMetadataByInstanceType),
		func(e lo.Entry[string, *drametadata.DeviceMetadata], _ int) (string, bool) {
			family, _, found := strings.Cut(e.Key, ".")
			// Bare-metal types boot far too slowly to sit in the middle of a spec.
			if !found || !lo.Contains(testableFamilies, family) || strings.HasSuffix(e.Key, ".metal") {
				return "", false
			}
			return e.Key, predicate(e.Key, e.Value)
		})
	Expect(matches).ToNot(BeEmpty(), "no testable instance type matches the predicate -- the scraped metadata has changed")
	return matches
}

func atLeastGPUs(count int) func(string, *drametadata.DeviceMetadata) bool {
	return func(_ string, m *drametadata.DeviceMetadata) bool { return len(m.Devices) >= count }
}

func singleGPU(_ string, m *drametadata.DeviceMetadata) bool { return len(m.Devices) == 1 }

// sharesAPCIeRoot and rootPerGPU are the two sides of the colocation question: whether some root holds
// enough GPUs to satisfy a same-root pair, or whether every GPU sits alone. rootPerGPU also requires
// enough GPUs for the claim's count, so a type it selects can only ever fail on the constraint.
func sharesAPCIeRoot(_ string, m *drametadata.DeviceMetadata) bool { return maxGPUsPerRoot(m) >= 2 }
func rootPerGPU(_ string, m *drametadata.DeviceMetadata) bool {
	return len(m.Devices) >= 2 && maxGPUsPerRoot(m) == 1
}

func hasEFA(name string, _ *drametadata.DeviceMetadata) bool {
	_, ok := drametadata.EFAMetadataByInstanceType[name]
	return ok
}

func maxGPUsPerRoot(m *drametadata.DeviceMetadata) int {
	perRoot := map[string]int{}
	for _, device := range m.Devices {
		if attribute, ok := device.Attributes[attributePCIeRoot]; ok && attribute.StringValue != nil {
			perRoot[*attribute.StringValue]++
		}
	}
	return lo.Max(append(lo.Values(perRoot), 0))
}

func instanceTypeOf(node *corev1.Node) string {
	return node.Labels[corev1.LabelInstanceTypeStable]
}

// pinInstanceTypes restricts the NodePool to the given instance types. The default NodePool allows only
// the c/m/r categories, so the category requirement has to be widened alongside the instance type one.
//
// Spot is added to the default's on-demand: GPU capacity is the binding constraint on these specs, and
// allowing both roughly doubles the pools Karpenter can fall back through. Nothing here depends on the
// capacity type, and a spot reclaim mid-spec just costs a relaunch.
func pinInstanceTypes(instanceTypes ...string) {
	GinkgoHelper()
	nodePool = coretest.ReplaceRequirements(nodePool,
		karpv1.NodeSelectorRequirementWithMinValues{
			Key:      v1.LabelInstanceCategory,
			Operator: corev1.NodeSelectorOpExists,
		},
		karpv1.NodeSelectorRequirementWithMinValues{
			Key:      karpv1.CapacityTypeLabelKey,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{karpv1.CapacityTypeSpot, karpv1.CapacityTypeOnDemand},
		},
		karpv1.NodeSelectorRequirementWithMinValues{
			Key:      corev1.LabelInstanceTypeStable,
			Operator: corev1.NodeSelectorOpIn,
			Values:   instanceTypes,
		},
	)
}

// podClaiming returns a pod with a single ResourceClaim carrying the given requests and constraints,
// generated from a ResourceClaimTemplate that is cleaned up with the spec.
func podClaiming(constraints []resourcev1.DeviceConstraint, requests ...resourcev1.DeviceRequest) *corev1.Pod {
	GinkgoHelper()
	template := coretest.ResourceClaimTemplate(resourcev1.ResourceClaimTemplate{
		Spec: resourcev1.ResourceClaimTemplateSpec{
			Spec: resourcev1.ResourceClaimSpec{
				Devices: resourcev1.DeviceClaim{Requests: requests, Constraints: constraints},
			},
		},
	})
	env.ExpectCreated(template)
	// ResourceClaimTemplate isn't in common.CleanableObjects -- listing resource.k8s.io types there
	// would break Cleanup() on clusters that don't serve the group -- so it is removed explicitly. The
	// ResourceClaim it generates is owned by the pod, so deleting the pod collects it.
	DeferCleanup(func() { env.ExpectDeleted(template) })

	return coretest.Pod(coretest.PodOptions{
		ResourceClaims:          []corev1.PodResourceClaim{coretest.PodResourceClaimTemplateReference("claim", template.Name)},
		ContainerResourceClaims: []corev1.ResourceClaim{{Name: "claim"}},
	})
}

// installNVIDIADRADriver installs the NVIDIA GPU DRA driver, which publishes the gpu.nvidia.com
// ResourceSlices and the DeviceClass the specs claim against. The AL2023 NVIDIA AMI variant that
// Karpenter resolves for GPU instance types already carries the kernel driver the plugin needs.
//
// consumableShares mirrors the chart's value of the same name ("" to leave sharing off, otherwise
// "memory", "unlimited", or a positive integer). It has to match the annotation Karpenter is given:
// Karpenter can't read the driver's configuration, so the two are asserted to agree by construction.
func installNVIDIADRADriver(consumableShares string) {
	GinkgoHelper()
	applyManifest("testdata/nvidia-dra-driver.yaml")
	if consumableShares != "" {
		enableConsumableShares(consumableShares)
	}
}

// enableConsumableShares turns on the driver's consumable-capacity support, which is what makes a GPU
// allocatable to more than one claim. The chart expresses this as exactly two env vars on the kubelet
// plugin, so it's patched in rather than carried as a second copy of the whole manifest. Safe to do
// right after the apply: no GPU node exists yet, so the DaemonSet has no pods to roll.
func enableConsumableShares(shares string) {
	GinkgoHelper()
	daemonSet := &appsv1.DaemonSet{}
	Expect(env.Client.Get(env.Context, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "dra-driver-nvidia-gpu-kubelet-plugin",
	}, daemonSet)).To(Succeed())

	container, index, ok := lo.FindIndexOf(daemonSet.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == "gpus"
	})
	Expect(ok).To(BeTrue(), "kubelet plugin has no gpus container -- the vendored manifest has changed shape")
	container.Env = append(container.Env,
		corev1.EnvVar{Name: "CONSUMABLE_SHARES", Value: shares},
		corev1.EnvVar{Name: "FEATURE_GATES", Value: "ConsumableShares=true"},
	)
	daemonSet.Spec.Template.Spec.Containers[index] = container
	env.ExpectUpdated(daemonSet)
}

// installDRANet installs dranet, which publishes the dra.net ResourceSlices for EFA interfaces, plus
// the DeviceClass it ships no manifest for.
func installDRANet() {
	GinkgoHelper()
	applyManifest("testdata/dranet.yaml")
	env.ExpectCreatedOrUpdated(coretest.DeviceClassWithSelector(efaDeviceClass, efadra.DriverName))
}

// applyManifest creates every object in a multi-document YAML file. Created-or-updated rather than
// created: the cluster-scoped driver objects outlive Cleanup() (which only collects the kinds in
// common.CleanableObjects), so a second spec re-applying them must not fail on AlreadyExists.
func applyManifest(path string) {
	GinkgoHelper()
	raw, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())

	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(raw)))
	var objects []client.Object
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		Expect(err).ToNot(HaveOccurred())
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		object := &unstructured.Unstructured{}
		Expect(utilyaml.Unmarshal(doc, &object.Object)).To(Succeed())
		// The header comments decode to an empty document.
		if len(object.Object) == 0 {
			continue
		}
		objects = append(objects, object)
	}
	Expect(objects).ToNot(BeEmpty(), path)
	env.ExpectCreatedOrUpdated(objects...)
}
