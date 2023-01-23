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
)

const (
	gitSHAMaxLength = 7
	noPrNumber      = "none"

	pipelineSuite   = "suite"
	pipelineIPv6    = "ipv6"
	pipelineUpgrade = "upgrade"
)

var (
	tektonCLICommandPath string
	pipelinesAndFilters  = map[string][]string{
		pipelineSuite: {
			"Integration",
			"Consolidation",
			"Utilization",
			"Interruption",
			"Chaos",
			"Drift",
		},
		pipelineIPv6: {
			"IPv6",
		},
		pipelineUpgrade: {},
	}
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

func tknArgs(message *notificationMessage, pipelineName, testFilter string) []string {
	prefixFirstPart := strings.ToLower(pipelineName)
	if testFilter != "" {
		prefixFirstPart = strings.ToLower(testFilter)
	}

	prefixSecondPart := strings.ReplaceAll(shortenedGitSHA(message.ReleaseIdentifier), ".", "-")
	gitRef := message.ReleaseIdentifier
	if message.PrNumber != noPrNumber {
		prefixSecondPart = fmt.Sprintf("pr-%s", message.PrNumber)
		gitRef = fmt.Sprintf("pull/%s/head:tempbranch", message.PrNumber)
	}

	if message.ReleaseType == releaseTypePeriodic {
		prefixSecondPart = releaseTypePeriodic
	}
	prefixName := fmt.Sprintf("%s-%s", prefixFirstPart, prefixSecondPart)

	args := []string{
		"pipeline",
		"start",
		pipelineName,
		"--namespace=karpenter-tests",
		"--serviceaccount=karpenter-tests",
		"--use-param-defaults",
		"--prefix-name=" + prefixName,
	}

	pipelineParams := []string{
		"kubernetes-version=" + "1.23",
		"git-repo-url=" + "https://github.com/aws/karpenter",
		"cleanup=true",
	}

	if testFilter != "" {
		pipelineParams = append(pipelineParams, "test-filter="+testFilter)
	}

	switch pipelineName {
	case pipelineSuite:
		pipelineParams = append(pipelineParams, "git-ref="+gitRef)
	case pipelineIPv6:
		pipelineParams = append(pipelineParams, "ip-family=IPv6")
		pipelineParams = append(pipelineParams, "git-ref="+gitRef)
	case pipelineUpgrade:
		pipelineParams = append(pipelineParams, "from-git-ref="+message.lastStableReleaseTagOrDefault())
		pipelineParams = append(pipelineParams, "to-git-ref="+message.ReleaseIdentifier)
	}

	for _, param := range pipelineParams {
		args = append(args, "--param")
		args = append(args, param)
	}

	return args
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
