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

package v1alpha1

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	AWSLabelPrefix         = "node.k8s.aws/"
	CapacityTypeLabel      = AWSLabelPrefix + "capacity-type"
	CapacityTypeSpot       = ec2.DefaultTargetCapacityTypeSpot
	CapacityTypeOnDemand   = ec2.DefaultTargetCapacityTypeOnDemand
	AWSToKubeArchitectures = map[string]string{
		"x86_64":                   v1alpha5.ArchitectureAmd64,
		v1alpha5.ArchitectureArm64: v1alpha5.ArchitectureArm64,
	}
	AWSRestrictedLabelDomains = []string{
		"k8s.aws",
	}
)

var (
	Scheme = runtime.NewScheme()
	Codec  = serializer.NewCodecFactory(Scheme, serializer.EnableStrict)
)

func init() {
	Scheme.AddKnownTypes(schema.GroupVersion{Group: v1alpha5.ExtensionsGroup, Version: "v1alpha1"}, &AWS{})
	v1alpha5.RestrictedLabels = append(v1alpha5.RestrictedLabels, AWSLabelPrefix)
	v1alpha5.WellKnownLabels[CapacityTypeLabel] = []string{CapacityTypeSpot, CapacityTypeOnDemand}
	v1alpha5.RestrictedLabelDomains = append(v1alpha5.RestrictedLabelDomains, AWSRestrictedLabelDomains...)
}
