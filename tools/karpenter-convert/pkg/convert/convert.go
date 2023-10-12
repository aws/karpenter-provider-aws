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

package convert

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	nodepoolutil "github.com/aws/karpenter-core/pkg/utils/nodepool"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type Context struct {
	PrintFlags *genericclioptions.PrintFlags
	Printer    printers.ResourcePrinter

	builder func() *resource.Builder

	resource.FilenameOptions
	genericiooptions.IOStreams
}

func NewCmd(f cmdutil.Factory, ioStreams genericiooptions.IOStreams) *cobra.Command {
	o := Context{
		PrintFlags: genericclioptions.NewPrintFlags("converted").WithDefaultOutput("yaml"),
		IOStreams:  ioStreams,
	}

	var rootCmd = &cobra.Command{
		Use: "karpenter-convert",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.RunConvert())
		},
	}

	cmdutil.AddFilenameOptionFlags(rootCmd, &o.FilenameOptions, "to need to get converted.")
	o.PrintFlags.AddFlags(rootCmd)

	return rootCmd
}

func (o *Context) Complete(f cmdutil.Factory, _ *cobra.Command) (err error) {
	err = o.FilenameOptions.RequireFilenameOrKustomize()
	if err != nil {
		return err
	}
	o.builder = f.NewBuilder
	o.Printer, err = o.PrintFlags.ToPrinter()
	return err
}

func (o *Context) RunConvert() error {
	scheme := runtime.NewScheme()
	if err := apis.AddToScheme(scheme); err != nil {
		return err
	}
	if err := v1alpha5.SchemeBuilder.AddToScheme(scheme); err != nil {
		return err
	}

	b := o.builder().
		WithScheme(scheme, v1alpha1.SchemeGroupVersion, v1alpha5.SchemeGroupVersion).
		LocalParam(true)

	r := b.
		ContinueOnError().
		FilenameParam(false, &o.FilenameOptions).
		Flatten().
		Do().
		IgnoreErrors(func(err error) bool {
			regexPattern := `no kind ".*" is registered for version`
			regex := regexp.MustCompile(regexPattern)
			if regex.MatchString(err.Error()) {
				fmt.Fprintln(o.IOStreams.ErrOut, "#warning:", err.Error())
				return true
			}

			return false
		})

	err := r.Err()
	if err != nil {
		return err
	}

	singleItemImplied := false
	infos, err := r.IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		return fmt.Errorf("no objects passed to convert")
	}

	for _, info := range infos {
		if info.Object == nil {
			continue
		}

		obj, err := Process(info.Object)
		if err != nil {
			fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
		} else {
			if err := o.Printer.PrintObj(obj, o.Out); err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
			}
		}
	}

	return nil
}

func Process(resource runtime.Object) (runtime.Object, error) {
	kind := resource.GetObjectKind().GroupVersionKind().Kind
	switch kind {
	case "Provisioner":
		return processProvisioner(resource), nil
	case "AWSNodeTemplate":
		return processNodeTemplate(resource), nil
	default:
		return nil, fmt.Errorf("unknown kind. expected one of Provisioner, AWSNodeTemplate. got %s", kind)
	}
}

func processNodeTemplate(resource runtime.Object) runtime.Object {
	nodetemplate := resource.(*v1alpha1.AWSNodeTemplate)
	// If the AMIFamily wasn't specified, then we know that it should be AL2 for the conversion
	if nodetemplate.Spec.AMIFamily == nil {
		nodetemplate.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
	}

	nodeclass := nodeclassutil.New(nodetemplate)
	nodeclass.TypeMeta = metav1.TypeMeta{
		Kind:       "EC2NodeClass",
		APIVersion: v1beta1.SchemeGroupVersion.String(),
	}
	nodeclass.Spec.Role = "<your AWS role here>"
	return nodeclass
}

func processProvisioner(resource runtime.Object) runtime.Object {
	provisioner := resource.(*v1alpha5.Provisioner)
	nodepool := nodepoolutil.New(provisioner)
	nodepool.TypeMeta = metav1.TypeMeta{
		Kind:       "NodePool",
		APIVersion: corev1beta1.SchemeGroupVersion.String(),
	}
	return nodepool
}
