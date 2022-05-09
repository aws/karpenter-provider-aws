package test

import (
	"fmt"

	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMapOptions struct {
	metav1.ObjectMeta
	Immutable   *bool
	Data       map[string]string
	BinaryData map[string][]byte
}

func ConfigMap(overrides ...ConfigMapOptions) *v1.ConfigMap {
	options := ConfigMapOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge config map options: %s", err))
		}
	}
	return &v1.ConfigMap{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Immutable: options.Immutable,
		Data: options.Data,
		BinaryData: options.BinaryData,
	}
}
