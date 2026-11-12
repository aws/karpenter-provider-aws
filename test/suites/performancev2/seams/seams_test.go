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

// Package seams_test runs the AWS plugin through core's perf.Suite without a cluster: a fake controller-runtime
// client holds the controller pod, and a fake clientset's pod proxy serves a /metrics page whose
// aws_sdk_go_request_total counters grow on every scrape. It checks that the plugin's metric, Capture, TimeoutScale,
// NodePool and levels work end to end, so a core bump that breaks a seam fails here instead of in a nightly run.
package seams_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/test/suites/performancev2/env"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf/scenarios"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/suites/performancev2"
)

func TestSeams(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PerformanceV2 AWS seams")
}

type raw []byte

func (r raw) DoRaw(context.Context) ([]byte, error) { return r, nil }
func (r raw) Stream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(r))), nil
}

// fakeController serves /metrics whose EC2 and SSM request counters grow by ec2PerScrape and 1 on every scrape.
type fakeController struct {
	mu      sync.Mutex
	scrapes int
}

const ec2PerScrape = 7

func (f *fakeController) metrics() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scrapes++
	n := f.scrapes
	return fmt.Appendf(nil, `# TYPE leader_election_master_status gauge
leader_election_master_status{name="karpenter-leader-election"} 1
# TYPE go_gc_heap_live_bytes gauge
go_gc_heap_live_bytes 1.048576e+08
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 2.097152e+08
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total %d
# TYPE process_start_time_seconds gauge
process_start_time_seconds 1.7e+09
# TYPE karpenter_build_info gauge
karpenter_build_info{commit="abc123",version="v1"} 1
# TYPE aws_sdk_go_request_total counter
aws_sdk_go_request_total{action="DescribeInstanceTypes",code="200",service="EC2"} %d
aws_sdk_go_request_total{action="GetParameter",code="200",service="SSM"} %d
aws_sdk_go_request_total{action="ReceiveMessage",code="200",service="SQS"} %d
# TYPE go_goroutines gauge
go_goroutines 40
`, n, ec2PerScrape*n, n, 100*n)
}

const ns = "kube-system"

// provider is the real AWS plugin with its clients swapped for fakes and its levels replaced by the test's.
func provider(levels perf.Levels, mutate ...func(*perf.Provider)) *perf.Provider {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "karpenter-0", Namespace: ns, Labels: map[string]string{"app.kubernetes.io/name": "karpenter"}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pod).Build()
	ctrl := &fakeController{}
	kc := k8sfake.NewClientset()
	kc.PrependProxyReactor("pods", func(a k8stesting.Action) (bool, restclient.ResponseWrapper, error) {
		if a.(k8stesting.ProxyGetAction).GetPath() == "/metrics" {
			return true, raw(ctrl.metrics()), nil
		}
		return true, raw(nil), nil
	})
	p := performancev2.AWS(env.Clients{Client: c, KubeClient: kc})
	p.Levels = levels
	for _, m := range mutate {
		m(p)
	}
	return p
}

// fakeScenario runs one zero-pod phase that lasts a few sample periods, so the source scrapes several times.
func fakeScenario(name string, check func(*perf.Result)) perf.Scenario {
	return perf.Scenario{
		Name: name, Vectors: map[perf.Vector]int{perf.VectorPods: 10},
		Phases: []perf.PhaseSpec{{Name: "scaleOut"}},
		Run: func(r *perf.Run) {
			ph := r.StartPhase("scaleOut")
			time.Sleep(5 * time.Second)
			check(ph.AwaitScaleOut(0, time.Minute))
		},
	}
}

var outDir string

var _ = BeforeSuite(func() {
	outDir = GinkgoT().TempDir()
	GinkgoT().Setenv("OUTPUT_DIR", outDir)
	GinkgoT().Setenv("CLUSTER_NAME", "seams-cluster")
	GinkgoT().Setenv("KARPENTER_NAMESPACE", ns)
})

