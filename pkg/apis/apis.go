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

	"github.com/awslabs/operatorpkg/object"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//go:generate controller-gen crd object:headerFile="../../hack/boilerplate.go.txt" paths="./..." output:crd:artifacts:config=crds
var (
	Group              = "karpenter.k8s.aws"
	CompatibilityGroup = "compatibility." + Group
	//go:embed crds/karpenter.k8s.aws_ec2nodeclasses.yaml
	EC2NodeClassCRD []byte
	//go:embed crds/karpenter.sh_nodepools.yaml
	NodePoolCRD []byte
	//go:embed crds/karpenter.sh_nodeclaims.yaml
	NodeClaimCRD []byte
	//go:embed crds/karpenter.sh_nodeoverlays.yaml
	NodeOverlayCRD []byte
	CRDs           = []*apiextensionsv1.CustomResourceDefinition{
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](EC2NodeClassCRD),
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](NodeClaimCRD),
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](NodePoolCRD),
		object.Unmarshal[apiextensionsv1.CustomResourceDefinition](NodeOverlayCRD),
	}
)
