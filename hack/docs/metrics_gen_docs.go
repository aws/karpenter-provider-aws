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
	if flag.NArg() != 2 {
		log.Printf("Usage: %s path/to/metrics/controller path/to/markdown.md", os.Args[0])
		os.Exit(1)
	}
	fset := token.NewFileSet()
	var packages []*ast.Package
	root := flag.Arg(0)

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
	sort.Slice(allMetrics, bySubsystem(allMetrics))

	outputFileName := flag.Arg(1)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("error creating output file %s, %s", outputFileName, err)
	}

	log.Println("writing output to", outputFileName)
	fmt.Fprintf(f, `---
title: "Metrics"
linkTitle: "Metrics"
weight: 100

description: >
  Inspect Karpenter Metrics
---
`)
	fmt.Fprintf(f, "<!-- this document is generated from hack/docs/metrics_gen_docs.go -->\n")
	fmt.Fprintf(f, "Karpenter writes several metrics to Prometheus to allow monitoring cluster provisioning status\n")
	previousSubsystem := ""
	for _, metric := range allMetrics {
		if metric.subsystem != previousSubsystem {
			fmt.Fprintf(f, "## %s%s Metrics\n", strings.ToTitle(metric.subsystem[0:1]), metric.subsystem[1:])
			previousSubsystem = metric.subsystem
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "### `%s`\n", metric.qualifiedName())
		fmt.Fprintf(f, "%s\n", metric.help)
		fmt.Fprintln(f)
	}

}

func bySubsystem(metrics []metricInfo) func(i int, j int) bool {
	subSystemSortOrder := map[string]int{}
	subSystemSortOrder["provisioner"] = 1
	subSystemSortOrder["nodes"] = 2
	subSystemSortOrder["pods"] = 3
	subSystemSortOrder["allocation_controller"] = 4
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
	var metrics []metricInfo
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
			if fmt.Sprintf("%s", ce.Fun.(*ast.SelectorExpr).X) != "prometheus" {
				continue
			}
			if len(ce.Args) != 2 {
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
					if selector := fmt.Sprintf("%s.%s", val.X, val.Sel); selector == "metrics.Namespace" {
						value = "karpenter"
					} else {
						log.Fatalf("unsupported selector %s", selector)
					}
				default:
					log.Fatalf("unsupported value %T %v", kv.Value, kv.Value)
				}
				keyValuePairs[key] = strings.TrimFunc(value, func(r rune) bool {
					return r == '"'
				})
			}
			metrics = append(metrics, metricInfo{
				namespace: keyValuePairs["Namespace"],
				subsystem: keyValuePairs["Subsystem"],
				name:      keyValuePairs["Name"],
				help:      keyValuePairs["Help"],
			})
		}
	}
	return metrics
}
