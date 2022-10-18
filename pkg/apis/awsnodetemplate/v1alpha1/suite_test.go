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

package v1alpha1

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
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation")
}

var _ = Describe("Validation", func() {
	var ant *AWSNodeTemplate

	BeforeEach(func() {
		ant = &AWSNodeTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec:       AWSNodeTemplateSpec{},
		}
	})

	Context("UserData", func() {
		It("should succeed if user data is empty", func() {
			ant.Spec.SubnetSelector = map[string]string{"foo": "bar"}
			ant.Spec.SecurityGroupSelector = map[string]string{"foo": "bar"}
			Expect(ant.Validate(ctx)).To(Succeed())
		})
		It("should fail if launch template is also specified", func() {
			ant.Spec.LaunchTemplateName = ptr.String("someLaunchTemplate")
			ant.Spec.UserData = ptr.String("someUserData")
			Expect(ant.Validate(ctx)).To(Not(Succeed()))
		})
	})
})
