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

package test

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
)

func RemoveNodeClassTagValidation(crds []*apiextensionsv1.CustomResourceDefinition) []*apiextensionsv1.CustomResourceDefinition {
	for _, crd := range apis.CRDs {
		if crd.Name != "ec2nodeclasses.karpenter.k8s.aws" {
			continue
		}
		overrideProperties := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["tags"]
		overrideProperties.XValidations = nil
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["tags"] = overrideProperties
	}
	return crds
}
