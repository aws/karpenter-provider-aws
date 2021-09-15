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

// Package v1alpha4 contains API Schema definitions for the v1alpha4 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +groupName=karpenter.k8s.aws
package v1alpha1

import (
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	DefaultLaunchTemplateVersion = "$Default"
	CapacityTypeSpot             = "spot"
	CapacityTypeOnDemand         = "on-demand"
)

var (
	AWSLabelPrefix           = "node.k8s.aws/"
	CapacityTypeLabel        = AWSLabelPrefix + "capacity-type"
	LaunchTemplateNameLabel  = AWSLabelPrefix + "launch-template-name"
	SubnetNameLabel          = AWSLabelPrefix + "subnet-name"
	SubnetTagKeyLabel        = AWSLabelPrefix + "subnet-tag-key"
	SecurityGroupNameLabel   = AWSLabelPrefix + "security-group-name"
	SecurityGroupTagKeyLabel = AWSLabelPrefix + "security-group-tag-key"
	AWSToKubeArchitectures   = map[string]string{
		"x86_64":                   v1alpha4.ArchitectureAmd64,
		v1alpha4.ArchitectureArm64: v1alpha4.ArchitectureArm64,
	}
)

var (
	Scheme = runtime.NewScheme()
	Codec = serializer.NewCodecFactory(Scheme, serializer.EnableStrict)
)

func init() {
	Scheme.AddKnownTypes(schema.GroupVersion{Group: v1alpha4.ExtensionsGroup, Version: "v1alpha1"}, &AWS{})
	v1alpha4.RestrictedLabels = append(v1alpha4.RestrictedLabels, AWSLabelPrefix)
}
