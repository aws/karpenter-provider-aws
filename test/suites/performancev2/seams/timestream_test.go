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
	"errors"
	"fmt"
	"math"
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

var run = performancev2.RunInfo{RunID: "123", RunAttempt: "2", Trigger: "schedule", Cluster: "perf-nodepools-123-2", Region: "us-west-2",
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
			"schema_version": "2", "commit": "abc123", "sizes": "nodepools=1000,pods=100",
			"run_id": "123", "run_attempt": "2", "trigger": "schedule", "cluster": "perf-nodepools-123-2", "region": "us-west-2",
			"k8s_version": "1.36", "cp_tier": "tier-xl", "controller_shape": "cpu=8/8,mem=24Gi/24Gi",
			"tier": "experimental", "upper": "true",
		}))
		Expect(measures(r)).To(Equal(map[string]measure{
			"value": {"3000.5", "DOUBLE"}, "bound": {"3281.8", "DOUBLE"}, "gated": {"true", "BOOLEAN"},
			"passed": {"true", "BOOLEAN"}, "overridden": {"false", "BOOLEAN"},
		}))
	})

	It("writes an ungated metric as measured, with no bound or result, and leaves out empty dimensions", func() {
		ts := &fakeTimestream{}
		Expect(performancev2.TimestreamSink(ts, "db", "t", performancev2.RunInfo{}, metrics)(context.Background(), result())).To(Succeed())
		r, ok := record(ts.records(), "cpu_cores")
		Expect(ok).To(BeTrue())
		Expect(measures(r)).To(Equal(map[string]measure{"value": {"2.25", "DOUBLE"}, "gated": {"false", "BOOLEAN"}}))
		Expect(dimensions(r)).To(Equal(map[string]string{
			"provider": "aws-kwok", "scenario": "nodepools", "phase": "scheduling", "id": "nodepools/scheduling",
			"schema_version": "2", "commit": "abc123", "sizes": "nodepools=1000,pods=100", "tier": "measured", "upper": "true",
		}))
		r, ok = record(ts.records(), "cpu_util")
		Expect(ok).To(BeTrue())
		Expect(dimensions(r)).To(HaveKeyWithValue("upper", "false"))
	})

	It("skips NaN and Inf values", func() {
		odd := append([]perf.Metric(nil), metrics...)
		odd = append(odd,
			perf.Metric{Name: "nan", Value: func(*perf.Result) float64 { return math.NaN() }},
			perf.Metric{Name: "inf", Value: func(*perf.Result) float64 { return math.Inf(-1) }})
		ts := &fakeTimestream{}
		Expect(performancev2.TimestreamSink(ts, "db", "t", run, odd)(context.Background(), result())).To(Succeed())
		_, ok := record(ts.records(), "nan")
		Expect(ok).To(BeFalse())
		_, ok = record(ts.records(), "inf")
		Expect(ok).To(BeFalse())
		Expect(ts.records()).To(HaveLen(len(metrics)))
	})

	It("returns a failed or rejected write as an error, still writes the other batches, and turns a panic into an error", func() {
		many := append([]perf.Metric(nil), metrics...)
		for i := range 100 {
			many = append(many, perf.Metric{Name: fmt.Sprintf("extra_%03d", i), Value: func(*perf.Result) float64 { return 1 }})
		}
		ts := &fakeTimestream{errs: map[int]error{0: &timestreamtypes.RejectedRecordsException{
			Message:         aws.String("One or more records have been rejected."),
			RejectedRecords: []timestreamtypes.RejectedRecord{{RecordIndex: 0, Reason: aws.String("dimension value is too long")}},
		}}}
		var err error
		Expect(func() { err = performancev2.TimestreamSink(ts, "db", "t", run, many)(context.Background(), result()) }).ToNot(Panic())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("total_time_seconds: dimension value is too long"))
		Expect(ts.calls).To(HaveLen(2), "the second batch is written after the first is rejected")

		ts = &fakeTimestream{errs: map[int]error{0: errors.New("throttled")}}
		Expect(performancev2.TimestreamSink(ts, "db", "t", run, metrics)(context.Background(), result())).To(MatchError(ContainSubstring("throttled")))

		boom := []perf.Metric{{Name: "boom", Value: func(*perf.Result) float64 { panic("nil counters") }}}
		Expect(func() { err = performancev2.TimestreamSink(ts, "db", "t", run, boom)(context.Background(), result()) }).ToNot(Panic())
		Expect(err).To(MatchError(ContainSubstring("nil counters")))
	})

	It("is off without PERF_TIMESTREAM_TABLE and on with it", func() {
		GinkgoT().Setenv("PERF_TIMESTREAM_TABLE", "")
		Expect(performancev2.AWS(env.Clients{}).Sinks).To(BeEmpty())
		GinkgoT().Setenv("PERF_TIMESTREAM_TABLE", "perfResults")
		GinkgoT().Setenv("PERF_TIMESTREAM_REGION", "us-east-2")
		Expect(performancev2.AWS(env.Clients{}).Sinks).To(HaveLen(1)) // building the client makes no AWS call
	})

	It("reads the run's dimensions from the CI environment", func() {
		for k, v := range map[string]string{"GITHUB_RUN_ID": "9", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_EVENT_NAME": "workflow_dispatch",
			"CLUSTER_NAME": "perf-pods-9-1", "AWS_REGION": "us-west-2", "K8S_VERSION": "1.36", "CP_TIER": "standard", "CONTROLLER_SHAPE": "cpu=-/-,mem=-/-"} {
			GinkgoT().Setenv(k, v)
		}
		Expect(performancev2.RunInfoFromEnv()).To(Equal(performancev2.RunInfo{RunID: "9", RunAttempt: "1", Trigger: "workflow_dispatch",
			Cluster: "perf-pods-9-1", Region: "us-west-2", K8sVersion: "1.36", ControlPlaneTier: "standard", ControllerShape: "cpu=-/-,mem=-/-"}))
	})
})

// Through core's run: the plugin's sink gets each phase's Result with its gates and the provider's own metric.
var sunk = &fakeTimestream{}

var _ = perf.Suite(func() *perf.Provider {
	return provider(perf.Levels{"sink/scaleOut": {performancev2.AWSAPICalls.Experimental(1)}}, func(p *perf.Provider) {
		p.Sinks = []func(context.Context, *perf.Result) error{
			performancev2.TimestreamSink(sunk, "db", "t", run, append(append([]perf.Metric(nil), perf.Metrics...), p.Metrics...)),
		}
	})
}, fakeScenario("sink", func(rep *perf.Result) {
	r, ok := record(sunk.records(), "aws_api_calls")
	Expect(ok).To(BeTrue())
	Expect(measures(r)).To(HaveKeyWithValue("gated", measure{"true", "BOOLEAN"}))
	Expect(measures(r)).To(HaveKeyWithValue("passed", measure{"false", "BOOLEAN"}))
	Expect(dimensions(r)).To(HaveKeyWithValue("id", "sink/scaleOut"))
	Expect(aws.ToString(r.Time)).To(Equal(fmt.Sprint(rep.Time.UnixMilli())))
	Expect(sunk.records()).To(HaveLen(len(perf.Metrics) + 1))
}))
