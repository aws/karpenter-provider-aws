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

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/component-base/cli"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/aws/karpenter/tools/karpenter-convert/pkg/convert"
)

func main() {
	kubeConfigFlags := genericclioptions.NewConfigFlags(false)
	f := cmdutil.NewFactory(kubeConfigFlags)
	cmd := convert.NewCmd(f, genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	os.Exit(cli.Run(cmd))
}
