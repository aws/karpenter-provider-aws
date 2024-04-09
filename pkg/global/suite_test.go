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

package global

import (
	"context"
	"os"
	"testing"
	"time"

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

var _ = Describe("Config", func() {
	AfterEach(func() {
		Config = config{}
		os.Clearenv()
	})

	It("should correctly override default vars when CLI flags are set", func() {
		err := Initialize(
			"--assume-role-arn", "env-role",
			"--assume-role-duration", "20m",
			"--cluster-ca-bundle", "env-bundle",
			"--cluster-name", "env-cluster",
			"--cluster-endpoint", "https://env-cluster",
			"--isolated-vpc",
			"--vm-memory-overhead-percent", "0.1",
			"--interruption-queue", "env-cluster",
			"--reserved-enis", "10")
		Expect(err).ToNot(HaveOccurred())
		expectConfigEqual(Config, config{
			AssumeRoleARN:           "env-role",
			AssumeRoleDuration:      20 * time.Minute,
			ClusterCABundle:         "env-bundle",
			ClusterName:             "env-cluster",
			ClusterEndpoint:         "https://env-cluster",
			IsolatedVPC:             true,
			VMMemoryOverheadPercent: 0.1,
			InterruptionQueue:       "env-cluster",
			ReservedENIs:            10,
		})
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

		err := Initialize()
		Expect(err).ToNot(HaveOccurred())

		expectConfigEqual(Config, config{
			AssumeRoleARN:           "env-role",
			AssumeRoleDuration:      20 * time.Minute,
			ClusterCABundle:         "env-bundle",
			ClusterName:             "env-cluster",
			ClusterEndpoint:         "https://env-cluster",
			IsolatedVPC:             true,
			VMMemoryOverheadPercent: 0.1,
			InterruptionQueue:       "env-cluster",
			ReservedENIs:            10,
		})
	})

	Context("Validation", func() {
		It("should fail when cluster name is not set", func() {
			err := Initialize()
			Expect(err).To(HaveOccurred())
		})
		It("should fail when assume role duration is less than 15 minutes", func() {
			err := Initialize("--cluster-name", "test-cluster", "--assume-role-duration", "1s")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when clusterEndpoint is invalid (not absolute)", func() {
			err := Initialize("--cluster-name", "test-cluster", "--cluster-endpoint", "00000000000000000000000.gr7.us-west-2.eks.amazonaws.com")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when vmMemoryOverheadPercent is negative", func() {
			err := Initialize("--cluster-name", "test-cluster", "--vm-memory-overhead-percent", "-0.01")
			Expect(err).To(HaveOccurred())
		})
		It("should fail when reservedENIs is negative", func() {
			err := Initialize("--cluster-name", "test-cluster", "--reserved-enis", "-1")
			Expect(err).To(HaveOccurred())
		})
	})
})

func expectConfigEqual(configA, configB config) {
	GinkgoHelper()
	Expect(configA.AssumeRoleARN).To(Equal(configB.AssumeRoleARN))
	Expect(configA.AssumeRoleDuration).To(Equal(configB.AssumeRoleDuration))
	Expect(configA.ClusterCABundle).To(Equal(configB.ClusterCABundle))
	Expect(configA.ClusterName).To(Equal(configB.ClusterName))
	Expect(configA.ClusterEndpoint).To(Equal(configB.ClusterEndpoint))
	Expect(configA.IsolatedVPC).To(Equal(configB.IsolatedVPC))
	Expect(configA.VMMemoryOverheadPercent).To(Equal(configB.VMMemoryOverheadPercent))
	Expect(configA.InterruptionQueue).To(Equal(configB.InterruptionQueue))
	Expect(configA.ReservedENIs).To(Equal(configB.ReservedENIs))
}
