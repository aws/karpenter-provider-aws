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

package settings_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis/settings"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Settings")
}

var _ = Describe("Validation", func() {
	It("should succeed to set defaults", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":     "my-cluster",
			},
		}
		ctx, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).ToNot(HaveOccurred())
		s := settings.FromContext(ctx)
		Expect(s.DefaultInstanceProfile).To(Equal(""))
		Expect(s.EnablePodENI).To(BeFalse())
		Expect(s.EnableENILimitedPodDensity).To(BeTrue())
		Expect(s.IsolatedVPC).To(BeFalse())
		Expect(s.VMMemoryOverheadPercent).To(Equal(0.075))
		Expect(len(s.Tags)).To(BeZero())
		Expect(s.ReservedENIs).To(Equal(0))
	})
	It("should succeed to set custom values", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint":            "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":                "my-cluster",
				"aws.defaultInstanceProfile":     "karpenter",
				"aws.enablePodENI":               "true",
				"aws.enableENILimitedPodDensity": "false",
				"aws.isolatedVPC":                "true",
				"aws.vmMemoryOverheadPercent":    "0.1",
				"aws.tags":                       `{"tag1": "value1", "tag2": "value2", "example.com/tag": "my-value"}`,
				"aws.reservedENIs":               "1",
			},
		}
		ctx, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).ToNot(HaveOccurred())
		s := settings.FromContext(ctx)
		Expect(s.DefaultInstanceProfile).To(Equal("karpenter"))
		Expect(s.EnablePodENI).To(BeTrue())
		Expect(s.EnableENILimitedPodDensity).To(BeFalse())
		Expect(s.IsolatedVPC).To(BeTrue())
		Expect(s.VMMemoryOverheadPercent).To(Equal(0.1))
		Expect(len(s.Tags)).To(Equal(3))
		Expect(s.Tags).To(HaveKeyWithValue("tag1", "value1"))
		Expect(s.Tags).To(HaveKeyWithValue("tag2", "value2"))
		Expect(s.Tags).To(HaveKeyWithValue("example.com/tag", "my-value"))
		Expect(s.ReservedENIs).To(Equal(1))
	})
	It("should succeed when setting values that no longer exist (backwards compatibility)", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint":            "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":                "my-cluster",
				"aws.defaultInstanceProfile":     "karpenter",
				"aws.enablePodENI":               "true",
				"aws.enableENILimitedPodDensity": "false",
				"aws.isolatedVPC":                "true",
				"aws.vmMemoryOverheadPercent":    "0.1",
				"aws.tags":                       `{"tag1": "value1", "tag2": "value2", "example.com/tag": "my-value"}`,
				"aws.reservedENIs":               "1",
				"aws.nodeNameConvention":         "resource-name",
			},
		}
		ctx, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).ToNot(HaveOccurred())
		s := settings.FromContext(ctx)
		Expect(s.DefaultInstanceProfile).To(Equal("karpenter"))
		Expect(s.EnablePodENI).To(BeTrue())
		Expect(s.EnableENILimitedPodDensity).To(BeFalse())
		Expect(s.IsolatedVPC).To(BeTrue())
		Expect(s.VMMemoryOverheadPercent).To(Equal(0.1))
		Expect(len(s.Tags)).To(Equal(3))
		Expect(s.Tags).To(HaveKeyWithValue("tag1", "value1"))
		Expect(s.Tags).To(HaveKeyWithValue("tag2", "value2"))
		Expect(s.Tags).To(HaveKeyWithValue("example.com/tag", "my-value"))
		Expect(s.ReservedENIs).To(Equal(1))
	})
	It("should succeed validation when tags contain parts of restricted domains", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":     "my-cluster",
				"aws.tags":            `{"karpenter.sh/custom-key": "value1", "karpenter.sh/managed": "true", "kubernetes.io/role/key": "value2", "kubernetes.io/cluster/other-tag/hello": "value3"}`,
			},
		}
		ctx, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).ToNot(HaveOccurred())
		s := settings.FromContext(ctx)
		Expect(s.Tags).To(HaveKeyWithValue("karpenter.sh/custom-key", "value1"))
		Expect(s.Tags).To(HaveKeyWithValue("karpenter.sh/managed", "true"))
		Expect(s.Tags).To(HaveKeyWithValue("kubernetes.io/role/key", "value2"))
		Expect(s.Tags).To(HaveKeyWithValue("kubernetes.io/cluster/other-tag/hello", "value3"))
	})
	It("should fail validation with panic when clusterName not included", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
			},
		}
		_, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())
	})
	It("should fail validation when clusterEndpoint is invalid (not absolute)", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterName":     "my-name",
				"aws.clusterEndpoint": "00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
			},
		}
		_, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())
	})
	It("should fail validation with panic when vmMemoryOverheadPercent is negative", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint":         "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":             "my-cluster",
				"aws.vmMemoryOverheadPercent": "-0.01",
			},
		}
		_, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())
	})
	It("should fail validation when tags have keys that are in the restricted set of keys", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":     "my-cluster",
				"aws.tags":            `{"karpenter.sh/provisioner-name": "value1"}`,
			},
		}
		_, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())

		cm = &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":     "my-cluster",
				"aws.tags":            `{"value1", "karpenter.sh/managed-by": "value"}`,
			},
		}
		_, err = (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())

		cm = &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":     "my-cluster",
				"aws.tags":            `{"kubernetes.io/cluster/my-cluster": "value2"}`,
			},
		}
		_, err = (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())
	})
	It("should fail validation with reservedENIs is negative", func() {
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.reservedENIs": "-1",
			},
		}
		_, err := (&settings.Settings{}).Inject(ctx, cm)
		Expect(err).To(HaveOccurred())
	})
})
