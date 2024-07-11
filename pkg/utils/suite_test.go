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

package utils_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	corev1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var np *corev1.NodePool
var nc *v1.EC2NodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils ")
}

var _ = BeforeEach(func() {
	nc = test.EC2NodeClass()
	np = coretest.NodePool()

})

var _ = Describe("GetKubelet", func() {
	It("use v1beta1 NodePool kubelet configuration when defined by the compatibility annotation", func() {
		nc.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods:     lo.ToPtr(int32(343)),
			PodsPerCore: lo.ToPtr(int32(243)),
			EvictionHard: map[string]string{
				"test-key-1": "test-value-2",
			},
			EvictionSoft: map[string]string{
				"test-key-1": "test-value-2",
			},
			EvictionSoftGracePeriod: map[string]metav1.Duration{
				"test-key-1": metav1.Duration{Duration: time.Minute},
			},
			EvictionMaxPodGracePeriod:   lo.ToPtr(int32(43412)),
			ClusterDNS:                  []string{"test-dns"},
			ImageGCHighThresholdPercent: lo.ToPtr(int32(2323)),
			ImageGCLowThresholdPercent:  lo.ToPtr(int32(23)),
			CPUCFSQuota:                 lo.ToPtr(true),
		}
		npkubelet := &v1.KubeletConfiguration{
			MaxPods: lo.ToPtr(int32(332213)),
		}
		kubeletbyte, err := json.Marshal(npkubelet)
		Expect(err).To(BeNil())
		np.Annotations = map[string]string{
			corev1.ProviderCompatabilityAnnotationKey: string(kubeletbyte),
		}
		actualKubelet, err := utils.GetKubelet(np.Annotations[corev1.ProviderCompatabilityAnnotationKey], nc)
		Expect(err).To(BeNil())
		Expect(npkubelet).To(BeEquivalentTo(actualKubelet))
	})
	It("should use v1 EC2NodeClass kubeletconfiguration of compatibility annotation is not found", func() {
		np.Annotations = map[string]string{
			corev1.ProviderCompatabilityAnnotationKey: "",
		}
		nc.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods:     lo.ToPtr(int32(343)),
			PodsPerCore: lo.ToPtr(int32(243)),
			EvictionHard: map[string]string{
				"test-key-1": "test-value-2",
			},
			EvictionSoft: map[string]string{
				"test-key-1": "test-value-2",
			},
			EvictionSoftGracePeriod: map[string]metav1.Duration{
				"test-key-1": metav1.Duration{Duration: time.Minute},
			},
			EvictionMaxPodGracePeriod:   lo.ToPtr(int32(43412)),
			ClusterDNS:                  []string{"test-dns"},
			ImageGCHighThresholdPercent: lo.ToPtr(int32(2323)),
			ImageGCLowThresholdPercent:  lo.ToPtr(int32(23)),
			CPUCFSQuota:                 lo.ToPtr(true),
		}
		kubelet, err := utils.GetKubelet(np.Annotations[corev1.ProviderCompatabilityAnnotationKey], nc)
		Expect(err).To(BeNil())
		Expect(nc.Spec.Kubelet).To(BeEquivalentTo(kubelet))
	})
})
