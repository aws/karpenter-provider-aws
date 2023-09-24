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
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/aws/karpenter/tools/kbump/pkg/kbump"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func main() {
	var role string

	var rootCmd = &cobra.Command{
		Use:   "kbump",
		Run: func(cmd *cobra.Command, args []string) {
			stat, err := os.Stdin.Stat()
			if err != nil {
				fmt.Println("file.Stat", err)
				return
			}

			size := stat.Size()
			if size == 0 {
				fmt.Println("No input provided.")
				return
			}

			inputYAML, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				fmt.Println("Error reading input:", err)
				return
			}
	
			var input metav1.TypeMeta
			err = yaml.Unmarshal(inputYAML, &input)
			if err != nil {
				fmt.Println("Error unmarshalling input:", err)
				return
			}
	
			output, err := kbump.Process(inputYAML, &kbump.Parameters{
				Role: &role,
			})
			if err != nil {
				fmt.Println("Error converting input", err)
				return
			}
	
			fmt.Println(string(output))
		},
	}
	rootCmd.Flags().StringVarP(&role, "role", "r", "", "Specify the role (optional)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}