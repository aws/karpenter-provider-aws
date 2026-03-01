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
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kubereplay",
	Short: "EKS audit log replay tool for Karpenter testing",
	Long: `kubereplay captures workload events from EKS audit logs
and replays them against a Karpenter cluster to test provisioning behavior.

Usage:
  kubereplay capture -o replay.json
  kubereplay replay -f replay.json`,
}

func main() {
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(replayCmd)
	rootCmd.AddCommand(demoCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
