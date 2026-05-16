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

package amifamily

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/smithy-go"

	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

// mockSSMProvider is a minimal ssm.Provider implementation for unit tests.
// errFn allows per-call error injection.
type mockSSMProvider struct {
	errFn func(name string) (string, error)
}

func (m mockSSMProvider) Get(_ context.Context, p ssm.Parameter) (string, error) {
	return m.errFn(p.Name)
}

// notFoundErr returns an error matching pkg/errors.IsNotFound (a smithy.APIError
// whose code is in the project's notFoundErrorCodes set).
func notFoundErr() error {
	return &smithy.GenericAPIError{Code: "ParameterNotFound", Message: "parameter not found"}
}

// transientErr returns a non-NotFound error — not a smithy.APIError at all, so
// errors.IsNotFound returns false (the discriminator treats it as transient).
func transientErr() error {
	return fmt.Errorf("simulated transient SSM error: throttling")
}

var _ = Describe("AMI family error classification on SSM failures", func() {
	ctx := context.Background()

	Describe("AL2.DescribeImageQuery", func() {
		family := AL2{}

		It("returns AMIsNotDiscoveredForAliasError when every SSM error is NotFound", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", notFoundErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v20240101")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns a non-typed error when any SSM error is transient (controller will requeue without flipping condition)", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", transientErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v20240101")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("resolving ssm parameters for alias"))
		})
	})

	Describe("AL2023.DescribeImageQuery", func() {
		family := AL2023{}

		It("returns AMIsNotDiscoveredForAliasError when every SSM error is NotFound", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", notFoundErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v20240101")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns a non-typed error when any SSM error is transient", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", transientErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v20240101")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("resolving ssm parameters for alias"))
		})
	})

	Describe("Bottlerocket.DescribeImageQuery", func() {
		family := Bottlerocket{}

		It("returns AMIsNotDiscoveredForAliasError when every SSM error is NotFound", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", notFoundErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v1.46.0")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns a non-typed error when any SSM error is transient", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", transientErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "v1.46.0")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("resolving ssm parameters for alias"))
		})
	})

	Describe("Windows.DescribeImageQuery", func() {
		family := Windows{Version: "2022", Build: "10.0.20348"}

		It("returns AMIsNotDiscoveredForAliasError when SSM error is NotFound", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", notFoundErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "latest")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns a non-typed error when SSM error is transient", func() {
			provider := mockSSMProvider{errFn: func(_ string) (string, error) {
				return "", transientErr()
			}}
			_, err := family.DescribeImageQuery(ctx, provider, "1.30", "latest")
			Expect(err).To(HaveOccurred())
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeFalse())
			Expect(err.Error()).To(ContainSubstring("resolving ssm parameter for alias"))
		})
	})
})
