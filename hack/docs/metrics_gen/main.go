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

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type metricInfo struct {
	namespace string
	subsystem string
	name      string
	help      string
	labels    []string
	// Scopes dimension resolution to one code base, disambiguating names reused with
	// different meanings (e.g. `reason` differs between karpenter and operatorpkg).
	labelScope string
	// Per-metric override for a dimension's documented values, when the value set is
	// metric-specific not global (e.g. per-object status-condition `type`).
	labelValues map[string][]valueInfo
	// labelInfos are the docs resolved from this metric's own []Label argument,
	// keyed by dimension name. Preferred over the global registry so same-named
	// dimensions with different values (e.g. the three `reason` Labels) don't collapse.
	labelInfos map[string]labelInfo
	// metricType is the Prometheus type (Counter/Gauge/Histogram/Summary).
	metricType string
	// stability is the documented level (STABLE/BETA/ALPHA/DEPRECATED) resolved from
	// the constructor's Stage arg. Empty when the constructor carries no Stage (the
	// parsed core is not yet on Stage, or a plain prometheus.* metric); such metrics
	// fall back to the stability lists at render time.
	stability string
}

// stabilityFromStage maps a constructor's Stage arg (opmetrics.GA/Beta/Alpha) to the
// documented stability level — the source of truth when present.
func stabilityFromStage(expr ast.Expr) string {
	switch identName(expr) {
	case "GA", "Stable":
		return "STABLE"
	case "Beta":
		return "BETA"
	case "Alpha":
		return "ALPHA"
	case "Deprecated":
		return "DEPRECATED"
	}
	return ""
}

// metricTypeFromCall derives the Prometheus type from the constructor name, e.g.
// NewPrometheusCounter / NewCounterVec -> Counter.
func metricTypeFromCall(fun ast.Expr) string {
	switch name := funcCallName(fun); {
	case strings.Contains(name, "Counter"):
		return "Counter"
	case strings.Contains(name, "Gauge"):
		return "Gauge"
	case strings.Contains(name, "Histogram"):
		return "Histogram"
	case strings.Contains(name, "Summary"):
		return "Summary"
	}
	return ""
}

var (
	// Maps a string const/var name to its value (ReasonLabel -> "reason"), to resolve
	// label names declared as constants.
	stringSymbols = map[string]string{}
	// Maps a []string const/var name to its values (e.g. aws-sdk-go-prometheus labels).
	sliceSymbols = map[string][]string{}
	// Maps a []Value var name to its values, so a Label.Values referencing a shared var
	// resolves like an inline literal.
	valueSliceSymbols = map[string][]valueInfo{}
	// Maps a Value var name to its value, so a []Value literal can reference Value vars by name.
	valueSymbols = map[string]valueInfo{}
	// Maps object Kind -> its status condition types (from metrics.ConditionTypeValues);
	// the `type` dimension's per-object value set.
	conditionTypesByKind = map[string][]valueInfo{}
	// ambiguous names resolved to conflicting values across packages; treated as unresolvable.
	ambiguousStrings = map[string]bool{}
	ambiguousSlices  = map[string]bool{}

	// Maps dimension name -> docs from metrics.Label{...} declarations; source of truth
	// for per-dimension help/values.
	labelRegistry = map[string]labelInfo{}
	// labelVarName / labelVarInfo resolve a Label by its Go var name, so a metric can
	// pick the specific Label it references even when several share a dimension name.
	labelVarName = map[string]string{}
	labelVarInfo = map[string]labelInfo{}
	// ambiguousLabelVars: same var name, differing docs across packages — can't tell
	// them apart from a bare identifier, so emit the dimension name without docs.
	ambiguousLabelVars = map[string]bool{}
	// labelSliceSymbols resolves a named []opmetrics.Label var, so a metric that
	// references one documents like an inline []Label{...} literal.
	labelSliceSymbols = map[string]labelSlice{}
	// funcReturns holds single-return helper bodies so a metric whose label argument
	// is a call (labelNames(), nodeLabelNames()) resolves by inlining the returned
	// expression; resolvingFuncs guards the recursion.
	funcReturns    = map[string]ast.Expr{}
	resolvingFuncs = map[string]bool{}
	// ambiguousFuncs: a single-return helper name defined differently across packages;
	// a bare call can't disambiguate them, so don't resolve it (avoids documenting the
	// wrong package's dimensions). funcReturnText fingerprints the first body per name.
	ambiguousFuncs = map[string]bool{}
	funcReturnText = map[string]string{}
	// unresolvedLabelMetrics: metrics whose label arg didn't resolve, reported to
	// stderr so a silently dimension-less doc entry is at least visible.
	unresolvedLabelMetrics []string
	// Label docs keyed by scope (declaring code base) then name; a metric with a
	// matching labelScope resolves here first, so a reused name (e.g. `reason`) gets
	// its own code base's docs.
	scopedLabelRegistry = map[string]map[string]labelInfo{}
)

// labelScopeForFile returns the scope a Label declaration belongs to, based on
// the file it was declared in. operatorpkg-declared Labels document the
// dimensions of operatorpkg's status/termination/event metrics. Matching an
// "operatorpkg" path segment (rather than any substring) avoids misattributing
// files that merely sit under a checkout/branch dir whose name contains it.
func labelScopeForFile(file string) string {
	if strings.Contains(file, "/operatorpkg/") || strings.Contains(file, "/operatorpkg@") {
		return "operatorpkg"
	}
	return ""
}

// labelInfo is the resolved documentation for a metric dimension.
type labelInfo struct {
	help   string
	values []valueInfo
}

// valueInfo is the resolved documentation for a single dimension value.
type valueInfo struct {
	name string
	help string
}

// Maps a metric to a []Value var this provider merges into its `reason` dimension,
// for reasons a core Label can't enumerate (EC2 event kinds -> nodeclaims_disrupted_total).
var reasonValueContributions = map[string]string{
	"karpenter_nodeclaims_disrupted_total": "interruptionKindValues",
}

