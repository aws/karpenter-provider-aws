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

package seams_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	timestreamtypes "github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/test/suites/performancev2/env"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf"

	"github.com/aws/karpenter-provider-aws/test/suites/performancev2"
)

// fakeTimestream records every WriteRecords call and fails the calls listed in errs, by call index.
type fakeTimestream struct {
	mu    sync.Mutex
	calls []*timestreamwrite.WriteRecordsInput
	errs  map[int]error
}

func (f *fakeTimestream) WriteRecords(_ context.Context, in *timestreamwrite.WriteRecordsInput, _ ...func(*timestreamwrite.Options)) (*timestreamwrite.WriteRecordsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	if err := f.errs[len(f.calls)-1]; err != nil {
		return nil, err
	}
	return &timestreamwrite.WriteRecordsOutput{}, nil
}

func (f *fakeTimestream) records() []timestreamtypes.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return lo.FlatMap(f.calls, func(in *timestreamwrite.WriteRecordsInput, _ int) []timestreamtypes.Record { return in.Records })
}

func record(records []timestreamtypes.Record, metric string) (timestreamtypes.Record, bool) {
	return lo.Find(records, func(r timestreamtypes.Record) bool { return aws.ToString(r.MeasureName) == metric })
}

func dimensions(r timestreamtypes.Record) map[string]string {
	return lo.SliceToMap(r.Dimensions, func(d timestreamtypes.Dimension) (string, string) { return aws.ToString(d.Name), aws.ToString(d.Value) })
}

type measure struct{ Value, Type string }

func measures(r timestreamtypes.Record) map[string]measure {
	return lo.SliceToMap(r.MeasureValues, func(m timestreamtypes.MeasureValue) (string, measure) {
		return aws.ToString(m.Name), measure{aws.ToString(m.Value), string(m.Type)}
	})
}

var phaseEnd = time.Date(2026, 11, 12, 7, 30, 0, 0, time.UTC)

func result() *perf.Result {
	return &perf.Result{
		SchemaVersion: 2, Provider: "aws-kwok", ID: "nodepools/scheduling", Scenario: "nodepools", Phase: "scheduling",
		Commit: "abc123", Sizes: map[perf.Vector]int{perf.VectorPods: 100, perf.VectorNodePools: 1000}, Time: phaseEnd,
		Total:     90 * time.Second,
		Resources: perf.ResourceStats{P95HeapMB: 3000.5, AvgCPUCores: 2.25},
		Gates:     []perf.GateResult{{Metric: "heap_mb", Value: 3000.5, Bound: 3281.8, Upper: true, Tier: perf.TierExperimental, Passed: true}},
	}
}

var run = performancev2.RunInfo{RunID: "123", RunAttempt: "2", RunURL: "https://github.com/aws/karpenter-provider-aws/actions/runs/123",
	CoreCommit: "d9c8e7cf2445", Trigger: "schedule", Cluster: "perf-nodepools-123-2", Region: "us-west-2",
	K8sVersion: "1.36", ControlPlaneTier: "tier-xl", ControllerShape: "cpu=8/8,mem=24Gi/24Gi"}

