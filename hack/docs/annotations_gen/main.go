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
	"slices"
	"strings"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/wellknown"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// annotations_gen renders every well known annotation, from core and the AWS provider, into a reference page that
// mirrors https://kubernetes.io/docs/reference/labels-annotations-taints/.
func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s path/to/markdown.md", os.Args[0])
	}
	outputFileName := os.Args[1]
	mdFile, err := os.ReadFile(outputFileName)
	if err != nil {
		log.Fatalf("error reading output file %s, %s", outputFileName, err)
	}

	genStart := "[comment]: <> (the content below is generated from hack/docs/annotations_gen/main.go)"
	genEnd := "[comment]: <> (end docs generated content from hack/docs/annotations_gen/main.go)"
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

	annotations := append(append([]wellknown.Annotation{}, karpv1.KarpenterAnnotations...), v1.AWSAnnotations...)
	if dupes := lo.FindDuplicatesBy(annotations, func(a wellknown.Annotation) string { return a.Name }); len(dupes) > 0 {
		log.Fatalf("annotations documented more than once: %v", lo.Map(dupes, func(a wellknown.Annotation, _ int) string { return a.Name }))
	}
	// User-facing annotations come first, since they're the ones readers act on.
	userFacing, internalOnly := lo.FilterReject(annotations, func(a wellknown.Annotation, _ int) bool { return !a.InternalOnly })
	byName := func(a, b wellknown.Annotation) int { return strings.Compare(a.Name, b.Name) }
	slices.SortFunc(userFacing, byName)
	slices.SortFunc(internalOnly, byName)

	var b strings.Builder
	b.WriteString("## User-Facing Annotations\n\n")
	for _, a := range userFacing {
		writeAnnotation(&b, a)
	}
	b.WriteString("## Internal Annotations\n\n")
	b.WriteString("Karpenter sets these annotations itself. They are documented for visibility only and may change at any time; never set them, never read them.\n\n")
	for _, a := range internalOnly {
		writeAnnotation(&b, a)
	}

	if err := os.WriteFile(outputFileName, []byte(topDoc+strings.TrimRight(b.String(), "\n")+"\n"+bottomDoc), 0644); err != nil {
		log.Fatalf("error writing output file %s, %s", outputFileName, err)
	}
}

func writeAnnotation(b *strings.Builder, a wellknown.Annotation) {
	fmt.Fprintf(b, "### `%s`\n\n", a.Name)
	fmt.Fprintf(b, "Type: Annotation\n\n")
	fmt.Fprintf(b, "Example: `%s: %q`\n\n", a.Name, a.Example)
	fmt.Fprintf(b, "Used on: %s\n\n", strings.Join(lo.Map(a.UsedOn, func(o runtime.Object, _ int) string { return object.GVK(o).Kind }), ", "))
	if a.Stage == "" {
		log.Fatalf("annotation %s is missing a stage", a.Name)
	}
	fmt.Fprintf(b, "Stability Level: %s\n\n", strings.ToUpper(string(a.Stage)))
	fmt.Fprintf(b, "%s\n\n", a.Help)
	if len(a.Values) > 0 {
		b.WriteString("Values:\n")
		for _, v := range a.Values {
			fmt.Fprintf(b, "- `%s` — %s\n", v.Name, v.Help)
		}
		b.WriteString("\n")
	}
}