// Docs for THIRD-PARTY metric dimensions (aws-sdk-go, controller-runtime, client-go)
// that can't carry a metrics.Label. Keyed by subsystem because a name (e.g. `code`)
// means different things across subsystems.
var labelInjections = map[string]map[string]labelInfo{
	"aws_sdk_go": {
		"service": {help: "The AWS service the request was made to, e.g. `EC2`."},
		"action":  {help: "The AWS API operation invoked, e.g. `DescribeSubnets`."},
		"code":    {help: "The HTTP status code of the response, e.g. `200`, `503`."},
	},
	"controller_runtime": {
		"controller": {help: "The name of the controller that owns the reconcile loop."},
		"name":       {help: "The name of the controller instance."},
		"result":     {help: "The outcome of the reconcile call.", values: []valueInfo{{name: "success"}, {name: "error"}, {name: "requeue"}, {name: "requeue_after"}}},
	},
	"client_go": {
		"verb":        {help: "The HTTP verb of the Kubernetes API request, e.g. `GET`, `POST`."},
		"code":        {help: "The HTTP status code of the Kubernetes API response."},
		"method":      {help: "The HTTP method of the Kubernetes API request."},
		"host":        {help: "The Kubernetes API server host the request was made to."},
		"group":       {help: "The API group of the request's target resource."},
		"version":     {help: "The API version of the request's target resource."},
		"kind":        {help: "The kind of the request's target resource."},
		"subresource": {help: "The subresource of the request, if any."},
	},
	"workqueue": {
		"name":       {help: "The name of the workqueue, typically the owning controller's name."},
		"controller": {help: "The name of the controller that emitted the metric."},
		"priority":   {help: "The priority band of the enqueued item."},
	},
	"leader_election": {
		"name": {help: "The name of the lease used for leader election."},
	},
}

func describeLabel(subsystem, name, scope string) (labelInfo, bool) {
	if inj, ok := labelInjections[subsystem]; ok {
		if li, ok := inj[name]; ok {
			return li, true
		}
	}
	if scope != "" {
		if scoped, ok := scopedLabelRegistry[scope]; ok {
			if li, ok := scoped[name]; ok {
				return li, true
			}
		}
	}
	if li, ok := labelRegistry[name]; ok {
		return li, true
	}
	return labelInfo{}, false
}

// Fallback stability for metrics whose constructor carries no Stage arg: third-party
// library metrics (controller-runtime, aws-sdk-go, client-go, ...) and the parsed
// karpenter core until it adopts Stage. Metrics that DO declare a Stage resolve from it
// directly (see stabilityFromStage), so these lists need not track them.
var (
	stableMetrics = []string{"controller_runtime", "aws_sdk_go", "client_go", "leader_election", "interruption", "cluster_state", "workqueue", "karpenter_build_info", "karpenter_nodepools_usage", "karpenter_nodepools_limit",
		"karpenter_nodeclaims_terminated_total", "karpenter_nodeclaims_created_total", "karpenter_nodes_terminated_total", "karpenter_nodes_created_total", "karpenter_pods_startup_duration_seconds",
		"karpenter_scheduler_scheduling_duration_seconds", "karpenter_nodepools_allowed_disruptions", "karpenter_voluntary_disruption_decisions_total"}
	betaMetrics = []string{"cloudprovider", "cloudprovider_batcher", "karpenter_nodeclaims_termination_duration_seconds", "karpenter_nodeclaims_instance_termination_duration_seconds",
		"karpenter_nodes_total_pod_requests", "karpenter_nodes_total_pod_limits", "karpenter_nodes_total_daemon_requests", "karpenter_nodes_total_daemon_limits", "karpenter_nodes_termination_duration_seconds",
		"karpenter_nodes_system_overhead", "karpenter_nodes_allocatable", "karpenter_pods_state", "karpenter_scheduler_queue_depth", "karpenter_voluntary_disruption_queue_failures_total",
		"karpenter_voluntary_disruption_decision_evaluation_duration_seconds", "karpenter_voluntary_disruption_eligible_nodes", "karpenter_voluntary_disruption_consolidation_timeouts_total",
		"nodeclaim_status_condition", "nodeclaim_termination",
		"nodepool_status_condition", "nodepool_termination",
		"ec2nodeclass_status_condition", "ec2nodeclass_termination"}
	// Deprecated generic status/termination metrics (no object prefix); still emitted
	// but superseded by per-object variants.
	deprecatedMetrics = []string{"status_condition", "termination"}
)

func (i metricInfo) qualifiedName() string {
	return strings.Join(lo.Compact([]string{i.namespace, i.subsystem, i.name}), "_")
}

// stabilityFromLists resolves a Stage-less metric's level by subsystem or qualified name.
func stabilityFromLists(m metricInfo) string {
	switch {
	case slices.Contains(deprecatedMetrics, m.subsystem) || slices.Contains(deprecatedMetrics, m.qualifiedName()):
		return "DEPRECATED"
	case slices.Contains(stableMetrics, m.subsystem) || slices.Contains(stableMetrics, m.qualifiedName()):
		return "STABLE"
	case slices.Contains(betaMetrics, m.subsystem) || slices.Contains(betaMetrics, m.qualifiedName()):
		return "BETA"
	default:
		return "ALPHA"
	}
}

// subsystemTitleOverrides set section headings for subsystems whose canonical
// capitalization the word-by-word title-caser can't reproduce.
var subsystemTitleOverrides = map[string]string{
	"ec2nodeclasses": "EC2NodeClasses",
}

