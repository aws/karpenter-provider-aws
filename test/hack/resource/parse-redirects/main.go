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
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type RedirectRule struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Status string `json:"status"`
}

func getAvailableKarpenterVersions() ([]string, error) {
	// scans the website content directory to find available for wildcard redirect expansion
	contentDir := "website/content/en"
	entries, err := os.ReadDir(contentDir)
	if err != nil {
		log.Fatalf("Error: Failed to read content directory: %v\n", err)
	}
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			versions = append(versions, name)
		}
	}
	return versions, nil
}

func getRule(source string, target string, versions []string) []RedirectRule {
	var rules []RedirectRule

	if !strings.Contains(source, "*") {
		return []RedirectRule{{
			Source: source,
			Target: target,
			Status: "301",
		}}
	}

	// Expand wildcard for each version
	for _, version := range versions {
		if strings.Contains(target, version) { // ensure non-looping redirects
			continue
		}
		expandedSource := strings.ReplaceAll(source, "*", version)
		rules = append(rules, RedirectRule{
			Source: expandedSource,
			Target: target,
			Status: "301",
		})
	}

	return rules
}

func main() {
	redirectsFile := "website/static/_redirects"
	versions, err := getAvailableKarpenterVersions()
	if err != nil {
		log.Fatalf("Error: Could not get available versions: %v\n", err)
	}

	file, err := os.Open(redirectsFile)
	if err != nil {
		log.Fatalf("Error reading %s: %v\n", redirectsFile, err)
	}
	defer file.Close()

	var rules []RedirectRule
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 {
			expandedRules := getRule(parts[0], parts[1], versions)
			rules = append(rules, expandedRules...)
		} else {
			log.Fatalf("Error: Invalid redirect format on line %d: %s\n", lineNum, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading %s: %v\n", redirectsFile, err)
	}
	jsonData, err := json.Marshal(rules)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v\n", err)
	}
	fmt.Println(string(jsonData))
}
