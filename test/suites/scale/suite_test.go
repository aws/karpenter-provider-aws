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

package scale_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *aws.Environment

func TestScale(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
		env.ExpectPrefixDelegationEnabled()
		SetDefaultEventuallyTimeout(time.Hour)
	})
	AfterSuite(func() {
		env.ExpectPrefixDelegationDisabled()
		env.Stop()
	})
	RunSpecs(t, "Scale")
}

var _ = BeforeEach(func(ctx context.Context) {
	env.SetContext(ctx)
	env.BeforeEach()
})
var _ = AfterEach(func() {
	env.ResetContext()
	env.Cleanup()
	env.AfterEach()
})