// metrics_gen parses source for Prometheus metric declarations and generates the metrics markdown docs.
func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatalf("Usage: %s path/to/metrics/controller path/to/metrics/controller2 path/to/markdown.md", os.Args[0])
	}
	var allPackages []*ast.Package
	for i := 0; i < flag.NArg()-1; i++ {
		allPackages = append(allPackages, getPackages(flag.Arg(i))...)
	}
	collectSymbols(allPackages)
	collectFuncReturns(allPackages)
	// must run after collectSymbols so Name/Values identifiers resolve.
	collectLabels(allPackages)
	// must run after collectLabels so referenced Label vars are resolved.
	collectLabelSlices(allPackages)
	allMetrics := getMetricsFromPackages(allPackages...)

	// per-object status metrics are created at runtime from status.NewController[T]()
	// and can't be read from a declaration; synthesize them from the registration
	// sites (+ deprecated + client_go).
	statusObjects := parseStatusControllerObjects(allPackages)
	allMetrics = append(allMetrics, perObjectStatusMetrics(statusObjects)...)
	allMetrics = append(allMetrics, deprecatedStatusMetrics(statusObjects)...)
	allMetrics = append(allMetrics, hardcodedMetrics()...)

	allMetrics = lo.UniqBy(allMetrics, func(m metricInfo) string {
		return fmt.Sprintf("%s/%s/%s", m.namespace, m.subsystem, m.name)
	})

	for _, subsystem := range []string{"rest_client", "certwatcher_read", "controller_runtime_webhook"} {
		allMetrics = lo.Reject(allMetrics, func(m metricInfo, _ int) bool {
			return strings.HasPrefix(m.name, subsystem)
		})
	}

	// controller-runtime and aws-sdk-go metrics carry no namespace/subsystem, so split it out of the name here.
	for _, subsystem := range []string{"controller_runtime", "aws_sdk_go", "client_go", "leader_election"} {
		for i := range allMetrics {
			if allMetrics[i].subsystem == "" && strings.HasPrefix(allMetrics[i].name, fmt.Sprintf("%s_", subsystem)) {
				allMetrics[i].subsystem = subsystem
				allMetrics[i].name = strings.TrimPrefix(allMetrics[i].name, fmt.Sprintf("%s_", subsystem))
			}
		}
	}
	sort.Slice(allMetrics, bySubsystem(allMetrics))

	// print metrics with unresolved label args so the missing dimensions are visible, not silently shipped.
	if len(unresolvedLabelMetrics) > 0 {
		slices.Sort(unresolvedLabelMetrics)
		unresolvedLabelMetrics = slices.Compact(unresolvedLabelMetrics)
		fmt.Fprintf(os.Stderr, "WARNING: %d metric(s) have an unresolvable label argument; their dimensions are omitted from the docs:\n", len(unresolvedLabelMetrics))
		for _, m := range unresolvedLabelMetrics {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
	}

	// surface a silent gap: without the per-Kind condition registry, every
	// status-condition `type` dimension documents no values. Warn rather than fail,
	// since the registry may legitimately be absent from the parsed sources.
	if len(conditionTypesByKind) == 0 {
		fmt.Fprintln(os.Stderr, "WARNING: no condition-type registry (map[string][]Value, e.g. metrics.ConditionTypeValues) resolved; status-condition `type` dimensions will have no enumerated values")
	}

	// fail loudly if the count drops below expected — catches silent regressions from
	// new identifiers or changed declaration patterns. Bump when metrics are intentionally removed.
	const minExpectedMetrics = 100
	if len(allMetrics) < minExpectedMetrics {
		log.Fatalf("expected at least %d metrics but only found %d; the generator may be silently dropping metrics due to unrecognized identifiers or new declaration patterns", minExpectedMetrics, len(allMetrics))
	}

	// The static prose (frontmatter, intro, and the metric-type explainer) is
	// hand-maintained in the output file; the generator only replaces the metric
	// tables between the marker comments, mirroring configuration_gen.
	outputFileName := flag.Arg(flag.NArg() - 1)
	mdFile, err := os.ReadFile(outputFileName)
	if err != nil {
		log.Fatalf("error reading output file %s, %s", outputFileName, err)
	}
	const genStart = "[comment]: <> (the content below is generated from hack/docs/metrics_gen/main.go)"
	const genEnd = "[comment]: <> (end docs generated content from hack/docs/metrics_gen/main.go)"
	startSections := strings.Split(string(mdFile), genStart)
	if len(startSections) != 2 {
		log.Fatalf("expected one generated block start marker in %s but got %d", outputFileName, len(startSections)-1)
	}
	endSections := strings.Split(string(mdFile), genEnd)
	if len(endSections) != 2 {
		log.Fatalf("expected one generated block end marker in %s but got %d", outputFileName, len(endSections)-1)
	}
	topDoc := fmt.Sprintf("%s%s\n\n", startSections[0], genStart)
	bottomDoc := fmt.Sprintf("\n%s%s", genEnd, endSections[1])

	var b strings.Builder
	previousSubsystem := ""
	for _, metric := range allMetrics {
		if metric.subsystem != previousSubsystem {
			if metric.subsystem != "" {
				subsystemTitle, ok := subsystemTitleOverrides[metric.subsystem]
				if !ok {
					subsystemTitle = strings.Join(lo.Map(strings.Split(metric.subsystem, "_"), func(s string, _ int) string {
						if s == "sdk" || s == "aws" {
							return strings.ToUpper(s)
						}
						return fmt.Sprintf("%s%s", strings.ToUpper(s[0:1]), s[1:])
					}), " ")
				}
				fmt.Fprintf(&b, "## %s Metrics\n", subsystemTitle)
				fmt.Fprintln(&b)
			}
			previousSubsystem = metric.subsystem
		}
		fmt.Fprintf(&b, "### `%s`\n", metric.qualifiedName())
		fmt.Fprintf(&b, "%s\n", metric.help)
		if metric.metricType != "" {
			fmt.Fprintf(&b, "- Type: [%s](https://prometheus.io/docs/concepts/metric_types/#%s)\n", metric.metricType, strings.ToLower(metric.metricType))
		}
		// the constructor's Stage arg is authoritative; fall back to the lists only for
		// metrics that carry no Stage (the parsed core, third-party library metrics).
		level := metric.stability
		if level == "" {
			level = stabilityFromLists(metric)
		}
		fmt.Fprintf(&b, "- Stability Level: %s\n", level)
		if dims := formatDimensions(metric); dims != "" {
			fmt.Fprintf(&b, "- Dimensions:%s\n", dims)
		}
		fmt.Fprintln(&b)
	}

	log.Println("writing output to", outputFileName)
	if err := os.WriteFile(outputFileName, []byte(topDoc+b.String()+bottomDoc), 0o644); err != nil {
		log.Fatalf("error writing output file %s, %s", outputFileName, err)
	}
}

// renders dimensions as a markdown sub-list. The leading newline makes it nest under the "- Dimensions:" line.
func formatDimensions(m metricInfo) string {
	if len(m.labels) == 0 {
		return ""
	}
	var b strings.Builder
	for _, l := range m.labels {
		b.WriteString(fmt.Sprintf("\n  - `%s`", l))
		info, ok := describeLabel(m.subsystem, l, m.labelScope)
		// a Label from THIS metric's []Label arg is authoritative — disambiguates
		// dimensions like `reason` that the global registry collapses.
		if mi, has := m.labelInfos[l]; has {
			info, ok = mi, true
		}
		if ok && info.help != "" {
			b.WriteString(fmt.Sprintf(" — %s", info.help))
		}
		// A metric-specific value set (e.g. per-object condition types) overrides the
		// dimension's global values.
		values := info.values
		if override, ok := m.labelValues[l]; ok {
			values = override
		}
		// a provider may emit extra values into a core metric's dimension (EC2 event
		// kinds -> nodeclaims_disrupted_total reason); merge deduped.
		if varName, ok := reasonValueContributions[m.qualifiedName()]; ok && l == metrics.ReasonLabel {
			seen := map[string]bool{}
			for _, v := range values {
				seen[v.name] = true
			}
			for _, v := range valueSliceSymbols[varName] {
				if !seen[v.name] {
					values = append(values, v)
					seen[v.name] = true
				}
			}
		}
		for _, v := range values {
			b.WriteString(fmt.Sprintf("\n    - `%s`", v.name))
			if v.help != "" {
				b.WriteString(fmt.Sprintf(" — %s", v.help))
			}
		}
	}
	return b.String()
}

