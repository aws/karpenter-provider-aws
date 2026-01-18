/*
Copyright 2021 The Kubernetes Authors.

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
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
	"sigs.k8s.io/yaml/kyaml"
)

const (
	fmtYAML  = "yaml"
	fmtKYAML = "kyaml"
)

func main() {
	fs := flag.NewFlagSet("yamlfmt", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s [<yaml-files>...]\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(fs.Output(), "If no files are specified, stdin will be used.\n")
		fs.PrintDefaults()
	}

	diff := fs.Bool("d", false, "diff input files with their formatted versions")
	help := fs.Bool("h", false, "print help and exit")
	format := fs.String("o", "yaml", "output format: may be 'yaml' or 'kyaml'")
	write := fs.Bool("w", false, "write result to input files instead of stdout")
	fs.Parse(os.Args[1:])

	if *help {
		fs.SetOutput(os.Stdout)
		fs.Usage()
		os.Exit(0)
	}

	switch *format {
	case "yaml", "kyaml":
		// OK
	default:
		fmt.Fprintf(os.Stderr, "unknown output format %q, must be one of 'yaml' or 'kyaml'\n", *format)
		os.Exit(1)
	}
	if *diff && *write {
		fmt.Fprintln(os.Stderr, "cannot use -d and -w together")
	}

	files := fs.Args()

	if len(files) == 0 {
		if err := renderYAML("<stdin>", os.Stdin, *format, *diff, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	for i, path := range files {
		// use a func to catch defer'ed Close
		func() {
			// Read the YAML file
			sourceYaml, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
				return
			}
			in := bytes.NewReader(sourceYaml)

			out := os.Stdout
			finalize := func() {}
			if *write {
				// Write to a temp file and rename when done.
				tmp, err := os.CreateTemp(filepath.Dir(path), ".yamlfmt.tmp.")
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					os.Exit(1)
				}
				defer tmp.Close()
				finalize = func() {
					if err := os.Rename(tmp.Name(), path); err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						os.Exit(1)
					}
				}
				out = tmp
			}
			if len(files) > 1 && !*write && !*diff {
				if i > 0 {
					fmt.Fprintln(out, "")
				}
				fmt.Fprintln(out, "# "+path)
			}
			if err := renderYAML(path, in, *format, *diff, out); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			finalize()
		}()
	}
}

func renderYAML(path string, in io.Reader, format string, printDiff bool, out io.Writer) error {
	if format == fmtKYAML {
		ky := &kyaml.Encoder{}

		if printDiff {
			ibuf, err := io.ReadAll(in)
			if err != nil {
				return err
			}
			obuf := bytes.Buffer{}
			if err := ky.FromYAML(bytes.NewReader(ibuf), &obuf); err != nil {
				return err
			}
			d := trivialDiff(path, string(ibuf), obuf.String())
			fmt.Fprint(out, d)
			return nil
		}

		return ky.FromYAML(in, out)
	}

	// else format == fmtYAML

	var decoder *yaml.Decoder
	var encoder *yaml.Encoder
	var finish func()

	if printDiff {
		ibuf, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		obuf := bytes.Buffer{}
		decoder = yaml.NewDecoder(bytes.NewReader(ibuf))
		encoder = yaml.NewEncoder(&obuf)
		finish = func() {
			d := trivialDiff(path, string(ibuf), obuf.String())
			fmt.Fprint(out, d)
		}
	} else {
		decoder = yaml.NewDecoder(in)
		encoder = yaml.NewEncoder(out)
	}
	encoder.SetIndent(2)

	for {
		var node yaml.Node // to retain comments
		if err := decoder.Decode(&node); err != nil {
			if err == io.EOF {
				break // End of input
			}
			return fmt.Errorf("failed to decode input: %w", err)
		}
		setBlockStyle(&node) // In case we read KYAML as input.
		if err := encoder.Encode(&node); err != nil {
			return fmt.Errorf("failed to encode node: %w", err)
		}
	}
	if finish != nil {
		finish()
	}
	return nil
}

func trivialDiff(path, a, b string) string {
	if a == b {
		return ""
	}

	x := strings.Split(strings.TrimSuffix(a, "\n"), "\n")
	y := strings.Split(strings.TrimSuffix(b, "\n"), "\n")
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", path, path))
	buf.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d\n", 1, len(x), 1, len(y)))
	for {
		n := 0
		for ; n < len(x) && n < len(y) && x[n] == y[n]; n++ {
			buf.WriteString(" " + x[n] + "\n")
		}
		x = x[n:]
		y = y[n:]

		nextX, nextY := nextCommon(x, y)
		for i := range nextX {
			buf.WriteString("-" + x[i] + "\n")
		}
		x = x[nextX:]
		for j := range nextY {
			buf.WriteString("+" + y[j] + "\n")
		}
		y = y[nextY:]

		if len(x) == 0 && len(y) == 0 {
			break
		}
	}
	return buf.String()
}

func nextCommon(x, y []string) (int, int) {
	for i := range len(x) {
		for j := range len(y) {
			if x[i] == y[j] {
				return i, j
			}
		}
	}
	return len(x), len(y)
}

func setBlockStyle(node *yaml.Node) {
	node.Style = node.Style & (^yaml.FlowStyle)
	for _, child := range node.Content {
		setBlockStyle(child)
	}
}