var _ = Describe("Timestream sink", func() {
	metrics := append(append([]perf.Metric(nil), perf.Metrics...), performancev2.AWSAPICalls)

	It("writes every metric, gated or not, in batches of at most 100", func() {
		many := append([]perf.Metric(nil), metrics...)
		for i := range 240 {
			many = append(many, perf.Metric{Name: fmt.Sprintf("extra_%03d", i), Value: func(*perf.Result) float64 { return float64(i) }})
		}
		ts := &fakeTimestream{}
		Expect(performancev2.TimestreamSink(ts, "karpenterTesting", "perfResults", run, many)(context.Background(), result())).To(Succeed())
		Expect(ts.calls).To(HaveLen(3))
		Expect(ts.calls[0].Records).To(HaveLen(100))
		Expect(ts.calls[1].Records).To(HaveLen(100))
		Expect(ts.calls[2].Records).To(HaveLen(len(many) - 200))
		for _, c := range ts.calls {
			Expect(aws.ToString(c.DatabaseName)).To(Equal("karpenterTesting"))
			Expect(aws.ToString(c.TableName)).To(Equal("perfResults"))
		}
		names := lo.Map(ts.records(), func(r timestreamtypes.Record, _ int) string { return aws.ToString(r.MeasureName) })
		Expect(names).To(HaveLen(len(many)))
		Expect(names).To(ContainElements("heap_mb", "cpu_util", "pod_ready_seconds_p95", "aws_api_calls", "extra_239"))
	})

	It("writes the run's dimensions and a gated metric's bound and result", func() {
		ts := &fakeTimestream{}
		Expect(performancev2.TimestreamSink(ts, "db", "t", run, metrics)(context.Background(), result())).To(Succeed())
		r, ok := record(ts.records(), "heap_mb")
		Expect(ok).To(BeTrue())
		Expect(r.MeasureValueType).To(Equal(timestreamtypes.MeasureValueTypeMulti))
		Expect(aws.ToString(r.Time)).To(Equal(fmt.Sprint(phaseEnd.UnixMilli())))
		Expect(r.TimeUnit).To(Equal(timestreamtypes.TimeUnitMilliseconds))
		Expect(dimensions(r)).To(Equal(map[string]string{
			"provider": "aws-kwok", "scenario": "nodepools", "phase": "scheduling", "id": "nodepools/scheduling",
			"schema_version": "2", "commit": "abc123", "aws_commit": "abc123", "core_commit": "d9c8e7cf2445", "sizes": "nodepools=1000,pods=100",
			"run_id": "123", "run_attempt": "2", "run_url": "https://github.com/aws/karpenter-provider-aws/actions/runs/123",
			"trigger": "schedule", "cluster": "perf-nodepools-123-2", "region": "us-west-2",
			"k8s_version": "1.36", "cp_tier": "tier-xl", "controller_shape": "cpu=8/8,mem=24Gi/24Gi", "tier": "experimental",
		}))
		m := measures(r)
		Expect(m).To(HaveKeyWithValue("headroom_pct", HaveField("Type", "DOUBLE")))
		headroom := lo.Must(strconv.ParseFloat(m["headroom_pct"].Value, 64))
		Expect(headroom).To(BeNumerically("~", (3281.8-3000.5)/3281.8*100, 1e-9))
		delete(m, "headroom_pct")
		Expect(m).To(Equal(map[string]measure{
			"value": {"3000.5", "DOUBLE"}, "bound": {"3281.8", "DOUBLE"}, "gated": {"true", "BOOLEAN"}, "upper": {"true", "BOOLEAN"},
			"passed": {"true", "BOOLEAN"}, "overridden": {"false", "BOOLEAN"}, "status": {"WARN", "VARCHAR"},
		}), "8.6%% headroom is under 10%%, so WARN")
	})

	It("signs headroom_pct by the gate's direction and sets status PASS, WARN, BREACH or FAIL", func() {
		rep := result()
		rep.ReservedCPUUtil, rep.Nodes = 0.5, 120
		rep.Gates = []perf.GateResult{
			{Metric: "cpu_util", Value: 0.5, Bound: 0.4, Upper: false, Tier: perf.TierStable, Passed: true},
			{Metric: "nodes", Value: 120, Bound: 100, Upper: true, Tier: perf.TierStable, Passed: false},
			{Metric: "cpu_cores", Value: 2.25, Bound: 4.5, Upper: true, Tier: perf.TierExperimental, Passed: true},
			{Metric: "heap_mb", Value: 3000.5, Bound: 2000, Upper: true, Tier: perf.TierExperimental, Passed: false},
		}
		ts := &fakeTimestream{}
		Expect(performancev2.TimestreamSink(ts, "db", "t", run, metrics)(context.Background(), rep)).To(Succeed())
		for metric, want := range map[string]struct {
			headroom      float64
			upper, status string
		}{"cpu_util": {25, "false", "PASS"}, "nodes": {-20, "true", "FAIL"}, "cpu_cores": {50, "true", "PASS"}, "heap_mb": {-50.025, "true", "BREACH"}} {
			r, ok := record(ts.records(), metric)
			Expect(ok).To(BeTrue())
			m := measures(r)
			Expect(lo.Must(strconv.ParseFloat(m["headroom_pct"].Value, 64))).To(BeNumerically("~", want.headroom, 1e-9), metric)
			Expect(m["upper"]).To(Equal(measure{want.upper, "BOOLEAN"}), metric)
			Expect(m["status"]).To(Equal(measure{want.status, "VARCHAR"}), metric)
		}
	})

	It("is off without PERF_TIMESTREAM_TABLE and on with it", func() {
		GinkgoT().Setenv("PERF_TIMESTREAM_TABLE", "")
		Expect(performancev2.AWS(env.Clients{}).Sinks).To(BeEmpty())
		GinkgoT().Setenv("PERF_TIMESTREAM_TABLE", "perfResults")
		GinkgoT().Setenv("PERF_TIMESTREAM_REGION", "us-east-2")
		Expect(performancev2.AWS(env.Clients{}).Sinks).To(HaveLen(1)) // building the client makes no AWS call
	})
})