func getPackages(root string) []*ast.Package {
	var packages []*ast.Package
	fset := token.NewFileSet()

	log.Println("parsing code in", root)
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		pkgs, err := parser.ParseDir(fset, path, func(info fs.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, parser.AllErrors)
		if err != nil {
			log.Fatalf("error parsing, %s", err)
		}
		// iterate packages in a stable order; ParseDir returns a map, so a directory
		// with two non-test packages would otherwise resolve nondeterministically.
		for _, name := range slices.Sorted(maps.Keys(pkgs)) {
			pkg := pkgs[name]
			if strings.HasSuffix(pkg.Name, "_test") {
				continue
			}
			packages = append(packages, pkg)
		}
		return nil
	})
	return packages
}

func getMetricsFromPackages(packages ...*ast.Package) []metricInfo {
	var allMetrics []metricInfo
	for _, pkg := range packages {
		for _, filePath := range slices.Sorted(maps.Keys(pkg.Files)) {
			ast.Inspect(pkg.Files[filePath], func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if m, ok := metricFromCallExpr(ce); ok {
					allMetrics = append(allMetrics, m)
				}
				return true
			})
		}
	}
	return allMetrics
}

func bySubsystem(metrics []metricInfo) func(i int, j int) bool {
	// Higher ordering comes first. If a value isn't designated here then the subsystem will be given a default of 0.
	// Metrics without a subsystem come first since there is no designation for the bucket they fall under
	subSystemSortOrder := map[string]int{
		"":                              100,
		"nodepools":                     10,
		"nodeclaims":                    9,
		"nodeclaim_status_condition":    8,
		"nodeclaim_termination":         8,
		"nodes":                         7,
		"pods":                          5,
		"nodepool_status_condition":     4,
		"nodepool_termination":          4,
		"ec2nodeclass_status_condition": 3,
		"ec2nodeclass_termination":      3,
		"status_condition":              -1,
		"termination":                   -1,
		"workqueue":                     -1,
		"client_go":                     -1,
		"aws_sdk_go":                    -1,
		"leader_election":               -2,
	}

	return func(i, j int) bool {
		lhs := metrics[i]
		rhs := metrics[j]
		if subSystemSortOrder[lhs.subsystem] != subSystemSortOrder[rhs.subsystem] {
			return subSystemSortOrder[lhs.subsystem] > subSystemSortOrder[rhs.subsystem]
		}
		return lhs.qualifiedName() > rhs.qualifiedName()
	}
}

// statusObject is a type that has a status controller registered via
// status.NewController[T](), parsed from the registration sites.
type statusObject struct {
	kind      string // the object Kind, e.g. "NodeClaim"
	subsystem string // the metric subsystem prefix, e.g. "nodeclaim"
}

// finds status.NewController[T]() registrations — the self-maintaining source for
// which per-object metrics exist (replacing a hardcoded list).
func parseStatusControllerObjects(packages []*ast.Package) []statusObject {
	seen := map[string]statusObject{}
	for _, pkg := range packages {
		for _, filePath := range slices.Sorted(maps.Keys(pkg.Files)) {
			ast.Inspect(pkg.Files[filePath], func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				// A generic call fn[T](...) parses as a call whose Fun is an IndexExpr
				// (one type arg) or IndexListExpr (several). status.NewController[T]().
				var fun ast.Expr
				var typeArg ast.Expr
				switch idx := ce.Fun.(type) {
				case *ast.IndexExpr:
					fun, typeArg = idx.X, idx.Index
				case *ast.IndexListExpr:
					if len(idx.Indices) == 0 {
						return true
					}
					fun, typeArg = idx.X, idx.Indices[0]
				default:
					return true
				}
				// Match operatorpkg's status.NewController[T](), not any generic function
				// that happens to be named NewController.
				sel, ok := fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "NewController" {
					return true
				}
				if pkgIdent, ok := sel.X.(*ast.Ident); !ok || pkgIdent.Name != "status" {
					return true
				}
				if kind := identName(typeArg); kind != "" {
					seen[kind] = statusObject{kind: kind, subsystem: strings.ToLower(kind)}
				}
				return true
			})
		}
	}
	objs := lo.Values(seen)
	sort.Slice(objs, func(i, j int) bool { return objs[i].subsystem < objs[j].subsystem })
	return objs
}

// one operatorpkg status/termination metric family. labels are the base dimensions
// in operatorpkg's declared order; runtime per-object labels can't be determined
// statically and are omitted.
type statusMetricTemplate struct {
	subsystemSuffix string
	name            string
	help            string
	labels          []string
	metricType      string
}

func statusMetricTemplates() []statusMetricTemplate {
	return []statusMetricTemplate{
		{"status_condition", "transition_seconds", "The amount of time a condition was in a given state (status) before transitioning to another state (to_status). e.g. Alarm := P99(Updated=False) > 5 minutes", []string{"type", "status", "to_status"}, "Histogram"},
		{"status_condition", "count", "The number of a condition for a given object, type and status. e.g. Alarm := Available=False > 0", []string{"namespace", "name", "type", "status", "reason"}, "Gauge"},
		{"status_condition", "current_status_seconds", "The current amount of time in seconds that a status condition has been in a specific state. Alarm := P99(Updated=Unknown) > 5 minutes", []string{"namespace", "name", "type", "status", "reason"}, "Gauge"},
		{"status_condition", "transitions_total", "The count of transitions of a given object, type and status.", []string{"type", "status", "reason"}, "Counter"},
		{"termination", "current_time_seconds", "The current amount of time in seconds that an object has been in terminating state.", []string{"namespace", "name"}, "Gauge"},
		{"termination", "duration_seconds", "The amount of time taken by an object to terminate completely.", nil, "Histogram"},
	}
}

// synthesizes the per-object status metrics operatorpkg creates at runtime
// (unreadable from a declaration); objects is parsed from the registration sites.
func perObjectStatusMetrics(objects []statusObject) []metricInfo {
	var out []metricInfo
	for _, obj := range objects {
		for _, t := range statusMetricTemplates() {
			// The `type` dimension of an object's status-condition metrics enumerates
			// that object's condition types.
			var labelValues map[string][]valueInfo
			if types, ok := conditionTypesByKind[obj.kind]; ok && slices.Contains(t.labels, "type") {
				labelValues = map[string][]valueInfo{"type": types}
			}
			out = append(out, metricInfo{
				namespace:   "operator",
				subsystem:   fmt.Sprintf("%s_%s", obj.subsystem, t.subsystemSuffix),
				name:        t.name,
				help:        t.help,
				labels:      t.labels,
				labelScope:  "operatorpkg",
				labelValues: labelValues,
				metricType:  t.metricType,
			})
		}
	}
	return out
}

