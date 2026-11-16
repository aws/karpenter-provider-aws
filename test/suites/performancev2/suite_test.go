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

package performancev2_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/karpenter/test/suites/performancev2/env"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf/scenarios"

	"github.com/aws/karpenter-provider-aws/test/suites/performancev2"
)

var clients env.Clients

func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() { clients = env.New() })
	RunSpecs(t, "PerformanceV2 AWS")
}

var _ = perf.Suite(func() *perf.Provider { return performancev2.AWS(clients) }, scenarios.All...)
