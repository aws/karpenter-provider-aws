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
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/metrics"
)

type metricInfo struct {
	namespace string
	subsystem string
	name      string
	help      string
}

var (
	stableMetrics = []string{"controller_runtime", "aws_sdk_go", "client_go", "leader_election", "interruption", "cluster_state", "workqueue", "karpenter_build_info", "karpenter_nodepool_usage", "karpenter_nodepool_limit",
		"karpenter_nodeclaims_terminated_total", "karpenter_nodeclaims_created_total", "karpenter_nodes_terminated_total", "karpenter_nodes_created_total", "karpenter_pods_startup_duration_seconds",
		"karpenter_scheduler_scheduling_duration_seconds", "karpenter_provisioner_scheduling_duration_seconds", "karpenter_nodepool_allowed_disruptions", "karpenter_voluntary_disruption_decisions_total"}
	betaMetrics = []string{"cloudprovider", "cloudprovider_batcher", "karpenter_nodeclaims_termination_duration_seconds", "karpenter_nodeclaims_instance_termination_duration_seconds",
		"karpenter_nodes_total_pod_requests", "karpenter_nodes_total_pod_limits", "karpenter_nodes_total_daemon_requests", "karpenter_nodes_total_daemon_limits", "karpenter_nodes_termination_duration_seconds",
		"karpenter_nodes_system_overhead", "karpenter_nodes_allocatable", "karpenter_pods_state", "karpenter_scheduler_queue_depth", "karpenter_voluntary_disruption_queue_failures_total",
		"karpenter_voluntary_disruption_decision_evaluation_duration_seconds", "karpenter_voluntary_disruption_eligible_nodes", "karpenter_voluntary_disruption_consolidation_timeouts_total",
		// Per-object status condition and termination metrics from operatorpkg
		"nodeclaim_status_condition", "nodeclaim_termination",
		"node_status_condition", "node_termination",
		"nodepool_status_condition", "nodepool_termination",
		"ec2nodeclass_status_condition", "ec2nodeclass_termination"}
	// Deprecated generic status condition and termination metrics (without object name prefix).
	// These are still emitted at runtime but are superseded by per-object variants.
	deprecatedMetrics = []string{"status_condition", "termination"}
)

func (i metricInfo) qualifiedName() string {
	return strings.Join(lo.Compact([]string{i.namespace, i.subsystem, i.name}), "_")
}

// metrics_gen_docs is used to parse the source code for Prometheus metrics and automatically generate markdown documentation
// based on the naming and help provided in the source code.

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatalf("Usage: %s path/to/metrics/controller path/to/metrics/controller2 path/to/markdown.md", os.Args[0])
	}
	var allMetrics []metricInfo
	for i := 0; i < flag.NArg()-1; i++ {
		packages := getPackages(flag.Arg(i))
		allMetrics = append(allMetrics, getMetricsFromPackages(packages...)...)
	}

	// The operatorpkg status and events controllers dynamically create per-object metrics
	// at runtime based on the Go type parameter passed to status.NewController[T]().
	// These cannot be extracted via AST parsing, so we generate them from a known list of
	// object types that have status controllers registered.
	allMetrics = append(allMetrics, perObjectStatusMetrics()...)

	// Dedupe metrics
	allMetrics = lo.UniqBy(allMetrics, func(m metricInfo) string {
		return fmt.Sprintf("%s/%s/%s", m.namespace, m.subsystem, m.name)
	})

	// Drop some metrics
	for _, subsystem := range []string{"rest_client", "certwatcher_read", "controller_runtime_webhook"} {
		allMetrics = lo.Reject(allMetrics, func(m metricInfo, _ int) bool {
			return strings.HasPrefix(m.name, subsystem)
		})
	}

	// Controller Runtime and AWS SDK Go for Prometheus naming is different in that they don't specify a namespace or subsystem
	// Getting the metrics requires special parsing logic
	for _, subsystem := range []string{"controller_runtime", "aws_sdk_go", "client_go", "leader_election"} {
		for i := range allMetrics {
			if allMetrics[i].subsystem == "" && strings.HasPrefix(allMetrics[i].name, fmt.Sprintf("%s_", subsystem)) {
				allMetrics[i].subsystem = subsystem
				allMetrics[i].name = strings.TrimPrefix(allMetrics[i].name, fmt.Sprintf("%s_", subsystem))
			}
		}
	}
	sort.Slice(allMetrics, bySubsystem(allMetrics))

	outputFileName := flag.Arg(flag.NArg() - 1)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("error creating output file %s, %s", outputFileName, err)
	}

	log.Println("writing output to", outputFileName)
	fmt.Fprintf(f, `---
title: "Metrics"
linkTitle: "Metrics"
weight: 7

description: >
  Inspect Karpenter Metrics
---
`)
	fmt.Fprintf(f, "<!-- this document is generated from hack/docs/metrics_gen/main.go -->\n")
	fmt.Fprintf(f, "Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. "+
		"These metrics are available by default at `karpenter.kube-system.svc.cluster.local:8080/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../settings)\n")
	previousSubsystem := ""

	for _, metric := range allMetrics {
		if metric.subsystem != previousSubsystem {
			if metric.subsystem != "" {
				subsystemTitle := strings.Join(lo.Map(strings.Split(metric.subsystem, "_"), func(s string, _ int) string {
					if s == "sdk" || s == "aws" {
						return strings.ToUpper(s)
					} else {
						return fmt.Sprintf("%s%s", strings.ToUpper(s[0:1]), s[1:])
					}
				}), " ")
				fmt.Fprintf(f, "## %s Metrics\n", subsystemTitle)
				fmt.Fprintln(f)
			}
			previousSubsystem = metric.subsystem
		}
		fmt.Fprintf(f, "### `%s`\n", metric.qualifiedName())
		fmt.Fprintf(f, "%s\n", metric.help)
		switch {
		case slices.Contains(deprecatedMetrics, metric.subsystem) || slices.Contains(deprecatedMetrics, metric.qualifiedName()):
			fmt.Fprintf(f, "- Stability Level: %s\n", "DEPRECATED")
		case slices.Contains(stableMetrics, metric.subsystem) || slices.Contains(stableMetrics, metric.qualifiedName()):
			fmt.Fprintf(f, "- Stability Level: %s\n", "STABLE")
		case slices.Contains(betaMetrics, metric.subsystem) || slices.Contains(betaMetrics, metric.qualifiedName()):
			fmt.Fprintf(f, "- Stability Level: %s\n", "BETA")
		default:
			fmt.Fprintf(f, "- Stability Level: %s\n", "ALPHA")
		}
		fmt.Fprintln(f)
	}

}