// deprecated generic status/termination metrics (no object prefix), still emitted
// under emitDeprecatedMetrics. They carry group/kind labels, so kind/type span every
// registered object.
func deprecatedStatusMetrics(objects []statusObject) []metricInfo {
	allKinds := lo.Map(objects, func(o statusObject, _ int) valueInfo { return valueInfo{name: o.kind} })
	var allTypes []valueInfo
	for _, o := range objects {
		allTypes = append(allTypes, conditionTypesByKind[o.kind]...)
	}
	// A condition type (e.g. ValidationSucceeded) can be set by more than one object;
	// dedupe by name so the union lists each once.
	allTypes = lo.UniqBy(allTypes, func(v valueInfo) string { return v.name })

	var out []metricInfo
	for _, t := range statusMetricTemplates() {
		labelValues := map[string][]valueInfo{"kind": allKinds}
		if slices.Contains(t.labels, "type") && len(allTypes) > 0 {
			labelValues["type"] = allTypes
		}
		out = append(out, metricInfo{
			namespace:   "operator",
			subsystem:   t.subsystemSuffix,
			name:        t.name,
			help:        t.help,
			labels:      slices.Concat(t.labels, []string{"group", "kind"}),
			labelScope:  "operatorpkg",
			labelValues: labelValues,
			metricType:  t.metricType,
		})
	}
	return out
}

// hardcodedMetrics are metrics that can't be parsed from any declaration: operatorpkg
// registers the client_go metrics inside RegisterClientMetrics() via unqualified
// NewPrometheus* calls (same package), which the generator doesn't recognize.
func hardcodedMetrics() []metricInfo {
	return []metricInfo{
		{name: "client_go_request_duration_seconds", help: "Request latency in seconds. Broken down by verb, group, version, kind, and subresource.", labels: []string{"verb", "group", "version", "kind", "subresource"}, metricType: "Histogram"},
		{name: "client_go_request_total", help: "Number of HTTP requests, partitioned by status code and method.", labels: []string{"code", "method"}, metricType: "Counter"},
	}
}

// extracts metric info from prometheus.New*() / opmetrics.NewPrometheus*() calls.
// pmetrics.* constructors are deliberately skipped — they build metrics from a
// runtime type param that can't resolve statically; those are synthesized by
// perObjectStatusMetrics.
func metricFromCallExpr(ce *ast.CallExpr) (metricInfo, bool) {
	funcPkg := getFuncPackage(ce.Fun)
	// prometheus.New*() has opts at Args[0]; opmetrics.NewPrometheus*() is
	// (registry, opts, labels) so Args[1].
	var optsIdx int
	switch funcPkg {
	case "prometheus":
		optsIdx = 0
	case "opmetrics":
		optsIdx = 1
	default:
		return metricInfo{}, false
	}
	if len(ce.Args) <= optsIdx {
		return metricInfo{}, false
	}
	arg, ok := ce.Args[optsIdx].(*ast.CompositeLit)
	if !ok {
		return metricInfo{}, false
	}
	keyValuePairs := map[string]string{}
	for _, el := range arg.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key := identName(kv.Key)
		switch key {
		case "Namespace", "Subsystem", "Name", "Help":
		default:
			continue
		}
		// prefer the curated identifier mapping (it intentionally overrides, e.g.
		// pluralizes subsystems), else resolve the expression directly — a literal, a
		// const/selector, or a "a"+"b"+const concatenation (all via resolveStringExpr).
		var value string
		if mk := identMappingKey(kv.Value); mk != "" {
			if v, ok := identMapping[mk]; ok {
				value = v
			} else if s, ok := resolveStringExpr(kv.Value); ok {
				value = s
			} else {
				// unresolvable ident: skip the metric rather than aborting the whole run,
				// matching the literal path below.
				return metricInfo{}, false
			}
		} else if s, ok := resolveStringExpr(kv.Value); ok {
			value = s
		} else {
			return metricInfo{}, false
		}
		keyValuePairs[key] = value
	}
	// labels are the arg after opts (absent for non-vector metrics). Best-effort:
	// unresolvable labels omit the Dimensions line rather than emit partial data.
	var labels []string
	var labelInfos map[string]labelInfo
	if labelsIdx := optsIdx + 1; len(ce.Args) > labelsIdx {
		if names, infos, ok := resolveLabelDimensions(ce.Args[labelsIdx]); ok {
			labels = names
			labelInfos = infos
		} else {
			// record unresolved label args so the omission is surfaced, not silent.
			unresolvedLabelMetrics = append(unresolvedLabelMetrics,
				strings.Trim(strings.Join([]string{keyValuePairs["Namespace"], keyValuePairs["Subsystem"], keyValuePairs["Name"]}, "_"), "_"))
		}
	}
	// stability comes from the constructor's Stage arg when present: opmetrics
	// constructors are (registry, opts, labels, stage), so Args[optsIdx+2]. Absent
	// (a plain prometheus.* metric, or the parsed core before it adopts Stage) leaves
	// it empty; render falls back to the stability lists.
	var stability string
	if funcPkg == "opmetrics" {
		if stageIdx := optsIdx + 2; len(ce.Args) > stageIdx {
			stability = stabilityFromStage(ce.Args[stageIdx])
		}
	}
	return metricInfo{
		namespace:  keyValuePairs["Namespace"],
		subsystem:  keyValuePairs["Subsystem"],
		name:       keyValuePairs["Name"],
		help:       keyValuePairs["Help"],
		labels:     labels,
		labelInfos: labelInfos,
		metricType: metricTypeFromCall(ce.Fun),
		stability:  stability,
	}, true
}

// records string and []string package-level symbols so label names declared as
// identifiers resolve. Conflicting values across packages are marked ambiguous.
func collectSymbols(packages []*ast.Package) {
	forEachValueSpec(packages, func(_, name string, value ast.Expr) {
		if s, ok := stringLiteralValue(value); ok {
			if existing, seen := stringSymbols[name]; seen && existing != s {
				ambiguousStrings[name] = true
				return
			}
			stringSymbols[name] = s
		}
	})
	// Pass 1b: resolve alias consts (X = Y, X = pkg.Y) to their underlying string.
	// Iterate to a fixpoint for aliases-of-aliases.
	const aliasResolutionPasses = 3
	for range aliasResolutionPasses {
		changed := false
		forEachValueSpec(packages, func(_, name string, value ast.Expr) {
			if _, seen := stringSymbols[name]; seen {
				return
			}
			switch value.(type) {
			case *ast.Ident, *ast.SelectorExpr:
				if s, ok := resolveStringExpr(value); ok {
					stringSymbols[name] = s
					changed = true
				}
			}
		})
		if !changed {
			break
		}
	}
	// Pass 2: []string composite literals (may reference the string symbols above).
	forEachValueSpec(packages, func(_, name string, value ast.Expr) {
		cl, ok := value.(*ast.CompositeLit)
		if !ok {
			return
		}
		vals, ok := stringSliceFromCompositeLit(cl)
		if !ok {
			return
		}
		if existing, seen := sliceSymbols[name]; seen && !slices.Equal(existing, vals) {
			ambiguousSlices[name] = true
			return
		}
		sliceSymbols[name] = vals
	})
	// Pass 3: single Value vars (first-class dimension values referenced by name from
	// a []Value literal, e.g. a metrics-owned error category).
	forEachValueSpec(packages, func(_, name string, value ast.Expr) {
		cl, ok := value.(*ast.CompositeLit)
		if !ok || identName(cl.Type) != "Value" {
			return
		}
		if v, ok := valueFromCompositeLit(cl); ok {
			valueSymbols[name] = v
		}
	})
	// Pass 4: []Value composite literals (shared value sets referenced by a Label's
	// Values field, e.g. operatorpkg's conditionStatusValues). Runs after Pass 3 so
	// elements that reference a single Value var resolve.
	forEachValueSpec(packages, func(_, name string, value ast.Expr) {
		cl, ok := value.(*ast.CompositeLit)
		if !ok || !isValueSliceType(cl.Type) {
			return
		}
		if vals, ok := valueSliceFromCompositeLit(cl); ok {
			valueSliceSymbols[name] = vals
		}
	})
	// Pass 5: the per-Kind condition-type registry (karpenter's
	// metrics.ConditionTypeValues map), used to document the `type` dimension of
	// each object's status-condition metrics.
	collectConditionTypes(packages)
}

