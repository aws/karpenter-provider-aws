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
	"fmt"

	"github.com/awslabs/operatorpkg/serrors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AMI error types", func() {
	Describe("IsAMIsNotDiscoveredForAliasError", func() {
		It("returns false for nil", func() {
			Expect(IsAMIsNotDiscoveredForAliasError(nil)).To(BeFalse())
		})

		It("returns false for unrelated errors", func() {
			Expect(IsAMIsNotDiscoveredForAliasError(fmt.Errorf("boom"))).To(BeFalse())
		})

		It("returns true for the typed error", func() {
			err := &AMIsNotDiscoveredForAliasError{
				error: fmt.Errorf("failed to discover any AMIs for alias"),
			}
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns true when wrapped with serrors.Wrap (matches family-file emission)", func() {
			err := serrors.Wrap(&AMIsNotDiscoveredForAliasError{
				error: fmt.Errorf("failed to discover any AMIs for alias"),
			}, "alias", "al2@v1")
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})

		It("returns true when wrapped with fmt.Errorf %w (matches controller call)", func() {
			err := fmt.Errorf("getting amis, %w", &AMIsNotDiscoveredForAliasError{
				error: fmt.Errorf("failed to discover any AMIs for alias"),
			})
			Expect(IsAMIsNotDiscoveredForAliasError(err)).To(BeTrue())
		})
	})
})
