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
)

var (
	tektonCLICommandPath string
	pipelinesAndFilters  = map[string][]string{
		"suite": []string{
			"Integration",
			"Consolidation",
			"Utilization",
			"Interruption",
			"Chaos",
			"Drift",
		},
		"ipv6": []string{},
	}
)

func runTests(message *notificationMessage, args ...string) error {
	log.Printf("running tests with args %v for release type: %s, release identifier: %s ", args, message.ReleaseType, message.ReleaseIdentifier)
	cmd := exec.Command(tektonCLICommandPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute the tkn command. %s. %s", output, err)
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

func tknArgs(message *notificationMessage, pipelineName, testFilter string) ([]string, error) {
	prefixFirstPart := strings.ToLower(pipelineName)
	if testFilter != "" {
		prefixFirstPart = strings.ToLower(testFilter)
	}

	prefixSecondPart := shortenedGitSHA(message.ReleaseIdentifier)
	gitRef := message.ReleaseIdentifier
	if message.PrNumber != noPrNumber {
		prefixSecondPart = fmt.Sprintf("pr-%s", message.PrNumber)
		gitRef = fmt.Sprintf("pull/%s/head:tempbranch", message.PrNumber)
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

	testRunParams := []string{
		"kubernetes-version=" + "1.23",
		"git-repo-url=" + "https://github.com/aws/karpenter",
		"git-ref=" + gitRef,
		"test-filter=" + testFilter,
		"cleanup=true",
	}

	for _, param := range testRunParams {
		args = append(args, "--param")
		args = append(args, param)
	}

	return args, nil
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
		return fmt.Errorf("failed to execute the aws command. %s. %s", output, err)
	}
	log.Printf("aws output with args %v. %s", args, output)
	return nil
}