// collectConditionTypes scans for a map[string][]Value composite literal (karpenter's
// metrics.ConditionTypeValues) and records each Kind's status condition types.
func collectConditionTypes(packages []*ast.Package) {
	forEachValueSpec(packages, func(_, _ string, value ast.Expr) {
		cl, ok := value.(*ast.CompositeLit)
		if !ok {
			return
		}
		mt, ok := cl.Type.(*ast.MapType)
		if !ok {
			return
		}
		if key, ok := mt.Key.(*ast.Ident); !ok || key.Name != "string" {
			return
		}
		if !isValueSliceType(mt.Value) {
			return
		}
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			kind, ok := resolveStringExpr(kv.Key)
			if !ok {
				continue
			}
			if vals, ok := resolveValues(kv.Value); ok {
				conditionTypesByKind[kind] = vals
			}
		}
	})
}

// records metrics.Label{...} declarations into the registry keyed by resolved Name.
// Entries merge by name — one with help wins over a bare reference.
func collectLabels(packages []*ast.Package) {
	forEachValueSpec(packages, func(file, varName string, value ast.Expr) {
		cl, ok := value.(*ast.CompositeLit)
		if !ok || !isLabelType(cl.Type) {
			return
		}
		fields := namedFields(cl)
		name, ok := resolveStringExpr(fields["Name"])
		if !ok {
			return
		}
		help, _ := resolveStringExpr(fields["Help"])
		values, _ := resolveValues(fields["Values"])
		info := labelInfo{help: help, values: values}
		// record by Go var name so a metric resolves the SPECIFIC Label it references.
		// Same var name with differing docs across packages is marked ambiguous — emit
		// no help rather than confidently-wrong help.
		if varName != "" {
			if existing, seen := labelVarInfo[varName]; seen && existing.help != info.help {
				ambiguousLabelVars[varName] = true
			}
			labelVarName[varName] = name
			labelVarInfo[varName] = info
		}
		// scoped-code-base (operatorpkg) Labels go ONLY into that scope, not the global
		// registry, so a reused name (e.g. `reason`) doesn't leak onto unrelated
		// metrics. First entry with help wins.
		registry := labelRegistry
		if scope := labelScopeForFile(file); scope != "" {
			if scopedLabelRegistry[scope] == nil {
				scopedLabelRegistry[scope] = map[string]labelInfo{}
			}
			registry = scopedLabelRegistry[scope]
		}
		if existing, seen := registry[name]; !seen || (existing.help == "" && help != "") {
			registry[name] = info
		}
	})
}

// records []opmetrics.Label vars so a metric referencing one resolves its
// dimensions. Runs after collectLabels so element Labels are resolved.
func collectLabelSlices(packages []*ast.Package) {
	forEachValueSpec(packages, func(_, varName string, value ast.Expr) {
		if varName == "" {
			return
		}
		switch v := value.(type) {
		case *ast.CompositeLit:
			if !isLabelSliceType(v.Type) {
				return
			}
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); !ok || id.Name != "append" {
				return
			}
		default:
			return
		}
		if names, infos, ok := resolveLabelDimensions(value); ok {
			labelSliceSymbols[varName] = labelSlice{names: names, infos: infos}
		}
	})
}

// records single-return helper bodies so a metric whose label arg calls one
// (labelNames()) resolves by inlining. Same-named funcs across packages overwrite (rare).
func collectFuncReturns(packages []*ast.Package) {
	for _, pkg := range packages {
		for _, filePath := range slices.Sorted(maps.Keys(pkg.Files)) {
			for _, decl := range pkg.Files[filePath].Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Body == nil || len(fd.Body.List) != 1 {
					continue
				}
				ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 1 {
					continue
				}
				name := fd.Name.Name
				text := types.ExprString(ret.Results[0])
				if prev, seen := funcReturnText[name]; seen && prev != text {
					ambiguousFuncs[name] = true
					continue
				}
				funcReturnText[name] = text
				funcReturns[name] = ret.Results[0]
			}
		}
	}
}

func funcCallName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// identName returns the identifier name of a (possibly pointer, possibly
// package-qualified) type/name expression, e.g. Label -> "Label",
// metrics.Value -> "Value", *v1.NodeClaim -> "NodeClaim".
func identName(t ast.Expr) string {
	switch v := t.(type) {
	case *ast.StarExpr:
		return identName(v.X)
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

func isLabelType(t ast.Expr) bool { return identName(t) == "Label" }

func isValueSliceType(t ast.Expr) bool {
	at, ok := t.(*ast.ArrayType)
	return ok && identName(at.Elt) == "Value"
}

func namedFields(cl *ast.CompositeLit) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	for _, el := range cl.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				out[key.Name] = kv.Value
			}
		}
	}
	return out
}

// resolves a Value{Name,Help} literal; Name may reference a const, unresolved Name -> not ok.
func valueFromCompositeLit(cl *ast.CompositeLit) (valueInfo, bool) {
	fields := namedFields(cl)
	name, ok := resolveStringExpr(fields["Name"])
	if !ok {
		return valueInfo{}, false
	}
	help, _ := resolveStringExpr(fields["Help"])
	return valueInfo{name: name, help: help}, true
}

// resolves a []Value literal; an unresolvable element is skipped rather than discarding the whole slice.
func valueSliceFromCompositeLit(cl *ast.CompositeLit) ([]valueInfo, bool) {
	if cl.Type != nil && !isValueSliceType(cl.Type) {
		return nil, false
	}
	out := make([]valueInfo, 0, len(cl.Elts))
	for _, el := range cl.Elts {
		if v, ok := resolveValue(el); ok {
			out = append(out, v)
		}
	}
	return out, true
}

