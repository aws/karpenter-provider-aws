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

package storage

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/samber/lo"
	"github.com/samber/lo/mutable"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/test"
)

var Env *environmentaws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	Describe("Persistent Volumes", func() {
		Context("Static", func() {
			It("should run a pod with a pre-bound persistent volume (empty storage class)", func() {
				pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					VolumeName:       "test-volume",
					StorageClassName: lo.ToPtr(""),
				})
				pv := test.PersistentVolume(test.PersistentVolumeOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: pvc.Spec.VolumeName,
					},
				})
				pod := test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})

				Env.ExpectCreated(NodeClass, NodePool, pv, pvc, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
			It("should run a pod with a pre-bound persistent volume (non-existent storage class)", func() {
				pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					VolumeName:       "test-volume",
					StorageClassName: lo.ToPtr("non-existent-storage-class"),
				})
				pv := test.PersistentVolume(test.PersistentVolumeOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: pvc.Spec.VolumeName,
					},
					StorageClassName: "non-existent-storage-class",
				})
				pod := test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})
				Env.ExpectCreated(NodeClass, NodePool, pv, pvc, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
			It("should run a pod with a pre-bound persistent volume while respecting topology constraints", func() {
				subnets := Env.DiscoveredSubnets()
				shuffledAZs := lo.Keys(subnets)
				mutable.Shuffle(shuffledAZs)

				pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					StorageClassName: lo.ToPtr("non-existent-storage-class"),
				})
				pv := test.PersistentVolume(test.PersistentVolumeOptions{
					StorageClassName: "non-existent-storage-class",
					Zones:            []string{shuffledAZs[0]},
				})
				pod := test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})
				Env.ExpectCreated(NodeClass, NodePool, pv, pvc, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
			It("should run a pod with a generic ephemeral volume", func() {
				pv := test.PersistentVolume(test.PersistentVolumeOptions{
					StorageClassName: "non-existent-storage-class",
				})
				pod := test.Pod(test.PodOptions{
					EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{{
						StorageClassName: lo.ToPtr("non-existent-storage-class"),
					}},
				})

				Env.ExpectCreated(NodeClass, NodePool, pv, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
		})

		Context("Dynamic", func() {
			var storageClass *storagev1.StorageClass
			BeforeEach(func() {
				Env.RequireEBSCSIDriver()
				storageClass = test.StorageClass(test.StorageClassOptions{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-storage-class",
					},
					Provisioner:       awssdk.String(Env.EBSCSIDriver.Provisioner),
					VolumeBindingMode: lo.ToPtr(storagev1.VolumeBindingWaitForFirstConsumer),
				})
			})

			It("should run a pod with a dynamic persistent volume", func() {
				pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					StorageClassName: &storageClass.Name,
				})
				pod := test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})

				Env.ExpectCreated(NodeClass, NodePool, storageClass, pvc, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
			It("should run a pod with a dynamic persistent volume while respecting allowed topologies", func() {
				subnets := Env.DiscoveredSubnets()
				shuffledAZs := lo.Keys(subnets)
				mutable.Shuffle(shuffledAZs)

				storageClass.AllowedTopologies = []corev1.TopologySelectorTerm{{
					MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{{
						Key:    Env.EBSCSIDriver.ZoneLabel,
						Values: []string{shuffledAZs[0]},
					}},
				}}

				pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					StorageClassName: &storageClass.Name,
				})
				pod := test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})

				Env.ExpectCreated(NodeClass, NodePool, storageClass, pvc, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
			It("should run a pod with a dynamic persistent volume while respecting volume limits", func() {
				ExpectSetEBSDriverLimit(1)
				DeferCleanup(func() {
					ExpectRemoveEBSDriverLimit()
				})

				count := 2
				pvcs := lo.Times(count, func(_ int) *corev1.PersistentVolumeClaim {
					return test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
						StorageClassName: &storageClass.Name,
					})
				})
				pods := lo.Map(pvcs, func(pvc *corev1.PersistentVolumeClaim, _ int) *corev1.Pod {
					return test.Pod(test.PodOptions{
						PersistentVolumeClaims: []string{pvc.Name},
					})
				})

				// Pod creation is spread out here to address an upstream issue where pods can schedule to the same node
				// and exceed its volume limits before the csi driver reports limits. This is not something Karpenter
				// currently compensates for.
				Env.ExpectCreated(NodeClass, NodePool, storageClass, pvcs[0], pods[0])
				Env.EventuallyExpectHealthy(pods[0])
				Env.ExpectCreatedNodeCount("==", 1)
				Env.ExpectCreated(pvcs[1], pods[1])
				Env.EventuallyExpectHealthy(pods[1])
				Env.ExpectCreatedNodeCount("==", 2)
			})
			It("should run a pod with a generic ephemeral volume", func() {
				pod := test.Pod(test.PodOptions{
					EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{{
						StorageClassName: &storageClass.Name,
					}},
				})

				Env.ExpectCreated(NodeClass, NodePool, storageClass, pod)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
			})
		})
	})

	Describe("Stateful workloads", func() {
		var numPods int
		var persistentVolumeClaim *corev1.PersistentVolumeClaim
		var storageClass *storagev1.StorageClass
		var statefulSet *appsv1.StatefulSet
		var selector labels.Selector
		BeforeEach(func() {
			Env.RequireEBSCSIDriver()

			numPods = 1
			subnets := Env.DiscoveredSubnets()
			shuffledAZs := lo.Keys(subnets)
			mutable.Shuffle(shuffledAZs)

			storageClass = test.StorageClass(test.StorageClassOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-storage-class",
				},
				Provisioner:       awssdk.String(Env.EBSCSIDriver.Provisioner),
				VolumeBindingMode: lo.ToPtr(storagev1.VolumeBindingWaitForFirstConsumer),
			})

			storageClass.AllowedTopologies = []corev1.TopologySelectorTerm{{
				MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{{
					Key:    Env.EBSCSIDriver.ZoneLabel,
					Values: []string{shuffledAZs[0]},
				}},
			}}

			persistentVolumeClaim = test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: &storageClass.Name,
			})
			statefulSet = test.StatefulSet(test.StatefulSetOptions{
				Replicas: int32(numPods), //nolint:gosec
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "my-app",
						}},
				},
			})
			// Ensure same volume is used across replica restarts.
			statefulSet.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{*persistentVolumeClaim}
			// Ensure volume mounts to pod, so that we test that we avoid the 6+ minute force detach delay.
			vm := corev1.VolumeMount{
				Name:      persistentVolumeClaim.Name,
				MountPath: "/usr/share",
			}
			statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{vm}
			selector = labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels)
		})

		It("should run on a new node without 6+ minute delays when disrupted", func() {
			// EBS volume detach + attach should usually take ~20s. Extra time is to prevent flakes due to EC2 APIs.
			forceDetachTimeout := 2 * time.Minute

			Env.ExpectCreated(NodeClass, NodePool, storageClass, statefulSet)
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			Env.EventuallyExpectHealthyPodCount(selector, numPods)

			Env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

			// Delete original nodeClaim to get the original node deleted
			Env.ExpectDeleted(nodeClaim)

			// Eventually the node will be tainted, which means its actively being disrupted
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
				_, ok := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
					return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
				})
				g.Expect(ok).To(BeTrue())
			}).Should(Succeed())

			Env.EventuallyExpectCreatedNodeCount(">=", 1)

			// After the deletion timestamp is set and all pods are drained the node should be gone.
			Env.EventuallyExpectNotFound(nodeClaim, node)

			// We expect the stateful workload to become healthy on new node before the 6-minute force detach timeout.
			// We start timer after pod binds to node because volume attachment happens during ContainerCreating
			Env.EventuallyExpectCreatedNodeClaimCount("==", 1)
			Env.EventuallyExpectCreatedNodeCount(">=", 1)
			Env.EventuallyExpectBoundPodCount(selector, numPods)
			Env.EventuallyExpectHealthyPodCountWithTimeout(forceDetachTimeout, selector, numPods)
		})
		It("should not block node deletion if stateful workload cannot be drained", func() {
			// Make pod un-drain-able by tolerating disruption taint.
			statefulSet.Spec.Template.Spec.Tolerations = []corev1.Toleration{{
				Key:      "karpenter.sh/disruption",
				Operator: corev1.TolerationOpEqual,
				Value:    "disrupting",
				Effect:   corev1.TaintEffectNoExecute,
			}}

			Env.ExpectCreated(NodeClass, NodePool, storageClass, statefulSet)
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			Env.EventuallyExpectHealthyPodCount(selector, numPods)

			// Delete original nodeClaim to get the original node deleted
			Env.ExpectDeleted(nodeClaim)

			// Eventually the node will be tainted, which means its actively being disrupted
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(node), node)).Should(Succeed())
				_, ok := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
					return t.MatchTaint(&karpv1.DisruptedNoScheduleTaint)
				})
				g.Expect(ok).To(BeTrue())
			}).Should(Succeed())

			// After the deletion timestamp is set and all pods are drained
			// the node should be gone regardless of orphaned volume attachment objects.
			Env.EventuallyExpectNotFound(nodeClaim, node)
		})
	})

	Describe("Ephemeral Storage", func() {
		DescribeTable("should run a pod with instance-store ephemeral storage that exceeds EBS root block device mappings", func(alias string) {
			if strings.HasPrefix(alias, "al2@") && Env.K8sMinorVersion() > 32 {
				Skip("AL2 is not supported on versions > 1.32")
			}
			NodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Alias: alias,
				},
			}
			pod := test.Pod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
					},
				},
			})
			Env.ExpectCreated(NodeClass, NodePool, pod)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)
		},
			Entry("AL2", "al2@latest"),
			Entry("AL2023", "al2023@latest"),
			Entry("Bottlerocket", "bottlerocket@latest"),
		)
	})
	return true
}

