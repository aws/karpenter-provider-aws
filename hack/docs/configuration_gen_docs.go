package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/karpenter/pkg/utils/options"
)

func main() {
	if len(os.Args) != 2 {
		log.Printf("Usage: %s path/to/markdown.md", os.Args[0])
		os.Exit(1)
	}
	outputFileName := os.Args[1]
	mdFile, err := os.ReadFile(outputFileName)
	if err != nil {
		log.Printf("Can't read %s file: %v", os.Args[1], err)
		os.Exit(2)
	}

	genStart := "[comment]: <> (the content below is generated from hack/docs/configuration_gen_docs.go)"
	genEnd := "[comment]: <> (end docs generated content from hack/docs/configuration_gen_docs.go)"
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

	opts := options.New()

	envVarsBlock := "| Environment Variable | CLI Flag | Description |\n"
	envVarsBlock += "|--|--|--|\n"
	opts.VisitAll(func(f *flag.Flag) {
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
