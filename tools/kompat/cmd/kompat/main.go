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
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/aws/karpenter-provider-aws/tools/kompat/pkg/kompat"
)

const (
	OutputJSON     = "json"
	OutputYAML     = "yaml"
	OutputTable    = "table"
	OutputMarkdown = "md"
)

var (
	version = ""
)

type GlobalOptions struct {
	Verbose bool
	Version bool
	Output  string
}

type RootOptions struct {
	LastNVersions int
	K8sVersion    string
	Branch        string
}

var (
	globalOpts = GlobalOptions{}
	rootOpts   = RootOptions{}
	rootCmd    = &cobra.Command{
		Use:     "kompat",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if rootOpts.Branch != "" {
				kompat.DefaultGithubBranch = rootOpts.Branch
			}
			kompatList, err := kompat.Parse(args...)
			if err != nil {
				fmt.Printf("Unable to parse kompat file: %v\n", err)
				os.Exit(1)
			}
			opts := kompat.Options{
				LastN:   rootOpts.LastNVersions,
				Version: rootOpts.K8sVersion,
			}
			switch globalOpts.Output {
			case OutputJSON:
				fmt.Println(kompatList.JSON())
				os.Exit(0)
			case OutputYAML:
				fmt.Println(kompatList.YAML())
				os.Exit(0)
			case OutputTable:
			case OutputMarkdown:
				fmt.Println(kompatList.Markdown(opts))
				os.Exit(0)
			}
		},
	}
)

func main() {
	rootCmd.PersistentFlags().BoolVar(&globalOpts.Verbose, "verbose", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&globalOpts.Version, "version", false, "version")
	rootCmd.PersistentFlags().StringVarP(&globalOpts.Output, "output", "o", OutputMarkdown,
		fmt.Sprintf("Output mode: %v", []string{OutputTable, OutputJSON, OutputYAML, OutputMarkdown}))

	rootCmd.AddCommand(&cobra.Command{Use: "completion", Hidden: true})
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().IntVarP(&rootOpts.LastNVersions, "last-n-versions", "n", 4, "Last n K8s versions")
	rootCmd.PersistentFlags().StringVar(&rootOpts.K8sVersion, "k8s-version", "", "search for compatibility with a specific k8s version")
	rootCmd.PersistentFlags().StringVarP(&rootOpts.Branch, "branch", "b", "main", "default github branch for remote lookups")

	lo.Must0(rootCmd.Execute())
}

func PrettyEncode(data any) string {
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	enc.SetIndent("", "    ")
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
	return buffer.String()
}

func PrettyTable[T any](data []T, wide bool) string {
	var headers []string
	var rows [][]string
	for _, dataRow := range data {
		var row []string
		// clear headers each time so we only keep one set
		headers = []string{}
		reflectStruct := reflect.Indirect(reflect.ValueOf(dataRow))
		for i := 0; i < reflectStruct.NumField(); i++ {
			typeField := reflectStruct.Type().Field(i)
			tag := typeField.Tag.Get("table")
			if tag == "" {
				continue
			}
			subtags := strings.Split(tag, ",")
			if len(subtags) > 1 && subtags[1] == "wide" && !wide {
				continue
			}
			headers = append(headers, subtags[0])
			row = append(row, reflect.ValueOf(dataRow).Field(i).String())
		}
		rows = append(rows, row)
	}
	out := bytes.Buffer{}
	table := tablewriter.NewWriter(&out)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(rows) // Add Bulk Data
	table.Render()
	return out.String()
}
