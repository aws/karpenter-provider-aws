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

package performancev2

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	timestreamtypes "github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/test/suites/performancev2/perf"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

const (
	// timestreamBatch is the WriteRecords limit on records per call.
	timestreamBatch = 100
	// timestreamTimeout bounds one phase's writes, so a slow endpoint can't stall the spec.
	timestreamTimeout = time.Minute
	// TierMeasured is the tier dimension of a metric with no gate in the phase.
	TierMeasured = "measured"
	// warnHeadroomPct is the headroom under which a passing gate's status is WARN, as in the harvester.
	warnHeadroomPct = 10
)

// pseudoVersionHash is the commit at the end of a Go pseudo-version, e.g. v1.15.1-0.20261110002142-d9c8e7cf2445.
var pseudoVersionHash = regexp.MustCompile(`[-.][0-9]{14}-([0-9a-f]{12})$`)

// RunInfo is the CI context of a run, written as dimensions on every record. Empty fields are left out.
type RunInfo struct {
	RunID, RunAttempt, RunURL, Trigger, Cluster, Region, K8sVersion, ControlPlaneTier, ControllerShape string
	// CoreCommit is the sigs.k8s.io/karpenter the suite (and so the KWOK controller, from the same go.mod) was built with.
	CoreCommit string
}

// RunInfoFromEnv reads RunInfo from the variables e2e-perf.yaml sets (and the GITHUB_ ones Actions always sets).
func RunInfoFromEnv() RunInfo {
	var url string
	if server, repo, id := os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID"); server != "" && repo != "" && id != "" {
		url = fmt.Sprintf("%s/%s/actions/runs/%s", server, repo, id)
	}
	return RunInfo{
		RunURL:           url,
		CoreCommit:       CoreCommit(),
		RunID:            os.Getenv("GITHUB_RUN_ID"),
		RunAttempt:       os.Getenv("GITHUB_RUN_ATTEMPT"),
		Trigger:          os.Getenv("GITHUB_EVENT_NAME"),
		Cluster:          os.Getenv("CLUSTER_NAME"),
		Region:           os.Getenv("AWS_REGION"),
		K8sVersion:       os.Getenv("K8S_VERSION"),
		ControlPlaneTier: os.Getenv("CP_TIER"),
		ControllerShape:  os.Getenv("CONTROLLER_SHAPE"),
	}
}

// CoreCommit is the commit of the sigs.k8s.io/karpenter module in this binary's build info: the hash of a
// pseudo-version, or else the version (a tag), following any replace directive. Empty without build info.
func CoreCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, m := range info.Deps {
		if m.Path != "sigs.k8s.io/karpenter" {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}
		return coreCommit(m.Version)
	}
	return ""
}

func coreCommit(version string) string {
	if h := pseudoVersionHash.FindStringSubmatch(version); h != nil {
		return h[1]
	}
	return version
}

// timestreamSinkFromEnv returns the Timestream sink when PERF_TIMESTREAM_TABLE is set, and nil otherwise, so a local
// run writes nothing. PERF_TIMESTREAM_DATABASE defaults to karpenterTesting and PERF_TIMESTREAM_REGION to us-east-2,
// where the e2e suites' tables are. The client is lazy: building it makes no AWS call.
func timestreamSinkFromEnv(ctx context.Context, metrics []perf.Metric) (func(context.Context, *perf.Result) error, error) {
	table := os.Getenv("PERF_TIMESTREAM_TABLE")
	if table == "" {
		return nil, nil
	}
	db := lo.CoalesceOrEmpty(os.Getenv("PERF_TIMESTREAM_DATABASE"), "karpenterTesting")
	// WriteRecords requires endpoint discovery, which the client turns on (EndpointDiscoveryAuto) by itself.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(lo.CoalesceOrEmpty(os.Getenv("PERF_TIMESTREAM_REGION"), "us-east-2")))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for the Timestream sink: %w", err)
	}
	return TimestreamSink(timestreamwrite.NewFromConfig(cfg), db, table, RunInfoFromEnv(), metrics), nil
}

// TimestreamSink writes one multi-measure record per metric of each phase to db.table: every one of metrics (core's
// perf.Metrics plus the provider's), gated or not, and any gate whose metric isn't in metrics. measure_name is the
// metric name. A rejected record or a failed call is returned as an error, which core logs without failing the spec;
// the other batches are still written.
func TimestreamSink(api sdk.TimestreamWriteAPI, db, table string, run RunInfo, metrics []perf.Metric) func(context.Context, *perf.Result) error {
	return func(ctx context.Context, r *perf.Result) (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("timestream sink panicked: %v", p)
			}
		}()
		ctx, cancel := context.WithTimeout(ctx, timestreamTimeout)
		defer cancel()
		records := TimestreamRecords(r, run, metrics)
		var errs []error
		for start := 0; start < len(records); start += timestreamBatch {
			batch := records[start:min(start+timestreamBatch, len(records))]
			_, werr := api.WriteRecords(ctx, &timestreamwrite.WriteRecordsInput{
				DatabaseName: aws.String(db), TableName: aws.String(table), Records: batch,
			})
			if werr != nil {
				errs = append(errs, fmt.Errorf("writing %s records %d-%d to %s.%s: %s", r.ID, start, start+len(batch)-1, db, table, describe(werr, batch)))
			}
		}
		return errors.Join(errs...)
	}
}

