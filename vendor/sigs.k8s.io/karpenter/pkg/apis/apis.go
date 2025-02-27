/*
Copyright The Kubernetes Authors.

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

package apis

import (
	_ "embed"

	"github.com/awslabs/operatorpkg/object"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	Group              = "karpenter.sh"
	CompatibilityGroup = "compatibility." + Group
)

//go:generate controller-gen crd object:headerFile="../../hack/boilerplate.go.txt" paths="./..." output:crd:artifacts:config=crds
var (
	//go:embed crds/karpenter.sh_nodepools.yaml
	NodePoolCRD []byte
	//go:embed crds/karpenter.sh_nodeclaims.yaml
	NodeClaimCRD []byte
	CRDs         = []*apiextensionsv1.CustomResourceDefinition{
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](NodePoolCRD),
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](NodeClaimCRD),
	}
)
