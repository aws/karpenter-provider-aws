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
	"time"

	"github.com/samber/lo"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
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

	genStart := "[comment]: <> (the content below is generated from hack/docs/repairpolicies_gen/main.go)"
	genEnd := "[comment]: <> (end docs generated content from hack/docs/repairpolicies_gen/main.go)"
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

	// RepairPolicies doesn't read any CloudProvider state, so a zero-value CloudProvider is enough.
	policies := (&cloudprovider.CloudProvider{}).RepairPolicies()
	// Priority only affects ordering, so the column is only worth showing once policies disagree on it.
	showPriority := len(lo.UniqBy(policies, func(p corecloudprovider.RepairPolicy) int { return p.Priority })) > 1

	block := "| Condition Type | Status | Toleration Duration | Termination Grace Period |"
	sep := "|---|---|---|---|"
	if showPriority {
		block += " Priority |"
		sep += "---|"
	}
	block += "\n" + sep + "\n"
	for _, p := range policies {
		block += fmt.Sprintf("| `%s` | `%s` | %s | %s |", p.ConditionType, p.ConditionStatus, formatDuration(p.TolerationDuration), formatTerminationGracePeriod(p.TerminationGracePeriod))
		if showPriority {
			block += fmt.Sprintf(" %d |", p.Priority)
		}
		block += "\n"
	}

	log.Println("writing output to", outputFileName)
	f, err := os.Create(outputFileName)
	if err != nil {
		log.Fatalf("unable to open %s to write generated output: %v", outputFileName, err)
	}
	f.WriteString(topDoc + block + bottomDoc)
}

func formatTerminationGracePeriod(tgp *time.Duration) string {
	switch {
	case tgp == nil:
		return "NodeClaim's `terminationGracePeriod`"
	case *tgp == 0:
		return "0 (forceful)"
	default:
		return formatDuration(*tgp)
	}
}

// formatDuration renders whole minutes as "30 minutes" to match the rest of the docs, falling back to Go's format.
func formatDuration(d time.Duration) string {
	if d%time.Minute != 0 {
		return d.String()
	}
	m := int(d.Minutes())
	return fmt.Sprintf("%d %s", m, lo.Ternary(m == 1, "minute", "minutes"))
}