// describe adds each rejected record's metric and reason to a RejectedRecordsException.
func describe(err error, batch []timestreamtypes.Record) string {
	var rejected *timestreamtypes.RejectedRecordsException
	if !errors.As(err, &rejected) {
		return err.Error()
	}
	reasons := lo.Map(rejected.RejectedRecords, func(rr timestreamtypes.RejectedRecord, _ int) string {
		name := "?"
		if i := int(rr.RecordIndex); i >= 0 && i < len(batch) {
			name = aws.ToString(batch[i].MeasureName)
		}
		return fmt.Sprintf("%s: %s", name, aws.ToString(rr.Reason))
	})
	return fmt.Sprintf("%s (rejected: %s)", err.Error(), strings.Join(reasons, "; "))
}

// TimestreamRecords builds the phase's records. NaN and Inf values are skipped, since Timestream can't store them.
func TimestreamRecords(r *perf.Result, run RunInfo, metrics []perf.Metric) []timestreamtypes.Record {
	gates := lo.SliceToMap(r.Gates, func(g perf.GateResult) (string, perf.GateResult) { return g.Metric, g })
	common := commonDimensions(r, run)
	var out []timestreamtypes.Record
	seen := map[string]bool{}
	add := func(name string, value float64, upper bool) {
		if seen[name] || !finite(value) {
			return
		}
		seen[name] = true
		g, gated := gates[name]
		tier := TierMeasured
		if gated {
			tier, upper = string(g.Tier), g.Upper
		}
		measures := []timestreamtypes.MeasureValue{double("value", value), boolean("gated", gated), boolean("upper", upper)}
		if gated {
			measures = append(measures, gateMeasures(value, g)...)
		}
		out = append(out, timestreamtypes.Record{
			MeasureName:      aws.String(name),
			MeasureValueType: timestreamtypes.MeasureValueTypeMulti,
			MeasureValues:    measures,
			Dimensions:       append(append([]timestreamtypes.Dimension(nil), common...), dims("tier", tier)...),
			Time:             aws.String(strconv.FormatInt(r.Time.UnixMilli(), 10)),
			TimeUnit:         timestreamtypes.TimeUnitMilliseconds,
		})
	}
	for _, m := range metrics {
		add(m.Name, m.Value(r), m.Upper)
	}
	for _, g := range r.Gates {
		add(g.Metric, g.Value, g.Upper)
	}
	return out
}

// gateMeasures are a gated metric's bound, headroom_pct, passed, overridden and status. status is FAIL when the gate
// failed, WARN when it passed with under 10% headroom, and PASS otherwise.
func gateMeasures(value float64, g perf.GateResult) []timestreamtypes.MeasureValue {
	var out []timestreamtypes.MeasureValue
	status := "PASS"
	if finite(g.Bound) {
		out = append(out, double("bound", g.Bound))
		if h, ok := headroomPct(value, g.Bound, g.Upper); ok {
			out = append(out, double("headroom_pct", h))
			if h < warnHeadroomPct {
				status = "WARN"
			}
		}
	}
	if !g.Passed {
		status = "FAIL"
	}
	return append(out, boolean("passed", g.Passed), boolean("overridden", g.Overridden), varchar("status", status))
}

func commonDimensions(r *perf.Result, run RunInfo) []timestreamtypes.Dimension {
	return dims(
		"provider", r.Provider, "scenario", r.Scenario, "phase", r.Phase, "id", r.ID,
		"schema_version", strconv.Itoa(r.SchemaVersion), "sizes", sizes(r.Sizes),
		// r.Commit is the controller's karpenter_build_info commit (its vcs.revision), i.e. this repo at git_ref
		"commit", r.Commit, "aws_commit", r.Commit, "core_commit", run.CoreCommit,
		"run_id", run.RunID, "run_attempt", run.RunAttempt, "run_url", run.RunURL, "trigger", run.Trigger,
		"cluster", run.Cluster, "region", run.Region, "k8s_version", run.K8sVersion,
		"cp_tier", run.ControlPlaneTier, "controller_shape", run.ControllerShape,
	)
}

// dims turns name/value pairs into dimensions, leaving out empty values, which Timestream rejects.
func dims(kv ...string) []timestreamtypes.Dimension {
	var out []timestreamtypes.Dimension
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			out = append(out, timestreamtypes.Dimension{Name: aws.String(kv[i]), Value: aws.String(kv[i+1])})
		}
	}
	return out
}

// sizes is the sorted "vector=n" list joined by commas, as the reports' file names and the harvester write it.
func sizes(s map[perf.Vector]int) string {
	parts := lo.MapToSlice(s, func(v perf.Vector, n int) string { return fmt.Sprintf("%s=%d", v, n) })
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// headroomPct is the room left to the bound as a percent of it: positive while passing, negative past the bound, for
// either direction. Not defined for a zero bound.
func headroomPct(value, bound float64, upper bool) (float64, bool) {
	if bound == 0 {
		return 0, false
	}
	h := (value - bound) / math.Abs(bound) * 100
	if upper {
		h = -h
	}
	return h, finite(h)
}

func varchar(name, v string) timestreamtypes.MeasureValue {
	return timestreamtypes.MeasureValue{Name: aws.String(name), Value: aws.String(v), Type: timestreamtypes.MeasureValueTypeVarchar}
}

func double(name string, v float64) timestreamtypes.MeasureValue {
	return timestreamtypes.MeasureValue{Name: aws.String(name), Value: aws.String(strconv.FormatFloat(v, 'f', -1, 64)), Type: timestreamtypes.MeasureValueTypeDouble}
}

func boolean(name string, v bool) timestreamtypes.MeasureValue {
	return timestreamtypes.MeasureValue{Name: aws.String(name), Value: aws.String(strconv.FormatBool(v)), Type: timestreamtypes.MeasureValueTypeBoolean}
}
