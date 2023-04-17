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

package listener

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/samber/lo"
)

type Pipeline string

const (
	PipelineSuite   Pipeline = "suite"
	PipelineUpgrade Pipeline = "upgrade"
)

type Filter string

const (
	FilterIntegration         Filter = "Integration"
	FilterConsolidation       Filter = "Consolidation"
	FilterUtilization         Filter = "Utilization"
	FilterInterruption        Filter = "Interruption"
	FilterChaos               Filter = "Chaos"
	FilterDrift               Filter = "Drift"
	FilterMachine             Filter = "Machine"
	FilterIPv6                Filter = "IPv6"
	FilterResourceBasedNaming Filter = "ResourceBasedNaming"
)

const (
	gitSHAMaxLength = 7
	noPrNumber      = "none"
)

var (
	tektonCLICommandPath string
	pipelinesAndFilters  = map[Pipeline][]Filter{
		PipelineSuite: {
			FilterIntegration,
			FilterConsolidation,
			FilterUtilization,
			FilterInterruption,
			FilterChaos,
			FilterDrift,
			FilterMachine,
			FilterIPv6,
			FilterResourceBasedNaming,
		},
		PipelineUpgrade: {},
	}
	preUpgradeVersion = "v0.26.1"
)

func runTests(message *notificationMessage, args ...string) error {
	log.Printf("running tests with args %v for release type: %s, release identifier: %s ", args, message.ReleaseType, message.ReleaseIdentifier)
	cmd := exec.Command(tektonCLICommandPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute the tkn command. %s. %w", output, err)
	}
	log.Printf("tkn start pipeline command output: %s", output)
	return nil
}

func shortenedGitSHA(identifier string) string {
	if len(identifier) > gitSHAMaxLength {
		return identifier[:gitSHAMaxLength]
	}
	return identifier
}

func tknArgs(message *notificationMessage, pipeline Pipeline, filter Filter) []string {
	gitRef := lo.Ternary(message.PrNumber == noPrNumber, message.ReleaseIdentifier, fmt.Sprintf("pull/%s/head:tempbranch", message.PrNumber))

	args := []string{
		"pipeline",
		"start",
		string(pipeline),
		"--namespace=karpenter-tests",
		"--serviceaccount=karpenter-tests",
		"--use-param-defaults",
		"--prefix-name=" + getPrefixName(message, pipeline, filter),
	}
	for _, param := range getPipelineParams(pipeline, filter, gitRef) {
		args = append(args, "--param", param)
	}
	return args
}

func getPrefixName(message *notificationMessage, pipeline Pipeline, filter Filter) string {
	prefixFirstPart := strings.ToLower(string(pipeline))
	if filter != "" {
		prefixFirstPart = strings.ToLower(string(filter))
	}
	prefixSecondPart := strings.ReplaceAll(shortenedGitSHA(message.ReleaseIdentifier), ".", "-")
	if message.PrNumber != noPrNumber {
		prefixSecondPart = fmt.Sprintf("pr-%s", message.PrNumber)
	}
	if message.ReleaseType == releaseTypePeriodic {
		prefixSecondPart = releaseTypePeriodic
	}
	return fmt.Sprintf("%s-%s", prefixFirstPart, prefixSecondPart)
}

func getPipelineParams(pipeline Pipeline, filter Filter, gitRef string) []string {
	params := []string{
		"kubernetes-version=1.23",
		"git-repo-url=https://github.com/aws/karpenter",
		"cleanup=true",
	}
	if filter != "" {
		params = append(params, fmt.Sprintf("test-filter=%s", filter))
	}

	switch filter {
	case FilterIPv6:
		params = append(params, "ip-family=IPv6")
	case FilterResourceBasedNaming:
		params = append(params, "hostname-type=resource-name")
	default:
		params = append(params, "hostname-type=ip-name")
	}
	switch pipeline {
	case PipelineUpgrade:
		params = append(params, fmt.Sprintf("from-git-ref=%s", preUpgradeVersion), fmt.Sprintf("to-git-ref=%s", gitRef))
	default:
		params = append(params, fmt.Sprintf("git-ref=%s", gitRef))
	}
	return params
}

func authenticateEKS(config *config) error {
	args := []string{
		"eks",
		"update-kubeconfig",
		"--name",
		config.tektonClusterName,
		"--region",
		config.region,
	}
	cmd := exec.Command(awsCLICommandPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute the aws command. %s. %w", output, err)
	}
	log.Printf("aws output with args %v. %s", args, output)
	return nil
}
