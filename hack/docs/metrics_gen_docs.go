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
	"sort"
	"strings"

	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/metrics"
)

type metricInfo struct {
	namespace string
	subsystem string
	name      string
	help      string
}

func (i metricInfo) qualifiedName() string {
	return fmt.Sprintf("%s_%s_%s", i.namespace, i.subsystem, i.name)
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
weight: 12

description: >
  Inspect Karpenter Metrics
---
`)
	fmt.Fprintf(f, "<!-- this document is generated from hack/docs/metrics_gen_docs.go -->\n")
	fmt.Fprintf(f, "Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. "+
		"These metrics are available by default at `karpenter.karpenter.svc.cluster.local:8080/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../globalsettings)\n")
	previousSubsystem := ""
	for _, metric := range allMetrics {
		if metric.subsystem != previousSubsystem {
			subsystemTitle := strings.Join(lo.Map(strings.Split(metric.subsystem, "_"), func(s string, _ int) string {
				return fmt.Sprintf("%s%s", strings.ToTitle(s[0:1]), s[1:])
			}), " ")
			fmt.Fprintf(f, "## %s Metrics\n", subsystemTitle)
			previousSubsystem = metric.subsystem
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "### `%s`\n", metric.qualifiedName())
		fmt.Fprintf(f, "%s\n", metric.help)
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
	subSystemSortOrder := map[string]int{}
	subSystemSortOrder["provisioner"] = 1
	subSystemSortOrder["nodes"] = 2
	subSystemSortOrder["pods"] = 3
	subSystemSortOrder["cloudprovider"] = 4
	subSystemSortOrder["allocation_controller"] = 5
	return func(i, j int) bool {
		lhs := metrics[i]
		rhs := metrics[j]
		if subSystemSortOrder[lhs.subsystem] != subSystemSortOrder[rhs.subsystem] {
			return subSystemSortOrder[lhs.subsystem] < subSystemSortOrder[rhs.subsystem]
		}
		return lhs.qualifiedName() < rhs.qualifiedName()
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
	log.Fatalf("unsupported func expression %T, %v", fun, fun)
	return ""
}

// we cannot get the value of an Identifier directly so we map it manually instead
func getIdentMapping(identName string) (string, error) {
	identMapping := map[string]string{
		"metrics.Namespace": metrics.Namespace,
		"Namespace":         metrics.Namespace,

		"nodeSubsystem":           "nodes",
		"interruptionSubsystem":   "interruption",
		"nodeTemplateSubsystem":   "nodetemplate",
		"deprovisioningSubsystem": "deprovisioning",
	}
	if v, ok := identMapping[identName]; ok {
		return v, nil
	}
	return "", fmt.Errorf("no identifier mapping exists for %s", identName)
}
