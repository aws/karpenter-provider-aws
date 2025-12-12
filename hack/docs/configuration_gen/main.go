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
	"log"
	"os"
	"strings"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s path/to/markdown.md", os.Args[0])
	}
	outputFileName := os.Args[1]
	mdFile, err := os.ReadFile(outputFileName)
	if err != nil {
		log.Printf("Can't read %s file: %v", os.Args[1], err)
		os.Exit(2)
	}

	genStart := "[comment]: <> (the content below is generated from hack/docs/configuration_gen/main.go)"
	genEnd := "[comment]: <> (end docs generated content from hack/docs/configuration_gen/main.go)"
	startDocSections := strings.Split(string(mdFile), genStart)
	if len(startDocSections) != 2 {
		log.Fatalf("expected one generated comment block start but got %d", len(startDocSections)-1)
	}
	endDocSections := strings.Split(string(mdFile), genEnd)
	if len(endDocSections) != 2 {
		log.Fatalf("expected one generated comment block end but got %d", len(endDocSections)-1)
	}
	topDoc := fmt.Sprintf("%s%s\n\n", startDocSections[0], genStart)
	bottomDoc := fmt.Sprintf("\n%s%s", genEnd, endDocSections[1])

	fs := &coreoptions.FlagSet{
		FlagSet: flag.NewFlagSet("karpenter", flag.ContinueOnError),
	}
	(&coreoptions.Options{}).AddFlags(fs)
	(&options.Options{}).AddFlags(fs)

	envVarsBlock := "| Environment Variable | CLI Flag | Description |\n"
	envVarsBlock += "|--|--|--|\n"
	fs.VisitAll(func(f *flag.Flag) {
		if f.DefValue == "" {
			envVarsBlock += fmt.Sprintf("| %s | %s | %s|\n", strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_"), "\\-\\-"+f.Name, f.Usage)
		} else {
			envVarsBlock += fmt.Sprintf("| %s | %s | %s (default = %s)|\n", strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_"), "\\-\\-"+f.Name, f.Usage, f.DefValue)
		}
	})

	log.Println("writing output to", outputFileName)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("unable to open %s to write generated output: %v", outputFileName, err)
	}
	f.WriteString(topDoc + envVarsBlock + bottomDoc)
}
