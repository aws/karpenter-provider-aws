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

package integration

import (
	"fmt"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type registryImage struct {
	Registry string
	Image    string
	Command  []string
}

var sleep = []string{"sleep", "infinity"}

var nonECRImages = []registryImage{
	{"docker-hub", "docker.io/library/busybox:1.37@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec", sleep},
	{"docker-hub-large-layer", "docker.io/library/python:3.13-slim@sha256:e48f59cdba69a3cbd9ed2cf0f348d90d45a8d350d98134cf1070bc106ee9ed90", sleep},
	{"registry-k8s-io", "registry.k8s.io/pause:3.10@sha256:ee6521f290b2168b6e0935a181d4cff9be1ac3f505666ef0e3c98fae8199917a", nil},
	{"quay", "quay.io/prometheus/busybox:latest@sha256:7379672d53c28113469275f2eb48adffa19954f39f8ef20c1e2992278f658e6c", sleep},
	{"ghcr", "ghcr.io/containerd/busybox:1.36@sha256:7b3ccabffc97de872a30dfd234fd972a66d247c8cfc69b0550f276481852627c", sleep},
	{"mcr", "mcr.microsoft.com/oss/kubernetes/pause:3.9@sha256:a67d781a5a51290a56f6fb603b8ac9509abce8948d5a52ff3e02e8669a83180d", nil},
	{"gcr", "gcr.io/google-containers/pause:3.2@sha256:927d98197ec1141a368550822d18fa1c60bdae27b78b0c004f705f548c07814f", nil},
	{"mirror-gcr", "mirror.gcr.io/library/busybox:1.37@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec", sleep},
	{"redhat-large-layer", "registry.access.redhat.com/ubi9/ubi-minimal:9.6@sha256:34880b64c07f28f64d95737f82f891516de9a3b43583f39970f7bf8e4cfa48b7", sleep},
	{"quay-large-layer", "quay.io/prometheus/prometheus:v3.5.0@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996", sleep},
	{"ghcr-large-layer", "ghcr.io/astral-sh/uv:python3.13-bookworm-slim@sha256:531f855bda2c73cd6ef67d56b733b357cea384185b3022bd09f05e002cd144ca", sleep},
	{"mcr-large-layer", "mcr.microsoft.com/dotnet/runtime:8.0@sha256:21b0f8ba67bef2f121781d17a8e21956aeaa2d8da8e4b5db0da1ea8f91e2b6a1", sleep},
	{"nvcr-large-layer", "nvcr.io/nvidia/cuda:12.6.3-base-ubuntu24.04@sha256:c87e78933f4c16e3272123bf2f75537306596d0fbaa395a29696a22786e5ee0e", sleep},
}

func registerRegistries() bool {
	return Describe("Registries", func() {
		BeforeEach(func() {
			if Env.PrivateCluster {
				Skip("private clusters can only pull from ECR")
			}
		})
		It("should run pods with images from non-ECR registries", func() {
			pods := lo.Map(nonECRImages, func(r registryImage, _ int) *corev1.Pod {
				return test.Pod(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"registry": r.Registry}},
					Image:      r.Image,
					Command:    r.Command,
				})
			})
			Env.ExpectCreated(NodeClass, NodePool)
			Env.ExpectCreated(lo.Map(pods, func(p *corev1.Pod, _ int) client.Object { return p })...)
			Eventually(func(g Gomega) {
				for _, pod := range pods {
					g.Expect(Env.Client.Get(Env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
					g.Expect(pod.Status.Conditions).To(ContainElement(And(
						HaveField("Type", Equal(corev1.PodReady)),
						HaveField("Status", Equal(corev1.ConditionTrue)),
					)), "%s: %s", pod.Spec.Containers[0].Image, describeContainerState(pod))
				}
			}).Should(Succeed())
		})
	})
}

func describeContainerState(pod *corev1.Pod) string {
	if len(pod.Status.ContainerStatuses) == 0 {
		return fmt.Sprintf("phase %s, node %q", pod.Status.Phase, pod.Spec.NodeName)
	}
	s := pod.Status.ContainerStatuses[0].State
	switch {
	case s.Waiting != nil:
		return fmt.Sprintf("waiting %s: %s", s.Waiting.Reason, s.Waiting.Message)
	case s.Terminated != nil:
		return fmt.Sprintf("terminated %s (exit %d): %s", s.Terminated.Reason, s.Terminated.ExitCode, s.Terminated.Message)
	default:
		return "running, not ready"
	}
}