func getPackages(root string) []*ast.Package {
	var packages []*ast.Package
	fset := token.NewFileSet()

	// walk our metrics controller directory
	log.Println("parsing code in", root)
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// parse the packagers that we find
		pkgs, err := parser.ParseDir(fset, path, func(info fs.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, parser.AllErrors)
		if err != nil {
			log.Fatalf("error parsing, %s", err)
		}
		for _, pkg := range pkgs {
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
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
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
		"nodepool":                      10,
		"nodeclaims":                    9,
		"nodeclaim_status_condition":    8,
		"nodeclaim_termination":         8,
		"nodes":                         7,
		"node_status_condition":         6,
		"node_termination":              6,
		"pods":                          5,
		"nodepool_status_condition":     4,
		"nodepool_termination":          4,
		"ec2nodeclass_status_condition": 3,
		"ec2nodeclass_termination":      3,
		"status_condition":              -1,
		"termination":                  -1,
		"workqueue":                    -1,
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

// perObjectStatusMetrics generates metrics for the operatorpkg status and events controllers.
// These metrics are dynamically created at runtime based on the Go type parameter passed to
// status.NewController[T]() and cannot be extracted via AST parsing. The object types are
// determined by the status controller registrations in karpenter and karpenter-provider-aws.
func perObjectStatusMetrics() []metricInfo {
	// Object types that have status controllers registered via status.NewController[T]()
	// in karpenter (nodeclaim, nodepool, node) and karpenter-provider-aws (ec2nodeclass).
	objectNames := []string{"nodeclaim", "nodepool", "node", "ec2nodeclass"}

	type metricTemplate struct {
		subsystemSuffix string
		name            string
		help            string
	}

	templates := []metricTemplate{
		{"status_condition", "transition_seconds", "The amount of time a condition was in a given state before transitioning. e.g. Alarm := P99(Updated=False) > 5 minutes"},
		{"status_condition", "count", "The number of a condition for a given object, type and status. e.g. Alarm := Available=False > 0"},
		{"status_condition", "current_status_seconds", "The current amount of time in seconds that a status condition has been in a specific state. Alarm := P99(Updated=Unknown) > 5 minutes"},
		{"status_condition", "transitions_total", "The count of transitions of a given object, type and status."},
		{"termination", "current_time_seconds", "The current amount of time in seconds that an object has been in terminating state."},
		{"termination", "duration_seconds", "The amount of time taken by an object to terminate completely."},
	}

	var metricsOut []metricInfo
	for _, obj := range objectNames {
		for _, t := range templates {
			metricsOut = append(metricsOut, metricInfo{
				namespace: "operator",
				subsystem: fmt.Sprintf("%s_%s", obj, t.subsystemSuffix),
				name:      t.name,
				help:      t.help,
			})
		}
	}

	// Deprecated generic metrics (without object name prefix) are still emitted at runtime
	// when emitDeprecatedMetrics is enabled on the status controller. These use group/kind
	// labels instead of baking the object name into the subsystem.
	for _, t := range templates {
		metricsOut = append(metricsOut, metricInfo{
			namespace: "operator",
			subsystem: t.subsystemSuffix,
			name:      t.name,
			help:      t.help,
		})
	}
	return metricsOut
}

// metricFromCallExpr attempts to extract metric info from a call expression.
// It recognizes prometheus.New*(), opmetrics.NewPrometheus*(), and pmetrics.NewPrometheus*() calls.
func metricFromCallExpr(ce *ast.CallExpr) (metricInfo, bool) {
	funcPkg := getFuncPackage(ce.Fun)
	funcName := getFuncName(ce.Fun)
	// Determine the index of the opts argument based on the package.
	// prometheus.New*() calls pass opts as Args[0], while
	// opmetrics/pmetrics.NewPrometheus*() calls from operatorpkg pass
	// (registry, opts, labelNames), so opts is Args[1].
	// Calls within the operatorpkg metrics package itself use unqualified
	// function names like NewPrometheusCounter directly.
	var optsIdx int
	switch {
	case funcPkg == "prometheus":
		optsIdx = 0
	case funcPkg == "opmetrics" || funcPkg == "pmetrics":
		optsIdx = 1
	case strings.HasPrefix(funcName, "NewPrometheus"):
		// Unqualified call within the operatorpkg metrics package itself
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
		key := fmt.Sprintf("%s", kv.Key)
		switch key {
		case "Namespace", "Subsystem", "Name", "Help":
		default:
			// skip any keys we don't care about
			continue
		}
		value := ""
		switch val := kv.Value.(type) {
		case *ast.BasicLit:
			value = val.Value
		case *ast.SelectorExpr:
			selector := fmt.Sprintf("%s.%s", val.X, val.Sel)
			v, err := getIdentMapping(selector)
			if err != nil {
				// Unresolvable selector (e.g. local variable reference inside a helper function).
				// Skip this metric rather than fataling since ast.Inspect walks all function bodies.
				return metricInfo{}, false
			}
			value = v
		case *ast.Ident:
			v, err := getIdentMapping(val.String())
			if err != nil {
				// Unresolvable identifier (e.g. function parameter or local variable).
				// Skip this metric.
				return metricInfo{}, false
			}
			value = v
		case *ast.BinaryExpr:
			value = getBinaryExpr(val)
		default:
			// Unknown value expression type; skip this metric.
			return metricInfo{}, false
		}
		keyValuePairs[key] = strings.TrimFunc(value, func(r rune) bool {
			return r == '"'
		})
	}
	return metricInfo{
		namespace: keyValuePairs["Namespace"],
		subsystem: keyValuePairs["Subsystem"],
		name:      keyValuePairs["Name"],
		help:      keyValuePairs["Help"],
	}, true
}

func getFuncName(fun ast.Expr) string {
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	if ident, ok := fun.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
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
	if _, ok := fun.(*ast.FuncLit); ok {
		return ""
	}
	return ""
}

func getBinaryExpr(b *ast.BinaryExpr) string {
	var x, y string
	switch val := b.X.(type) {
	case *ast.BasicLit:
		x = strings.Trim(val.Value, `"`)
	case *ast.BinaryExpr:
		x = getBinaryExpr(val)
	default:
		log.Fatalf("unsupported value %T %v", val, val)
	}
	switch val := b.Y.(type) {
	case *ast.BasicLit:
		y = strings.Trim(val.Value, `"`)
	case *ast.BinaryExpr:
		y = getBinaryExpr(val)
	default:
		log.Fatalf("unsupported value %T %v", val, val)
	}
	return x + y
}

// we cannot get the value of an Identifier directly so we map it manually instead
func getIdentMapping(identName string) (string, error) {
	identMapping := map[string]string{
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

		"metrics.PodSubsystem":       "pods",
		"NodeSubsystem":              "nodes",
		"metrics.NodeSubsystem":      "nodes",
		"machineSubsystem":           "machines",
		"NodeClaimSubsystem":         "nodeclaims",
		"metrics.NodeClaimSubsystem": "nodeclaims",
		// TODO @joinnis: We should eventually change this subsystem to be
		// plural so that it aligns with the other subsystems
		"nodePoolSubsystem":            "nodepools",
		"metrics.NodePoolSubsystem":    "nodepools",
		"interruptionSubsystem":        "interruption",
		"deprovisioningSubsystem":      "deprovisioning",
		"voluntaryDisruptionSubsystem": "voluntary_disruption",
		"batcherSubsystem":             "cloudprovider_batcher",
		"cloudProviderSubsystem":       "cloudprovider",
		"stateSubsystem":               "cluster_state",
		"schedulerSubsystem":           "scheduler",
	}
	if v, ok := identMapping[identName]; ok {
		return v, nil
	}
	return "", serrors.Wrap(fmt.Errorf("no identifier mapping exists"), "identifier", identName)
}
