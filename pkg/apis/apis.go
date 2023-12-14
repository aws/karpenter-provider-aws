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

// Package apis contains Kubernetes API groups.
package apis

import (
	_ "embed"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/apis"
	"sigs.k8s.io/karpenter/pkg/utils/functional"
)

var (
	// Builder includes all types within the apis package
	Builder = runtime.NewSchemeBuilder(
		v1beta1.SchemeBuilder.AddToScheme,
	)
	// AddToScheme may be used to add all resources defined in the project to a Scheme
	AddToScheme = Builder.AddToScheme
)

//go:generate controller-gen crd object:headerFile="../../hack/boilerplate.go.txt" paths="./..." output:crd:artifacts:config=crds
var (
	//go:embed crds/karpenter.k8s.aws_ec2nodeclasses.yaml
	EC2NodeClassCRD []byte
	CRDs            = append(apis.CRDs,
		lo.Must(functional.Unmarshal[v1.CustomResourceDefinition](EC2NodeClassCRD)),
	)
)
