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

package v1alpha1_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation")
}

var _ = Describe("Validation", func() {
	var ant *v1alpha1.AWSNodeTemplate

	BeforeEach(func() {
		ant = &v1alpha1.AWSNodeTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SubnetSelector:        map[string]string{"foo": "bar"},
					SecurityGroupSelector: map[string]string{"foo": "bar"},
				},
			},
		}
	})

	Context("UserData", func() {
		It("should succeed if user data is empty", func() {
			Expect(ant.Validate(ctx)).To(Succeed())
		})
		It("should fail if launch template is also specified", func() {
			ant.Spec.LaunchTemplateName = ptr.String("someLaunchTemplate")
			ant.Spec.UserData = ptr.String("someUserData")
			Expect(ant.Validate(ctx)).To(Not(Succeed()))
		})
	})
	Context("Tags", func() {
		It("should succeed when tags are empty", func() {
			ant.Spec.Tags = map[string]string{}
			Expect(ant.Validate(ctx)).To(Succeed())
		})
		It("should succeed if tags aren't in restricted tag keys", func() {
			ant.Spec.Tags = map[string]string{
				"karpenter.sh/custom-key": "value",
				"karpenter.sh/managed":    "true",
				"kubernetes.io/role/key":  "value",
			}
			Expect(ant.Validate(ctx)).To(Succeed())
		})
		It("should succeed by validating that regex is properly escaped", func() {
			ant.Spec.Tags = map[string]string{
				"karpenterzsh/provisioner-name": "value",
			}
			Expect(ant.Validate(ctx)).To(Succeed())
			ant.Spec.Tags = map[string]string{
				"kubernetesbio/cluster/test": "value",
			}
			Expect(ant.Validate(ctx)).To(Succeed())
			ant.Spec.Tags = map[string]string{
				"karpenterzsh/managed-by": "test",
			}
			Expect(ant.Validate(ctx)).To(Succeed())
		})
		It("should fail if tags contain a restricted domain key", func() {
			ant.Spec.Tags = map[string]string{
				"karpenter.sh/provisioner-name": "value",
			}
			Expect(ant.Validate(ctx)).To(Not(Succeed()))
			ant.Spec.Tags = map[string]string{
				"kubernetes.io/cluster/test": "value",
			}
			Expect(ant.Validate(ctx)).To(Not(Succeed()))
			ant.Spec.Tags = map[string]string{
				"karpenter.sh/managed-by": "test",
			}
			Expect(ant.Validate(ctx)).To(Not(Succeed()))
		})
	})
})
