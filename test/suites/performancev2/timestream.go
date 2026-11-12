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
)

// RunInfo is the CI context of a run, written as dimensions on every record. Empty fields are left out.
type RunInfo struct {
	RunID, RunAttempt, Trigger, Cluster, Region, K8sVersion, ControlPlaneTier, ControllerShape string
}

// RunInfoFromEnv reads RunInfo from the variables e2e-perf.yaml sets (and the GITHUB_ ones Actions always sets).
func RunInfoFromEnv() RunInfo {
	return RunInfo{
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
		if seen[name] || math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
		seen[name] = true
		g, gated := gates[name]
		tier := TierMeasured
		measures := []timestreamtypes.MeasureValue{double("value", value), boolean("gated", gated)}
		if gated {
			tier, upper = string(g.Tier), g.Upper
			if !math.IsNaN(g.Bound) && !math.IsInf(g.Bound, 0) {
				measures = append(measures, double("bound", g.Bound))
			}
			measures = append(measures, boolean("passed", g.Passed), boolean("overridden", g.Overridden))
		}
		out = append(out, timestreamtypes.Record{
			MeasureName:      aws.String(name),
			MeasureValueType: timestreamtypes.MeasureValueTypeMulti,
			MeasureValues:    measures,
			Dimensions:       append(append([]timestreamtypes.Dimension(nil), common...), dims("tier", tier, "upper", strconv.FormatBool(upper))...),
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

func commonDimensions(r *perf.Result, run RunInfo) []timestreamtypes.Dimension {
	return dims(
		"provider", r.Provider, "scenario", r.Scenario, "phase", r.Phase, "id", r.ID,
		"schema_version", strconv.Itoa(r.SchemaVersion), "commit", r.Commit, "sizes", sizes(r.Sizes),
		"run_id", run.RunID, "run_attempt", run.RunAttempt, "trigger", run.Trigger,
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

func double(name string, v float64) timestreamtypes.MeasureValue {
	return timestreamtypes.MeasureValue{Name: aws.String(name), Value: aws.String(strconv.FormatFloat(v, 'f', -1, 64)), Type: timestreamtypes.MeasureValueTypeDouble}
}

func boolean(name string, v bool) timestreamtypes.MeasureValue {
	return timestreamtypes.MeasureValue{Name: aws.String(name), Value: aws.String(strconv.FormatBool(v)), Type: timestreamtypes.MeasureValueTypeBoolean}
}