func resolveValue(expr ast.Expr) (valueInfo, bool) {
	if cl, ok := expr.(*ast.CompositeLit); ok {
		return valueFromCompositeLit(cl)
	}
	if v, ok := valueSymbols[identName(expr)]; ok {
		return v, true
	}
	return valueInfo{}, false
}

func resolveValues(expr ast.Expr) ([]valueInfo, bool) {
	switch v := expr.(type) {
	case *ast.CompositeLit:
		return valueSliceFromCompositeLit(v)
	case *ast.Ident:
		if vals, ok := valueSliceSymbols[v.Name]; ok {
			return vals, true
		}
	case *ast.SelectorExpr:
		if vals, ok := valueSliceSymbols[v.Sel.Name]; ok {
			return vals, true
		}
	case *ast.CallExpr:
		// flatten a composed append(base, elem/spread...) into one []Value.
		if id, ok := v.Fun.(*ast.Ident); !ok || id.Name != "append" || len(v.Args) == 0 {
			return nil, false
		}
		out, ok := resolveValues(v.Args[0])
		if !ok {
			return nil, false
		}
		rest := v.Args[1:]
		for i, arg := range rest {
			if v.Ellipsis.IsValid() && i == len(rest)-1 {
				if vals, ok := resolveValues(arg); ok {
					out = append(out, vals...)
				}
				continue
			}
			if val, ok := resolveValue(arg); ok {
				out = append(out, val)
			}
		}
		return out, true
	}
	return nil, false
}

// invokes fn for each package-level const/var (file, name, value). file lets callers
// attribute a declaration to a code base.
func forEachValueSpec(packages []*ast.Package, fn func(file, name string, value ast.Expr)) {
	for _, pkg := range packages {
		// Iterate files in a stable order; pkg.Files is a map, so ranging it directly
		// would make "first entry wins" resolution (and thus the docs) nondeterministic.
		for _, filePath := range slices.Sorted(maps.Keys(pkg.Files)) {
			file := pkg.Files[filePath]
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nm := range vs.Names {
						if i < len(vs.Values) {
							fn(filePath, nm.Name, vs.Values[i])
						}
					}
				}
			}
		}
	}
}

