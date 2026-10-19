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
	"strings"

	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s karpenter version", os.Args[0])
	}
	if os.Args[2] == "no tag" {
		log.Printf("No version")
		os.Exit(0)
	}

	v := strings.TrimSuffix(strings.TrimPrefix(os.Args[2], "v"), ".0")
	appendVersion := fmt.Sprintf(
		`
  - appVersion: %s.x
    minK8sVersion: %s
    maxK8sVersion: %s`,
		v,
		version.MinK8sVersion,
		version.MaxK8sVersion)

	yamlFile, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("Can't read %s file: %v", os.Args[1], err)
		os.Exit(1)
	}

	log.Println("writing output to", os.Args[1])
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatalf("unable to open %s to write generated output: %v", os.Args[1], err)
	}
	f.WriteString(string(yamlFile) + appendVersion)
}
