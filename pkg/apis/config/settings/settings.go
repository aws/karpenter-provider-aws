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
	"strings"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"

	"github.com/aws/karpenter-core/pkg/apis/config"
)

var ContextKey = Registration

var Registration = &config.Registration{
	ConfigMapName: "karpenter-global-settings",
	Constructor:   NewSettingsFromConfigMap,
	DefaultData:   lo.Must(defaultSettings.Data()),
}

var defaultSettings = Settings{
	EnableInterruptionHandling: false,
	Tags:                       map[string]string{},
}

type Settings struct {
	EnableInterruptionHandling bool              `json:"aws.enableInterruptionHandling,string"`
	Tags                       map[string]string `json:"aws.tags,omitempty"`
}

func (s Settings) MarshalJSON() ([]byte, error) {
	type internal Settings
	d := map[string]string{}

	// Store a value of tags locally, so we can marshal the rest of the struct
	tags := s.Tags
	s.Tags = nil

	raw, err := json.Marshal(internal(s))
	if err != nil {
		return nil, fmt.Errorf("marshaling settings, %w", err)
	}
	if err = json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("unmarshalling settings into map, %w", err)
	}
	// Rewind the tags from the map into separate values
	if err = FromMap(tags)("aws.tags", &d); err != nil {
		return nil, fmt.Errorf("rewinding tags into map, %w", err)
	}
	return json.Marshal(d)
}

func (s Settings) Data() (map[string]string, error) {
	d := map[string]string{}

	if err := json.Unmarshal(lo.Must(json.Marshal(defaultSettings)), &d); err != nil {
		return d, fmt.Errorf("unmarshalling json data, %w", err)
	}
	return d, nil
}

// NewSettingsFromConfigMap creates a Settings from the supplied ConfigMap
func NewSettingsFromConfigMap(cm *v1.ConfigMap) (Settings, error) {
	s := defaultSettings

	if err := configmap.Parse(cm.Data,
		configmap.AsBool("aws.enableInterruptionHandling", &s.EnableInterruptionHandling),
		AsMap("aws.tags", &s.Tags),
	); err != nil {
		// Failing to parse means that there is some error in the Settings, so we should crash
		panic(fmt.Sprintf("parsing config data, %v", err))
	}
	return s, nil
}

func ToContext(ctx context.Context, s Settings) context.Context {
	return context.WithValue(ctx, ContextKey, s)
}

func FromContext(ctx context.Context) Settings {
	data := ctx.Value(ContextKey)
	if data == nil {
		// This is developer error if this happens, so we should panic
		panic("settings doesn't exist in context")
	}
	return data.(Settings)
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

// FromMap takes values from a map and rewinds the values into map[string]string values where the key
// contains the prefix key and the value is the map value.
// e.g. {"tag1": "value1"} becomes {"aws.tags.tag1": "value1"} when passed the key "aws.tags"
func FromMap(data map[string]string) func(key string, target *map[string]string) error {
	return func(key string, target *map[string]string) error {
		// Rewind the values into implicit JSON "." syntax
		for k, v := range data {
			(*target)[fmt.Sprintf("%s.%s", key, k)] = v
		}
		return nil
	}
}