// unquotes via strconv.Unquote — a naive trim would strip backticks inside the
// content (e.g. help text like "... or `expired`").
func stringLiteralValue(expr ast.Expr) (string, bool) {
	bl, ok := expr.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func stringSliceFromCompositeLit(cl *ast.CompositeLit) ([]string, bool) {
	if at, ok := cl.Type.(*ast.ArrayType); ok {
		if id, ok := at.Elt.(*ast.Ident); !ok || id.Name != "string" {
			return nil, false
		}
	} else if cl.Type != nil {
		return nil, false
	}
	out := make([]string, 0, len(cl.Elts))
	for _, el := range cl.Elts {
		s, ok := resolveStringExpr(el)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// labelSlice is a resolved []opmetrics.Label: the ordered dimension names plus
// per-dimension documentation.
type labelSlice struct {
	names []string
	infos map[string]labelInfo
}

// resolves a metric's label arg into ordered dimension names and (for a []Label literal) per-dimension docs.
func resolveLabelDimensions(expr ast.Expr) ([]string, map[string]labelInfo, bool) {
	if names, ok := resolveLabels(expr); ok {
		return names, nil, true
	}
	switch v := expr.(type) {
	case *ast.CompositeLit:
		if isLabelSliceType(v.Type) {
			ls := labelSliceFromElts(v.Elts)
			// An explicitly empty []Label{} is a resolved "no dimensions" — only treat
			// it as unresolved if it had elements we couldn't resolve.
			if len(v.Elts) > 0 && len(ls.names) == 0 {
				return nil, nil, false
			}
			return ls.names, ls.infos, true
		}
	case *ast.Ident:
		if ls, ok := labelSliceSymbols[v.Name]; ok {
			return ls.names, ls.infos, true
		}
	case *ast.SelectorExpr:
		if ls, ok := labelSliceSymbols[v.Sel.Name]; ok {
			return ls.names, ls.infos, true
		}
	case *ast.CallExpr:
		// a call to a local single-return helper (labelNames(), nodeLabelNames()):
		// resolve by inlining its returned expression.
		if id, ok := v.Fun.(*ast.Ident); !ok || id.Name != "append" || len(v.Args) == 0 {
			if name := funcCallName(v.Fun); name != "" && !resolvingFuncs[name] && !ambiguousFuncs[name] {
				if ret, ok := funcReturns[name]; ok {
					resolvingFuncs[name] = true
					n, i, ok := resolveLabelDimensions(ret)
					delete(resolvingFuncs, name)
					return n, i, ok
				}
			}
			return nil, nil, false
		}
		// flatten append(base, elems/spread...); unresolvable pieces are skipped so known dimensions still document.
		merged := labelSlice{infos: map[string]labelInfo{}}
		add := func(names []string, infos map[string]labelInfo) {
			for _, n := range names {
				merged.names = append(merged.names, n)
				if infos != nil {
					merged.infos[n] = infos[n]
				}
			}
		}
		if names, infos, ok := resolveLabelDimensions(v.Args[0]); ok {
			add(names, infos)
		}
		rest := v.Args[1:]
		for i, arg := range rest {
			if v.Ellipsis.IsValid() && i == len(rest)-1 {
				if names, infos, ok := resolveLabelDimensions(arg); ok {
					add(names, infos)
				}
				continue
			}
			if name, info, ok := labelInfoFromExpr(arg); ok {
				merged.names = append(merged.names, name)
				merged.infos[name] = info
			}
		}
		if len(merged.names) == 0 {
			return nil, nil, false
		}
		return merged.names, merged.infos, true
	}
	return nil, nil, false
}

// resolves []opmetrics.Label elements; an unresolvable element is skipped, not the whole slice.
func labelSliceFromElts(elts []ast.Expr) labelSlice {
	ls := labelSlice{infos: map[string]labelInfo{}}
	for _, el := range elts {
		name, info, ok := labelInfoFromExpr(el)
		if !ok {
			continue
		}
		ls.names = append(ls.names, name)
		ls.infos[name] = info
	}
	return ls
}

func isLabelSliceType(t ast.Expr) bool {
	at, ok := t.(*ast.ArrayType)
	return ok && identName(at.Elt) == "Label"
}

func labelInfoFromExpr(el ast.Expr) (string, labelInfo, bool) {
	switch v := el.(type) {
	case *ast.CompositeLit:
		fields := namedFields(v)
		name, ok := resolveStringExpr(fields["Name"])
		if !ok {
			return "", labelInfo{}, false
		}
		help, _ := resolveStringExpr(fields["Help"])
		values, _ := resolveValues(fields["Values"])
		return name, labelInfo{help: help, values: values}, true
	case *ast.Ident:
		if name, ok := labelVarName[v.Name]; ok {
			if ambiguousLabelVars[v.Name] {
				return name, labelInfo{}, true
			}
			return name, labelVarInfo[v.Name], true
		}
	case *ast.SelectorExpr:
		if name, ok := labelVarName[v.Sel.Name]; ok {
			if ambiguousLabelVars[v.Sel.Name] {
				return name, labelInfo{}, true
			}
			return name, labelVarInfo[v.Sel.Name], true
		}
	}
	return "", labelInfo{}, false
}

func resolveLabels(expr ast.Expr) ([]string, bool) {
	switch v := expr.(type) {
	case *ast.CompositeLit:
		return stringSliceFromCompositeLit(v)
	case *ast.Ident:
		if ambiguousSlices[v.Name] {
			return nil, false
		}
		if s, ok := sliceSymbols[v.Name]; ok {
			return s, true
		}
	case *ast.SelectorExpr:
		if ambiguousSlices[v.Sel.Name] {
			return nil, false
		}
		if s, ok := sliceSymbols[v.Sel.Name]; ok {
			return s, true
		}
	}
	return nil, false
}

// knownExternalConsts resolves string values whose Name is a const from a package the
// generator does not parse (k8s.io/api, k8s.io/apimachinery), keyed by the const's
// identifier name. Without these, values declared as e.g. string(metav1.ConditionTrue)
// or string(corev1.PodPending) silently drop out of the docs. See resolveStringExpr.
var knownExternalConsts = map[string]string{
	"ConditionTrue":    "True",
	"ConditionFalse":   "False",
	"ConditionUnknown": "Unknown",
	"PodPending":       "Pending",
	"PodRunning":       "Running",
	"PodSucceeded":     "Succeeded",
	"PodFailed":        "Failed",
	"PodUnknown":       "Unknown",
}

func resolveStringExpr(expr ast.Expr) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return stringLiteralValue(v)
	case *ast.Ident:
		if ambiguousStrings[v.Name] {
			return "", false
		}
		if s, ok := stringSymbols[v.Name]; ok {
			return s, true
		}
		if s, ok := knownExternalConsts[v.Name]; ok {
			return s, true
		}
	case *ast.SelectorExpr:
		if ambiguousStrings[v.Sel.Name] {
			return "", false
		}
		if s, ok := stringSymbols[v.Sel.Name]; ok {
			return s, true
		}
		if s, ok := knownExternalConsts[v.Sel.Name]; ok {
			return s, true
		}
	case *ast.CallExpr:
		// Unwrap string(X) conversions, used when a Label value references a
		// string-based named type such as disruption.Decision or v1.ConsolidationPolicy.
		if fn, ok := v.Fun.(*ast.Ident); ok && fn.Name == "string" && len(v.Args) == 1 {
			return resolveStringExpr(v.Args[0])
		}
		// unwrap casing helpers so the documented value matches the runtime string
		// (e.g. strings.ToLower, pretty.ToSnakeCase).
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok && len(v.Args) == 1 {
			if pkg, ok := sel.X.(*ast.Ident); ok {
				// strconv.FormatBool(true/false) -> the literal text, for metrics.BoolValues.
				if pkg.Name == "strconv" && sel.Sel.Name == "FormatBool" {
					if id, ok := v.Args[0].(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
						return id.Name, true
					}
				}
				if inner, ok := resolveStringExpr(v.Args[0]); ok {
					switch pkg.Name + "." + sel.Sel.Name {
					case "strings.ToLower":
						return strings.ToLower(inner), true
					case "strings.ToUpper":
						return strings.ToUpper(inner), true
					case "pretty.ToSnakeCase":
						return pretty.ToSnakeCase(inner), true
					}
				}
			}
		}
	case *ast.BinaryExpr:
		// Resolve string concatenation ("a" + "b" + const), used to wrap long
		// Label.Help text across multiple source lines.
		if v.Op == token.ADD {
			if l, ok := resolveStringExpr(v.X); ok {
				if r, ok := resolveStringExpr(v.Y); ok {
					return l + r, true
				}
			}
		}
	}
	return "", false
}

func getFuncPackage(fun ast.Expr) string {
	if pexpr, ok := fun.(*ast.ParenExpr); ok {
		return getFuncPackage(pexpr.X)
	}
	if sexpr, ok := fun.(*ast.StarExpr); ok {
		return getFuncPackage(sexpr.X)
	}
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		return fmt.Sprintf("%s", sel.X)
	}
	if ident, ok := fun.(*ast.Ident); ok {
		return ident.String()
	}
	if iexpr, ok := fun.(*ast.IndexExpr); ok {
		return getFuncPackage(iexpr.X)
	}
	return ""
}

// identMappingKey returns the curated-mapping lookup key for a metric-opts value:
// "pkg.Name" for a selector, "Name" for an ident, "" otherwise (a literal or
// concatenation, resolved directly instead).
func identMappingKey(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return identName(v.X) + "." + v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

// identMapping resolves identifiers/selectors the generator can't read from a
// declaration, or must override (e.g. pluralized subsystems).
var identMapping = map[string]string{
	"metrics.Namespace": metrics.Namespace,
	"Namespace":         metrics.Namespace,

	"pmetrics.Namespace":         "operator",
	"MetricNamespace":            "operator",
	"MetricSubsystem":            "status_condition",
	"TerminationSubsystem":       "termination",
	"WorkQueueSubsystem":         "workqueue",
	"DepthKey":                   "depth",
	"AddsKey":                    "adds_total",
	"QueueLatencyKey":            "queue_duration_seconds",
	"WorkDurationKey":            "work_duration_seconds",
	"UnfinishedWorkKey":          "unfinished_work_seconds",
	"LongestRunningProcessorKey": "longest_running_processor_seconds",
	"RetriesKey":                 "retries_total",

	"PodSubsystem":                 "pods",
	"metrics.PodSubsystem":         "pods",
	"NodeSubsystem":                "nodes",
	"metrics.NodeSubsystem":        "nodes",
	"NodeClaimSubsystem":           "nodeclaims",
	"metrics.NodeClaimSubsystem":   "nodeclaims",
	"nodePoolSubsystem":            "nodepools",
	"metrics.NodePoolSubsystem":    "nodepools",
	"interruptionSubsystem":        "interruption",
	"voluntaryDisruptionSubsystem": "voluntary_disruption",
	"batcherSubsystem":             "cloudprovider_batcher",
	"cloudProviderSubsystem":       "cloudprovider",
	"stateSubsystem":               "cluster_state",
	"schedulerSubsystem":           "scheduler",
	"nodeClassSubsystem":           "ec2nodeclasses",
}