func ExpectSetEBSDriverLimit(limit int) {
	GinkgoHelper()
	Env.RequireEBSCSINodeDaemonSet(fmt.Sprintf("--volume-attach-limit=%d", limit))
	ds := &appsv1.DaemonSet{}
	Expect(Env.Client.Get(Env.Context, client.ObjectKey{Namespace: "kube-system", Name: Env.EBSCSIDriver.NodeDaemonSet}, ds)).To(Succeed())
	stored := ds.DeepCopy()

	containers := ds.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name != "ebs-plugin" {
			continue
		}
		containers[i].Args = append(containers[i].Args, fmt.Sprintf("--volume-attach-limit=%d", limit))
		break
	}
	Expect(Env.Client.Patch(Env.Context, ds, client.StrategicMergeFrom(stored))).To(Succeed())
}

func ExpectRemoveEBSDriverLimit() {
	GinkgoHelper()
	ds := &appsv1.DaemonSet{}
	Expect(Env.Client.Get(Env.Context, client.ObjectKey{Namespace: "kube-system", Name: Env.EBSCSIDriver.NodeDaemonSet}, ds)).To(Succeed())
	stored := ds.DeepCopy()

	containers := ds.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name != "ebs-plugin" {
			continue
		}
		containers[i].Args = lo.Reject(containers[i].Args, func(arg string, _ int) bool {
			return strings.Contains(arg, "--volume-attach-limit")
		})
		break
	}
	Expect(Env.Client.Patch(Env.Context, ds, client.StrategicMergeFrom(stored))).To(Succeed())
}
