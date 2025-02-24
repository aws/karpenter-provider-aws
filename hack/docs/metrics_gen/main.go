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
	betaMetrics = []string{"status_condition", "cloudprovider", "cloudprovider_batcher", "karpenter_nodeclaims_termination_duration_seconds", "karpenter_nodeclaims_instance_termination_duration_seconds",
		"karpenter_nodes_total_pod_requests", "karpenter_nodes_total_pod_limits", "karpenter_nodes_total_daemon_requests", "karpenter_nodes_total_daemon_limits", "karpenter_nodes_termination_duration_seconds",
		"karpenter_nodes_system_overhead", "karpenter_nodes_allocatable", "karpenter_pods_state", "karpenter_scheduler_queue_depth", "karpenter_voluntary_disruption_queue_failures_total",
		"karpenter_voluntary_disruption_decision_evaluation_duration_seconds", "karpenter_voluntary_disruption_eligible_nodes", "karpenter_voluntary_disruption_consolidation_timeouts_total"}
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

	// Dedupe metrics
	allMetrics = lo.UniqBy(allMetrics, func(m metricInfo) string {
		return fmt.Sprintf("%s/%s/%s", m.namespace, m.subsystem, m.name)
	})

	// Remover métricas DEPRECATED antes de gerar a documentação
	allMetrics = lo.Reject(allMetrics, func(m metricInfo, _ int) bool {
		return strings.Contains(strings.ToLower(m.help), "deprecated")
	})

	// Código para gerar a documentação continua normalmente...

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
	fmt.Fprintf(f, "<!-- this document is generated from hack/docs/metrics_gen_docs.go -->\n")
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
			return true
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
	// metrics are all package global variables
	var allMetrics []metricInfo
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch v := decl.(type) {
				case *ast.FuncDecl:
				// ignore
				case *ast.GenDecl:
					if v.Tok == token.VAR {
						allMetrics = append(allMetrics, handleVariableDeclaration(v)...)
					}
				default:

				}
			}
		}
	}
	return allMetrics
}

func bySubsystem(metrics []metricInfo) func(i int, j int) bool {
	// Higher ordering comes first. If a value isn't designated here then the subsystem will be given a default of 0.
	// Metrics without a subsystem come first since there is no designation for the bucket they fall under
	subSystemSortOrder := map[string]int{
		"":                 100,
		"nodepool":         10,
		"nodeclaims":       9,
		"nodes":            8,
		"pods":             7,
		"status_condition": -1,
		"workqueue":        -1,
		"client_go":        -1,
		"aws_sdk_go":       -1,
		"leader_election":  -2,
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

func handleVariableDeclaration(v *ast.GenDecl) []metricInfo {
	var promMetrics []metricInfo
	for _, spec := range v.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, v := range vs.Values {
			ce, ok := v.(*ast.CallExpr)
			if !ok {
				continue
			}
			funcPkg := getFuncPackage(ce.Fun)
			if funcPkg != "prometheus" {
				continue
			}
			if len(ce.Args) == 0 {
				continue
			}
			arg := ce.Args[0].(*ast.CompositeLit)
			keyValuePairs := map[string]string{}
			for _, el := range arg.Elts {
				kv := el.(*ast.KeyValueExpr)
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
					if v, err := getIdentMapping(selector); err != nil {
						log.Fatalf("unsupported selector %s, %s", selector, err)
					} else {
						value = v
					}
				case *ast.Ident:
					if v, err := getIdentMapping(val.String()); err != nil {
						log.Fatal(err)
					} else {
						value = v
					}
				case *ast.BinaryExpr:
					value = getBinaryExpr(val)
				default:
					log.Fatalf("unsupported value %T %v", kv.Value, kv.Value)
				}
				keyValuePairs[key] = strings.TrimFunc(value, func(r rune) bool {
					return r == '"'
				})
			}
			promMetrics = append(promMetrics, metricInfo{
				namespace: keyValuePairs["Namespace"],
				subsystem: keyValuePairs["Subsystem"],
				name:      keyValuePairs["Name"],
				help:      keyValuePairs["Help"],
			})
		}
	}
	return promMetrics
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
	log.Fatalf("unsupported func expression %T, %v", fun, fun)
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
	return "", fmt.Errorf("no identifier mapping exists for %s", identName)
}