// aws_api_calls, kept by Capture "aws_sdk_go_", reaches the Result's counters, the gate results and the summary.
var _ = perf.Suite(func() *perf.Provider {
	return provider(perf.Levels{"metric/scaleOut": {performancev2.AWSAPICalls.Experimental(1), perf.HeapMB.Experimental(1000)}})
}, fakeScenario("metric", func(rep *perf.Result) {
	Expect(rep).ToNot(BeNil())
	Expect(rep.Counters).To(HaveKey(`aws_sdk_go_request_total{action=DescribeInstanceTypes,code=200,service=EC2}`))
	ec2 := rep.Counter("aws_sdk_go_request_total", map[string]string{"service": "EC2"})
	ssm := rep.Counter("aws_sdk_go_request_total", map[string]string{"service": "SSM"})
	Expect(ec2).To(BeNumerically(">=", 2*ec2PerScrape), "a delta over at least 3 scrapes")
	Expect(ssm * ec2PerScrape).To(Equal(ec2))
	Expect(rep.Counter("aws_sdk_go_request_total", map[string]string{"service": "SQS"})).To(BeNumerically(">", 0))
	calls := float64(ec2 + ssm)
	Expect(performancev2.AWSAPICalls.Value(rep)).To(Equal(calls), "aws_api_calls sums every service but the pollers")
	Expect(rep.Gates).To(ContainElement(SatisfyAll(
		HaveField("Metric", "aws_api_calls"), HaveField("Value", calls), HaveField("Passed", false), HaveField("Tier", perf.TierExperimental))))
	Expect(rep.Gates).To(ContainElement(SatisfyAll(HaveField("Metric", "heap_mb"), HaveField("Value", 100.0), HaveField("Passed", true))))
	Expect(rep.Commit).To(Equal("abc123"))
	b, err := os.ReadFile(filepath.Join(outDir, "perf_summary.md"))
	Expect(err).ToNot(HaveOccurred())
	Expect(string(b)).To(ContainSubstring("| aws_api_calls |"))
	Expect(string(b)).To(ContainSubstring("CLUSTER_NAME='seams-cluster'"), "ReproEnv reaches the repro line")
}))

// The plugin's TimeoutScale stretches the scenarios' waits.
var _ = perf.Suite(func() *perf.Provider {
	return provider(perf.Levels{"timeouts/scaleOut": {}})
}, perf.Scenario{Name: "timeouts", Vectors: map[perf.Vector]int{perf.VectorPods: 1}, Phases: []perf.PhaseSpec{{Name: "scaleOut"}},
	Run: func(r *perf.Run) {
		Expect(r.Provider.TimeoutScale).To(BeNumerically(">", 1))
		r.Create(r.NodeClass, r.NodePool) // the fake API server never marks the NodePool Ready
		start := time.Now()
		Expect(InterceptGomegaFailures(func() { r.AwaitNodePoolsReady(2 * time.Second) })).ToNot(BeEmpty())
		Expect(time.Since(start)).To(BeNumerically(">=", time.Duration(float64(2*time.Second)*r.Provider.TimeoutScale)))
	}})

var _ = Describe("AWS plugin", func() {
	It("builds an EC2NodeClass and a NodePool that references it", func() {
		GinkgoT().Setenv("CLUSTER_NAME", "c1")
		p := performancev2.AWS(env.Clients{})
		Expect(p.Controller.Namespace).To(Equal(ns))
		nc := p.NodeClass(&scenarios.Topology)
		Expect(nc).To(BeAssignableToTypeOf(&v1.EC2NodeClass{}))
		Expect(nc.(*v1.EC2NodeClass).Spec.Role).To(Equal("KarpenterNodeRole-c1"))
		np := p.NodePool(&scenarios.Topology, nc)
		Expect(np.Spec.Template.Spec.NodeClassRef.Kind).To(Equal("EC2NodeClass"))
		Expect(np.Spec.Template.Spec.NodeClassRef.Name).To(Equal(nc.GetName()))
		Expect(np.Spec.Template.Spec.Requirements).To(ContainElement(SatisfyAll(
			HaveField("Key", v1.LabelInstanceCategory), HaveField("Values", ConsistOf("c", "m", "r")))))
		Expect(np.Spec.Template.Spec.ExpireAfter).To(Equal(karpv1.MustParseNillableDuration("Never")))
	})
	It("enables every core scenario and passes validation and the evidence check, so no Stable gate lacks a planted regression", func() {
		p := performancev2.AWS(env.Clients{})
		for _, s := range scenarios.All {
			for _, ph := range s.Phases {
				Expect(p.Levels).To(HaveKey(s.Name+"/"+ph.Name), "levels for %s/%s", s.Name, ph.Name)
			}
		}
		Expect(perf.Validate(p, scenarios.All)).To(Succeed())
		Expect(perf.CheckEvidence(p)).To(Succeed())
	})
	It("captures the AWS SDK metrics its own metric reads", func() {
		Expect(performancev2.AWS(env.Clients{}).Capture).To(ContainElement("aws_sdk_go_"))
	})
})
