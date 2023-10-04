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

package options_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/operator/options"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Options")
}

var _ = Describe("Options", func() {
	var fs *flag.FlagSet
	var opts *options.Options
	BeforeEach(func() {
		fs = flag.NewFlagSet("karpenter", flag.ContinueOnError)
		opts = &options.Options{}
		opts.AddFlags(fs)

		// Inject default settings
		var err error
		ctx, err = (&settings.Settings{}).Inject(ctx, nil)
		Expect(err).To(BeNil())
	})

	Context("Merging", func() {
		It("shouldn't overwrite cli flags / env vars", func() {
			err := opts.Parse(
				fs,
				"--assume-role-arn", "options-cluster-role",
				"--cluster-ca-bundle", "options-bundle",
				"--cluster-name", "options-cluster",
				"--cluster-endpoint", "https://options-cluster",
				"--isolated-vpc", "false",
				"--interruption-queue-name", "options-cluster",
			)
			Expect(err).ToNot(HaveOccurred())
			ctx = settings.ToContext(ctx, &settings.Settings{
				AssumeRoleARN:           "settings-cluster-role",
				AssumeRoleDuration:      time.Minute * 20,
				ClusterCABundle:         "settings-bundle",
				ClusterName:             "settings-cluster",
				ClusterEndpoint:         "https://settings-cluster",
				IsolatedVPC:             true,
				VMMemoryOverheadPercent: 0.1,
				InterruptionQueueName:   "settings-cluster",
				ReservedENIs:            10,
			})
			opts.MergeSettings(ctx)
			Expect(opts.AssumeRoleARN).To(Equal("options-cluster-role"))
			Expect(opts.AssumeRoleDuration).To(Equal(time.Minute * 20))
			Expect(opts.ClusterCABundle).To(Equal("options-bundle"))
			Expect(opts.ClusterName).To(Equal("options-cluster"))
			Expect(opts.ClusterEndpoint).To(Equal("https://options-cluster"))
			Expect(opts.IsolatedVPC).To(BeFalse())
			Expect(opts.VMMemoryOverheadPercent).To(Equal(0.1))
			Expect(opts.InterruptionQueueName).To(Equal("options-cluster"))
			Expect(opts.ReservedENIs).To(Equal(10))
		})

	})

	Context("Validation", func() {
		It("should fail when cluster name is not set", func() {
			err := opts.Parse(fs)
			// Overwrite ClusterName since it is commonly set by environment variables in dev environments
			opts.ClusterName = ""
			Expect(err).ToNot(HaveOccurred())
			Expect(func() {
				opts.MergeSettings(ctx)
				fmt.Printf("%#v", opts)
				}).To(Panic())
		})
		It("should fail when assume role duration is less than 15 minutes", func() {
			err := opts.Parse(fs, "--assume-role-duration", "1s")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when clusterEndpoint is invalid (not absolute)", func() {
			err := opts.Parse(fs, "--cluster-endpoint", "00000000000000000000000.gr7.us-west-2.eks.amazonaws.com")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when vmMemoryOverheadPercent is negative", func() {
			err := opts.Parse(fs, "--vm-memory-overhead-percent", "-0.01")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when reservedENIs is negative", func() {
			err := opts.Parse(fs, "--reserved-enis", "-1")
			Expect(err).To(HaveOccurred())
		})
	})
})
