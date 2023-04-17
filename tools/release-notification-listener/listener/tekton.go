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

type Suite string

const (
	SuiteUpgrade             Suite = "Upgrade"
	SuiteIntegration         Suite = "Integration"
	SuiteConsolidation       Suite = "Consolidation"
	SuiteUtilization         Suite = "Utilization"
	SuiteInterruption        Suite = "Interruption"
	SuiteChaos               Suite = "Chaos"
	SuiteDrift               Suite = "Drift"
	SuiteMachine             Suite = "Machine"
	SuiteIPv6                Suite = "IPv6"
	SuiteResourceBasedNaming Suite = "ResourceBasedNaming"
)

const (
	gitSHAMaxLength = 7
	noPrNumber      = "none"
)

var (
	tektonCLICommandPath string
	pipelinesAndSuites   = map[Pipeline][]Suite{
		PipelineSuite: {
			SuiteIntegration,
			SuiteConsolidation,
			SuiteUtilization,
			SuiteInterruption,
			SuiteChaos,
			SuiteDrift,
			SuiteMachine,
			SuiteIPv6,
			SuiteResourceBasedNaming,
		},
		PipelineUpgrade: {
			SuiteUpgrade,
		},
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

func tknArgs(message *notificationMessage, pipeline Pipeline, filter Suite) []string {
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

func getPrefixName(message *notificationMessage, pipeline Pipeline, filter Suite) string {
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

func getPipelineParams(pipeline Pipeline, suite Suite, gitRef string) []string {
	params := []string{
		"kubernetes-version=1.23",
		"git-repo-url=https://github.com/aws/karpenter",
		"cleanup=true",
	}
	// Apply test filters to the suites
	switch suite {
	case SuiteUpgrade:
	case SuiteResourceBasedNaming:
		params = append(params, "test-filter=''")
	default:
		params = append(params, fmt.Sprintf("test-filter=%s", suite))
	}
	// Apply label filters to the suites
	switch suite {
	case SuiteUpgrade:
	case SuiteResourceBasedNaming:
		params = append(params, "label-filter=Smoke")
	default:
		params = append(params, "label-filter=''")
	}
	// Apply custom settings for setup given some test suites
	switch suite {
	case SuiteIPv6:
		params = append(params, "ip-family=IPv6")
	case SuiteResourceBasedNaming:
		params = append(params, "hostname-type=resource-name")
	}
	// Apply settings based on the pipeline type
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
