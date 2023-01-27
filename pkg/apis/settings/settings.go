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

package settings

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
)

type NodeNameConvention string

const (
	IPName       NodeNameConvention = "ip-name"
	ResourceName NodeNameConvention = "resource-name"
)

type settingsKeyType struct{}

var ContextKey = settingsKeyType{}

var defaultSettings = &Settings{
	ClusterName:                "",
	ClusterEndpoint:            "",
	DefaultInstanceProfile:     "",
	EnablePodENI:               false,
	EnableENILimitedPodDensity: true,
	IsolatedVPC:                false,
	NodeNameConvention:         IPName,
	VMMemoryOverheadPercent:    0.075,
	InterruptionQueueName:      "",
	Tags:                       map[string]string{},
}

// +k8s:deepcopy-gen=true
type Settings struct {
	ClusterName                string `validate:"required"`
	ClusterEndpoint            string `validate:"required"`
	DefaultInstanceProfile     string
	EnablePodENI               bool
	EnableENILimitedPodDensity bool
	IsolatedVPC                bool
	NodeNameConvention         NodeNameConvention `validate:"required"`
	VMMemoryOverheadPercent    float64            `validate:"min=0"`
	InterruptionQueueName      string
	Tags                       map[string]string
}

func (*Settings) ConfigMap() string {
	return "karpenter-global-settings"
}

// Inject creates a Settings from the supplied ConfigMap
func (*Settings) Inject(ctx context.Context, cm *v1.ConfigMap) (context.Context, error) {
	s := defaultSettings.DeepCopy()

	if err := configmap.Parse(cm.Data,
		configmap.AsString("aws.clusterName", &s.ClusterName),
		configmap.AsString("aws.clusterEndpoint", &s.ClusterEndpoint),
		configmap.AsString("aws.defaultInstanceProfile", &s.DefaultInstanceProfile),
		configmap.AsBool("aws.enablePodENI", &s.EnablePodENI),
		configmap.AsBool("aws.enableENILimitedPodDensity", &s.EnableENILimitedPodDensity),
		configmap.AsBool("aws.isolatedVPC", &s.IsolatedVPC),
		AsTypedString("aws.nodeNameConvention", &s.NodeNameConvention),
		configmap.AsFloat64("aws.vmMemoryOverheadPercent", &s.VMMemoryOverheadPercent),
		configmap.AsString("aws.interruptionQueueName", &s.InterruptionQueueName),
		AsMap("aws.tags", &s.Tags),
	); err != nil {
		return ctx, fmt.Errorf("parsing settings, %w", err)
	}
	if err := s.Validate(); err != nil {
		return ctx, fmt.Errorf("validating settings, %w", err)
	}
	return ToContext(ctx, s), nil
}

// Validate leverages struct tags with go-playground/validator so you can define a struct with custom
// validation on fields i.e.
//
//	type ExampleStruct struct {
//	    Example  metav1.Duration `json:"example" validate:"required,min=10m"`
//	}
func (s Settings) Validate() error {
	return multierr.Combine(
		s.validateEndpoint(),
		validator.New().Struct(s),
	)
}

func (s Settings) validateEndpoint() error {
	endpoint, err := url.Parse(s.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("\"%s\" not a valid clusterEndpoint URL", s.ClusterEndpoint)
	}
	return nil
}

func ToContext(ctx context.Context, s *Settings) context.Context {
	return context.WithValue(ctx, ContextKey, s)
}

func FromContext(ctx context.Context) *Settings {
	data := ctx.Value(ContextKey)
	if data == nil {
		// This is developer error if this happens, so we should panic
		panic("settings doesn't exist in context")
	}
	return data.(*Settings)
}

// AsTypedString passes the value at key through into the target, if it exists.
func AsTypedString[T ~string](key string, target *T) configmap.ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			*target = T(raw)
		}
		return nil
	}
}

// AsMap parses any value with the prefix key into a map with suffixes as keys and values as values in the target map.
// e.g. {"aws.tags.tag1":"value1"} gets parsed into the map Tags as {"tag1": "value1"}
func AsMap(key string, target *map[string]string) configmap.ParseFunc {
	return func(data map[string]string) error {
		m := map[string]string{}

		// Unwind the values into structured keys
		for k, v := range data {
			if strings.HasPrefix(k, key+".") {
				m[k[len(key+"."):]] = v
			}
		}
		*target = m
		return nil
	}
}
