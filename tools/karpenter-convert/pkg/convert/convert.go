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
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/samber/lo"
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

	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	nodepoolutil "github.com/aws/karpenter-core/pkg/utils/nodepool"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/aws/karpenter/pkg/apis/v1alpha5"
)

const karpenterNodeRolePlaceholder string = "$KARPENTER_NODE_ROLE"

type Context struct {
	PrintFlags *genericclioptions.PrintFlags
	Printer    printers.ResourcePrinter

	builder func() *resource.Builder

	resource.FilenameOptions
	genericiooptions.IOStreams

	IgnoreDefaults bool
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

	rootCmd.Flags().BoolVar(&o.IgnoreDefaults, "ignore-defaults", o.IgnoreDefaults, "Ignore defining default requirements when migrating Provisioners to NodePool.")
	cmdutil.AddJsonFilenameFlag(rootCmd.Flags(), &o.Filenames, "Filename, directory, or URL to files to need to get converted.")
	rootCmd.Flags().BoolVarP(&o.Recursive, "recursive", "R", o.Recursive, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")

	o.PrintFlags.AddFlags(rootCmd)

	return rootCmd
}

func (o *Context) Complete(f cmdutil.Factory, _ *cobra.Command) (err error) {
	if len(o.Filenames) == 0 {
		return fmt.Errorf("must specify -f")
	}

	o.builder = f.NewBuilder
	o.Printer, err = o.PrintFlags.ToPrinter()
	return err
}

//nolint:gocyclo
func (o *Context) RunConvert() error {
	scheme := runtime.NewScheme()
	if err := apis.AddToScheme(scheme); err != nil {
		return err
	}
	if err := corev1alpha5.SchemeBuilder.AddToScheme(scheme); err != nil {
		return err
	}

	b := o.builder().
		WithScheme(scheme, v1alpha1.SchemeGroupVersion, corev1alpha5.SchemeGroupVersion).
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

		obj, err := convert(info.Object, o)
		if err != nil {
			fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
		} else {
			var buffer bytes.Buffer
			writer := io.Writer(&buffer)

			if err := o.Printer.PrintObj(obj, writer); err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
			}

			output := dropFields(buffer)

			if _, err := o.Out.Write([]byte(output)); err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
			}
		}
	}
	return nil
}

func dropFields(buffer bytes.Buffer) string {
	output := buffer.String()
	output = strings.Replace(output, "status: {}\n", "", -1)
	output = strings.Replace(output, "      creationTimestamp: null\n", "", -1)
	output = strings.Replace(output, "  creationTimestamp: null\n", "", -1)
	output = strings.Replace(output, "      resources: {}\n", "", -1)

	return output
}

// Convert a Provisioner into a NodePool and an AWSNodeTemplate into a NodeClass.
// If the input is of a different kind, returns an error
func convert(resource runtime.Object, o *Context) (runtime.Object, error) {
	kind := resource.GetObjectKind().GroupVersionKind().Kind
	switch kind {
	case "Provisioner":
		return convertProvisioner(resource, o), nil
	case "AWSNodeTemplate":
		return convertNodeTemplate(resource), nil
	default:
		return nil, fmt.Errorf("unknown kind. expected one of Provisioner, AWSNodeTemplate. got %s", kind)
	}
}

func convertNodeTemplate(resource runtime.Object) runtime.Object {
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

	// From the input NodeTemplate, keep only name, labels, annotations and finalizers
	nodeclass.ObjectMeta = metav1.ObjectMeta{
		Name:        nodetemplate.Name,
		Labels:      nodetemplate.Labels,
		Annotations: nodetemplate.Annotations,
		Finalizers:  nodetemplate.Finalizers,
	}

	// Cleanup the status provided in input
	nodeclass.Status = v1beta1.EC2NodeClassStatus{}

	// Leave a placeholder for the role. This can be substituted with `envsubst` or other means
	nodeclass.Spec.Role = karpenterNodeRolePlaceholder
	return nodeclass
}

func convertProvisioner(resource runtime.Object, o *Context) runtime.Object {
	coreprovisioner := resource.(*corev1alpha5.Provisioner)

	if !o.IgnoreDefaults {
		provisioner := lo.ToPtr(v1alpha5.Provisioner(lo.FromPtr(coreprovisioner)))
		provisioner.SetDefaults(context.Background())
		coreprovisioner = lo.ToPtr(corev1alpha5.Provisioner(lo.FromPtr(provisioner)))
	}

	nodepool := nodepoolutil.New(coreprovisioner)
	nodepool.TypeMeta = metav1.TypeMeta{
		Kind:       "NodePool",
		APIVersion: corev1beta1.SchemeGroupVersion.String(),
	}

	// From the input Provisioner, keep only name, labels, annotations and finalizers
	nodepool.ObjectMeta = metav1.ObjectMeta{
		Name:        coreprovisioner.Name,
		Labels:      coreprovisioner.Labels,
		Annotations: coreprovisioner.Annotations,
		Finalizers:  coreprovisioner.Finalizers,
	}

	// Reset timestamp if present
	nodepool.Spec.Template.CreationTimestamp = metav1.Time{}

	// Cleanup the status provided in input
	nodepool.Status = corev1beta1.NodePoolStatus{}

	return nodepool
}
