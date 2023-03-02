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
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-playground/validator/v10"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
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
	ReservedENIs:               0,
}

// +k8s:deepcopy-gen=true
type Settings struct {
	ClusterName                string `validate:"required"`
	ClusterEndpoint            string
	DefaultInstanceProfile     string
	EnablePodENI               bool
	EnableENILimitedPodDensity bool
	IsolatedVPC                bool
	NodeNameConvention         NodeNameConvention `validate:"required"`
	VMMemoryOverheadPercent    float64            `validate:"min=0"`
	InterruptionQueueName      string
	Tags                       map[string]string
	ReservedENIs               int `validate:"min=0"`
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
		AsStringMap("aws.tags", &s.Tags),
		configmap.AsInt("aws.reservedENIs", &s.ReservedENIs),
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
		s.validateTags(),
		validator.New().Struct(s),
	)
}

func (s Settings) validateEndpoint() error {
	if s.ClusterEndpoint == "" {
		return nil
	}
	endpoint, err := url.Parse(s.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("\"%s\" not a valid clusterEndpoint URL", s.ClusterEndpoint)
	}
	return nil
}

func (s Settings) validateTags() (err error) {
	for k := range s.Tags {
		for _, pattern := range v1alpha1.RestrictedTagPatterns {
			if pattern.MatchString(k) {
				err = multierr.Append(err, apis.ErrInvalidKeyName(k, "tags", fmt.Sprintf("tag contains a restricted tag %q", pattern.String())))
			}
		}
	}
	return err
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

// AsStringMap parses a value as a JSON map of map[string]string.
func AsStringMap(key string, target *map[string]string) configmap.ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			m := map[string]string{}
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return err
			}
			*target = m
		}
		return nil
	}
}
