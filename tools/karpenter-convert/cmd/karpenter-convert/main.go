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

	"github.com/aws/karpenter/tools/karpenter-convert/pkg/convert"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"k8s.io/component-base/cli"
)

func main() {
	kubeConfigFlags := genericclioptions.NewConfigFlags(false).WithDeprecatedPasswordFlag()
	f := cmdutil.NewFactory(kubeConfigFlags)
	cmd := convert.NewCmd(f, genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	code := cli.Run(cmd)
	os.Exit(code)
}
