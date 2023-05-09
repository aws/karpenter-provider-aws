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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment

func TestScale(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
		Expect(env.PromClient).ToNot(BeNil(), "Prometheus service could not be discovered. "+
			"If testing scale testing locally, prometheus should be able to be accessed on localhost:9090 by port-forwarding "+
			"with \"kubectl port-forward -n <namespace> svc/<name> 9090\"")
		SetDefaultEventuallyTimeout(15 * time.Minute)
	})
	RunSpecs(t, "Scale")
}

var _ = BeforeEach(func() {
	// We restart during scale testing to clear out the summary quantiles so that we get separate metrics
	// for each individual test case
	env.EventuallyExpectKarpenterRestarted()
	env.BeforeEach()
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() {
	PrintSLOMetrics()
	env.AfterEach()
})

func PrintSLOMetrics() {
	// TODO @joinnis: Testing the PromQL querying
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_machines_created", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_nodes_created", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_pods_startup_time_seconds", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_pods_startup_time_seconds_count", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_consistency_errors", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_deprovisioning_replacement_node_initialized_seconds", nil).String())
	logging.FromContext(env).Infof(env.ExpectPrometheusQuery("karpenter_deprovisioning_evaluation_duration_seconds", nil).String())
}
