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

package v1alpha5

import (
	"strings"

	"knative.dev/pkg/apis"
)

type (
	// ContainerRuntimeConfiguration defines args to be used when configuring the Container Runtime.
	// Note, not all Providers or Container Runtime implementations will support all of these settings.
	ContainerRuntimeConfiguration struct {
		// RegistryMirrors a set of RegistryMirror configurations.
		//+optional
		RegistryMirrors []RegistryMirror `json:"registryMirrors,omitempty"`
	}

	// RegistryMirror configuration
	RegistryMirror struct {
		// Registry the registry's domain name or "*" to match all registries.
		Registry string `json:"registry,omitempty"`
		// Endpoints the endpoints to use as mirrors for that registry.
		Endpoints []RegistryMirrorEndpoint `json:"endpoints,omitempty"`
	}

	// RegistryMirrorEndpoint configuration.
	RegistryMirrorEndpoint struct {
		// URL of the registry mirror endpoint.
		URL string `json:"url,omitempty"`
	}
)

func (c *ContainerRuntimeConfiguration) validate() (errs *apis.FieldError) {
	return c.validateRegistryMirrors()
}

func (c *ContainerRuntimeConfiguration) validateRegistryMirrors() (errs *apis.FieldError) {
	for key, mirror := range c.RegistryMirrors {
		if len(strings.TrimSpace(mirror.Registry)) == 0 {
			errs = errs.Also(errs, apis.ErrMissingField("registry")).ViaFieldIndex("registryMirrors", key)
		}
		for ekey, ep := range mirror.Endpoints {
			if len(strings.TrimSpace(ep.URL)) == 0 {
				errs = errs.Also(errs, apis.ErrMissingField("url")).ViaFieldIndex("registryMirrors", key).ViaFieldIndex("endpoints", ekey)
			}
		}
	}
	return errs
}
