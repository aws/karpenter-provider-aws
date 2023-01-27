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

	. "github.com/aws/karpenter-core/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/apis/config/settings"
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
		s, _ := settings.NewSettingsFromConfigMap(cm)
		Expect(s.DefaultInstanceProfile).To(Equal(""))
		Expect(s.EnablePodENI).To(BeFalse())
		Expect(s.EnableENILimitedPodDensity).To(BeTrue())
		Expect(s.IsolatedVPC).To(BeFalse())
		Expect(s.NodeNameConvention).To(Equal(settings.IPName))
		Expect(s.VMMemoryOverheadPercent).To(Equal(0.075))
		Expect(len(s.Tags)).To(BeZero())
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
				"aws.nodeNameConvention":         "resource-name",
				"aws.vmMemoryOverheadPercent":    "0.1",
				"aws.tags.tag1":                  "value1",
				"aws.tags.tag2":                  "value2",
			},
		}
		s, _ := settings.NewSettingsFromConfigMap(cm)
		Expect(s.DefaultInstanceProfile).To(Equal("karpenter"))
		Expect(s.EnablePodENI).To(BeTrue())
		Expect(s.EnableENILimitedPodDensity).To(BeFalse())
		Expect(s.IsolatedVPC).To(BeTrue())
		Expect(s.NodeNameConvention).To(Equal(settings.ResourceName))
		Expect(s.VMMemoryOverheadPercent).To(Equal(0.1))
		Expect(len(s.Tags)).To(Equal(2))
		Expect(s.Tags).To(HaveKeyWithValue("tag1", "value1"))
		Expect(s.Tags).To(HaveKeyWithValue("tag2", "value2"))
	})
	It("should fail validation with panic when clusterName not included", func() {
		defer ExpectPanic()
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint": "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
			},
		}
		_, _ = settings.NewSettingsFromConfigMap(cm)
	})
	It("should fail validation with panic when clusterEndpoint not included", func() {
		defer ExpectPanic()
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterName": "my-name",
			},
		}
		_, _ = settings.NewSettingsFromConfigMap(cm)
	})
	It("should fail validation with panic when clusterEndpoint is invalid (not absolute)", func() {
		defer ExpectPanic()
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterName":     "my-name",
				"aws.clusterEndpoint": "00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
			},
		}
		_, _ = settings.NewSettingsFromConfigMap(cm)
	})
	It("should fail validation with panic when vmMemoryOverheadPercent is negative", func() {
		defer ExpectPanic()
		cm := &v1.ConfigMap{
			Data: map[string]string{
				"aws.clusterEndpoint":         "https://00000000000000000000000.gr7.us-west-2.eks.amazonaws.com",
				"aws.clusterName":             "my-cluster",
				"aws.vmMemoryOverheadPercent": "-0.01",
			},
		}
		_, _ = settings.NewSettingsFromConfigMap(cm)
	})
})
