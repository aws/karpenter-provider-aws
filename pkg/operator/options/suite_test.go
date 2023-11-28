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
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	. "knative.dev/pkg/logging/testing"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Options")
}

var _ = Describe("Options", func() {
	var envState map[string]string
	var environmentVariables = []string{
		"ASSUME_ROLE_ARN",
		"ASSUME_ROLE_DURATION",
		"CLUSTER_CA_BUNDLE",
		"CLUSTER_NAME",
		"CLUSTER_ENDPOINT",
		"ISOLATED_VPC",
		"VM_MEMORY_OVERHEAD_PERCENT",
		"INTERRUPTION_QUEUE",
		"RESERVED_ENIS",
	}

	var fs *coreoptions.FlagSet
	var opts *options.Options

	BeforeEach(func() {
		envState = map[string]string{}
		for _, ev := range environmentVariables {
			val, ok := os.LookupEnv(ev)
			if ok {
				envState[ev] = val
			}
			os.Unsetenv(ev)
		}
		fs = &coreoptions.FlagSet{
			FlagSet: flag.NewFlagSet("karpenter", flag.ContinueOnError),
		}
		opts = &options.Options{}
		opts.AddFlags(fs)

		// Inject default settings
		var err error
		ctx, err = (&settings.Settings{}).Inject(ctx, nil)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		for _, ev := range environmentVariables {
			os.Unsetenv(ev)
		}
		for ev, val := range envState {
			os.Setenv(ev, val)
		}
	})

	Context("Merging", func() {
		It("shouldn't overwrite options when all are set", func() {
			err := opts.Parse(
				fs,
				"--assume-role-arn", "options-cluster-role",
				"--assume-role-duration", "20m",
				"--cluster-ca-bundle", "options-bundle",
				"--cluster-name", "options-cluster",
				"--cluster-endpoint", "https://options-cluster",
				"--isolated-vpc",
				"--vm-memory-overhead-percent", "0.1",
				"--interruption-queue", "options-cluster",
				"--reserved-enis", "10",
			)
			Expect(err).ToNot(HaveOccurred())
			ctx = settings.ToContext(ctx, &settings.Settings{
				AssumeRoleARN:           "settings-cluster-role",
				AssumeRoleDuration:      time.Minute * 22,
				ClusterCABundle:         "settings-bundle",
				ClusterName:             "settings-cluster",
				ClusterEndpoint:         "https://settings-cluster",
				IsolatedVPC:             true,
				VMMemoryOverheadPercent: 0.05,
				InterruptionQueueName:   "settings-cluster",
				ReservedENIs:            8,
			})
			opts.MergeSettings(ctx)
			expectOptionsEqual(opts, test.Options(test.OptionsFields{
				AssumeRoleARN:           lo.ToPtr("options-cluster-role"),
				AssumeRoleDuration:      lo.ToPtr(20 * time.Minute),
				ClusterCABundle:         lo.ToPtr("options-bundle"),
				ClusterName:             lo.ToPtr("options-cluster"),
				ClusterEndpoint:         lo.ToPtr("https://options-cluster"),
				IsolatedVPC:             lo.ToPtr(true),
				VMMemoryOverheadPercent: lo.ToPtr[float64](0.1),
				InterruptionQueue:       lo.ToPtr("options-cluster"),
				ReservedENIs:            lo.ToPtr(10),
			}))

		})
		It("should overwrite options when none are set", func() {
			err := opts.Parse(fs)
			Expect(err).ToNot(HaveOccurred())
			ctx = settings.ToContext(ctx, &settings.Settings{
				AssumeRoleARN:           "settings-cluster-role",
				AssumeRoleDuration:      time.Minute * 22,
				ClusterCABundle:         "settings-bundle",
				ClusterName:             "settings-cluster",
				ClusterEndpoint:         "https://settings-cluster",
				IsolatedVPC:             true,
				VMMemoryOverheadPercent: 0.05,
				InterruptionQueueName:   "settings-cluster",
				ReservedENIs:            8,
			})
			opts.MergeSettings(ctx)
			expectOptionsEqual(opts, test.Options(test.OptionsFields{
				AssumeRoleARN:           lo.ToPtr("settings-cluster-role"),
				AssumeRoleDuration:      lo.ToPtr(22 * time.Minute),
				ClusterCABundle:         lo.ToPtr("settings-bundle"),
				ClusterName:             lo.ToPtr("settings-cluster"),
				ClusterEndpoint:         lo.ToPtr("https://settings-cluster"),
				IsolatedVPC:             lo.ToPtr(true),
				VMMemoryOverheadPercent: lo.ToPtr[float64](0.05),
				InterruptionQueue:       lo.ToPtr("settings-cluster"),
				ReservedENIs:            lo.ToPtr(8),
			}))

		})
		It("should correctly merge options and settings when mixed", func() {
			err := opts.Parse(
				fs,
				"--assume-role-arn", "options-cluster-role",
				"--cluster-ca-bundle", "options-bundle",
				"--cluster-name", "options-cluster",
				"--cluster-endpoint", "https://options-cluster",
				"--interruption-queue", "options-cluster",
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
			expectOptionsEqual(opts, test.Options(test.OptionsFields{
				AssumeRoleARN:           lo.ToPtr("options-cluster-role"),
				AssumeRoleDuration:      lo.ToPtr(20 * time.Minute),
				ClusterCABundle:         lo.ToPtr("options-bundle"),
				ClusterName:             lo.ToPtr("options-cluster"),
				ClusterEndpoint:         lo.ToPtr("https://options-cluster"),
				IsolatedVPC:             lo.ToPtr(true),
				VMMemoryOverheadPercent: lo.ToPtr[float64](0.1),
				InterruptionQueue:       lo.ToPtr("options-cluster"),
				ReservedENIs:            lo.ToPtr(10),
			}))
		})

		It("should correctly fallback to env vars when CLI flags aren't set", func() {
			os.Setenv("ASSUME_ROLE_ARN", "env-role")
			os.Setenv("ASSUME_ROLE_DURATION", "20m")
			os.Setenv("CLUSTER_CA_BUNDLE", "env-bundle")
			os.Setenv("CLUSTER_NAME", "env-cluster")
			os.Setenv("CLUSTER_ENDPOINT", "https://env-cluster")
			os.Setenv("ISOLATED_VPC", "true")
			os.Setenv("VM_MEMORY_OVERHEAD_PERCENT", "0.1")
			os.Setenv("INTERRUPTION_QUEUE", "env-cluster")
			os.Setenv("RESERVED_ENIS", "10")
			fs = &coreoptions.FlagSet{
				FlagSet: flag.NewFlagSet("karpenter", flag.ContinueOnError),
			}
			opts.AddFlags(fs)
			err := opts.Parse(fs)
			Expect(err).ToNot(HaveOccurred())
			expectOptionsEqual(opts, test.Options(test.OptionsFields{
				AssumeRoleARN:           lo.ToPtr("env-role"),
				AssumeRoleDuration:      lo.ToPtr(20 * time.Minute),
				ClusterCABundle:         lo.ToPtr("env-bundle"),
				ClusterName:             lo.ToPtr("env-cluster"),
				ClusterEndpoint:         lo.ToPtr("https://env-cluster"),
				IsolatedVPC:             lo.ToPtr(true),
				VMMemoryOverheadPercent: lo.ToPtr[float64](0.1),
				InterruptionQueue:       lo.ToPtr("env-cluster"),
				ReservedENIs:            lo.ToPtr(10),
			}))
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

func expectOptionsEqual(optsA *options.Options, optsB *options.Options) {
	GinkgoHelper()
	Expect(optsA.AssumeRoleARN).To(Equal(optsB.AssumeRoleARN))
	Expect(optsA.AssumeRoleDuration).To(Equal(optsB.AssumeRoleDuration))
	Expect(optsA.ClusterCABundle).To(Equal(optsB.ClusterCABundle))
	Expect(optsA.ClusterName).To(Equal(optsB.ClusterName))
	Expect(optsA.ClusterEndpoint).To(Equal(optsB.ClusterEndpoint))
	Expect(optsA.IsolatedVPC).To(Equal(optsB.IsolatedVPC))
	Expect(optsA.VMMemoryOverheadPercent).To(Equal(optsB.VMMemoryOverheadPercent))
	Expect(optsA.InterruptionQueue).To(Equal(optsB.InterruptionQueue))
	Expect(optsA.ReservedENIs).To(Equal(optsB.ReservedENIs))
}
