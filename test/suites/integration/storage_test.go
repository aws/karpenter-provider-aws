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

package integration_test

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("Persistent Volumes", func() {
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

			env.ExpectCreated(nodeClass, nodePool, pv, pvc, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
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
			env.ExpectCreated(nodeClass, nodePool, pv, pvc, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should run a pod with a pre-bound persistent volume while respecting topology constraints", func() {
			subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
			shuffledAZs := lo.Shuffle(lo.Keys(subnets))

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
			env.ExpectCreated(nodeClass, nodePool, pv, pvc, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
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

			env.ExpectCreated(nodeClass, nodePool, pv, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
	})

	Context("Dynamic", func() {
		var storageClass *storagev1.StorageClass
		BeforeEach(func() {
			// Ensure that the EBS driver is installed, or we can't run the test.
			var ds appsv1.DaemonSet
			if err := env.Client.Get(env.Context, client.ObjectKey{
				Namespace: "kube-system",
				Name:      "ebs-csi-node",
			}, &ds); err != nil {
				if errors.IsNotFound(err) {
					Skip(fmt.Sprintf("skipping dynamic PVC test due to missing EBS driver %s", err))
				} else {
					Fail(fmt.Sprintf("determining EBS driver status, %s", err))
				}
			}
			storageClass = test.StorageClass(test.StorageClassOptions{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-storage-class",
				},
				Provisioner:       aws.String("ebs.csi.aws.com"),
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

			env.ExpectCreated(nodeClass, nodePool, storageClass, pvc, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should run a pod with a dynamic persistent volume while respecting allowed topologies", func() {
			subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
			shuffledAZs := lo.Shuffle(lo.Keys(subnets))

			storageClass.AllowedTopologies = []v1.TopologySelectorTerm{{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{{
					Key:    "topology.ebs.csi.aws.com/zone",
					Values: []string{shuffledAZs[0]},
				}},
			}}

			pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: &storageClass.Name,
			})
			pod := test.Pod(test.PodOptions{
				PersistentVolumeClaims: []string{pvc.Name},
			})

			env.ExpectCreated(nodeClass, nodePool, storageClass, pvc, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should run a pod with a dynamic persistent volume while respecting volume limits", func() {
			ExpectSetEBSDriverLimit(1)
			DeferCleanup(func() {
				ExpectRemoveEBSDriverLimit()
			})

			count := 2
			pvcs := lo.Times(count, func(_ int) *v1.PersistentVolumeClaim {
				return test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
					StorageClassName: &storageClass.Name,
				})
			})
			pods := lo.Map(pvcs, func(pvc *v1.PersistentVolumeClaim, _ int) *v1.Pod {
				return test.Pod(test.PodOptions{
					PersistentVolumeClaims: []string{pvc.Name},
				})
			})

			// Pod creation is spread out here to address an upstream issue where pods can schedule to the same node
			// and exceed its volume limits before the csi driver reports limits. This is not something Karpenter
			// currently compensates for.
			env.ExpectCreated(nodeClass, nodePool, storageClass, pvcs[0], pods[0])
			env.EventuallyExpectHealthy(pods[0])
			env.ExpectCreatedNodeCount("==", 1)
			env.ExpectCreated(pvcs[1], pods[1])
			env.EventuallyExpectHealthy(pods[1])
			env.ExpectCreatedNodeCount("==", 2)
		})
		It("should run a pod with a generic ephemeral volume", func() {
			pod := test.Pod(test.PodOptions{
				EphemeralVolumeTemplates: []test.EphemeralVolumeTemplateOptions{{
					StorageClassName: &storageClass.Name,
				}},
			})

			env.ExpectCreated(nodeClass, nodePool, storageClass, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
	})
})

var _ = Describe("Ephemeral Storage", func() {
	It("should run a pod with instance-store ephemeral storage that exceeds EBS root block device mappings", func() {
		nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)

		pod := test.Pod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
			},
		})

		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectDeleted(pod)
	})
})

func ExpectSetEBSDriverLimit(limit int) {
	GinkgoHelper()
	ds := &appsv1.DaemonSet{}
	Expect(env.Client.Get(env.Context, client.ObjectKey{Namespace: "kube-system", Name: "ebs-csi-node"}, ds)).To(Succeed())
	stored := ds.DeepCopy()

	containers := ds.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name != "ebs-plugin" {
			continue
		}
		containers[i].Args = append(containers[i].Args, fmt.Sprintf("--volume-attach-limit=%d", limit))
		break
	}
	Expect(env.Client.Patch(env.Context, ds, client.MergeFrom(stored))).To(Succeed())
}

func ExpectRemoveEBSDriverLimit() {
	GinkgoHelper()
	ds := &appsv1.DaemonSet{}
	Expect(env.Client.Get(env.Context, client.ObjectKey{Namespace: "kube-system", Name: "ebs-csi-node"}, ds)).To(Succeed())
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
	Expect(env.Client.Patch(env.Context, ds, client.MergeFrom(stored))).To(Succeed())
}
