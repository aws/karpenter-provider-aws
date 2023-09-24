package kbump

import (
	"errors"
	"sigs.k8s.io/yaml"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	nodepoolutil "github.com/aws/karpenter-core/pkg/utils/nodepool"
)

type Parameters struct {
	Role *string
}

func Process(inputYAML []byte, params *Parameters) ([]byte, error) {
	var input metav1.TypeMeta
	err := yaml.Unmarshal(inputYAML, &input)
	if err != nil {
		return nil, err
	}

	switch input.Kind {
		case "Provisioner":
			return processProvisioner(inputYAML)
		case "AWSNodeTemplate":
			return processNodeTemplate(inputYAML, params)
		default:
			return nil, errors.New("Unknown Kind")
	}
}

func processNodeTemplate(inputYAML []byte, params *Parameters) ([]byte, error) {
	if params.Role == nil || len(*params.Role) == 0 {
		return nil, errors.New("Parameter 'role' is required. Please specify it with the flag -r MyAwsRole")
	}

	var nodetemplate v1alpha1.AWSNodeTemplate

	err := yaml.Unmarshal(inputYAML, &nodetemplate)
	if err != nil {
		return nil, err
	}

	nodeclass := nodeclassutil.New(&nodetemplate)
	nodeclass.TypeMeta = metav1.TypeMeta{
		Kind: "EC2NodeClass",
		APIVersion: "karpenter.k8s.aws",
	}
	nodeclass.Spec.Role = params.Role
	nodeclass.Spec.InstanceProfile = nil

	outputYAML, err := yaml.Marshal(&nodeclass)
	if err != nil {
		return nil, err
	}
	return outputYAML, nil
}

func processProvisioner(inputYAML []byte) ([]byte, error) {
	var provisioner v1alpha5.Provisioner

	err := yaml.Unmarshal(inputYAML, &provisioner)
	if err != nil {
		return nil, err
	}

	nodepool := nodepoolutil.New(&provisioner)
	nodepool.TypeMeta = metav1.TypeMeta{
		Kind: "NodePool",
		APIVersion: "karpenter.sh/v1beta1",
	}

	outputYAML, err := yaml.Marshal(&nodepool)
	if err != nil {
		return nil, err
	}
	return outputYAML, nil
}