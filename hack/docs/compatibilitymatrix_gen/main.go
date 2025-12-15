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
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aws/karpenter-provider-aws/tools/kompat/pkg/kompat"
)

func main() {
	outputFileName := os.Args[1]
	mdFile, err := os.ReadFile(outputFileName)
	if err != nil {
		log.Printf("Can't read %s file: %v", os.Args[1], err)
		os.Exit(1)
	}

	genStart := "[comment]: <> (the content below is generated from hack/docs/compatibilitymatrix_gen/main.go)"
	genEnd := "[comment]: <> (end docs generated content from hack/docs/compatibilitymatrix_gen/main.go)"
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

	baseText, err := kompat.Parse(os.Args[2])
	if err != nil {
		log.Fatalf("unable to generate compatibility matrix")
	}

	log.Println("writing output to", outputFileName)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("unable to open %s to write generated output: %v", outputFileName, err)
	}
	numOfk8sVersion, _ := strconv.Atoi(os.Args[3])
	f.WriteString(topDoc + baseText.Markdown(kompat.Options{LastN: numOfk8sVersion}) + bottomDoc)
}
