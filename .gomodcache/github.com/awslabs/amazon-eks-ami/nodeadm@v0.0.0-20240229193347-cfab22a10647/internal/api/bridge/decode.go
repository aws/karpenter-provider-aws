package bridge

import (
	"fmt"

	api "github.com/awslabs/amazon-eks-ami/nodeadm/api"
	internalapi "github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// DecodeNodeConfig unmarshals the given data into an internal NodeConfig object.
// The data may be JSON or YAML.
func DecodeNodeConfig(data []byte) (*internalapi.NodeConfig, error) {
	scheme := runtime.NewScheme()
	err := localSchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	codecs := serializer.NewCodecFactory(scheme)
	obj, gvk, err := codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	if gvk.Kind != api.KindNodeConfig {
		return nil, fmt.Errorf("failed to decode %q (wrong Kind)", gvk.Kind)
	}
	if gvk.Group != api.GroupName {
		return nil, fmt.Errorf("failed to decode %q, unexpected group: %s", gvk.Kind, gvk.Group)
	}
	if internalConfig, ok := obj.(*internalapi.NodeConfig); ok {
		return internalConfig, nil
	}
	return nil, fmt.Errorf("unable to convert %T to internal NodeConfig", obj)
}
