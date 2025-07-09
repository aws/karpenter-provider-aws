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

package bootstrap

import (
	"github.com/pelletier/go-toml/v2"
)

func NewBottlerocketConfig(userdata *string) (*BottlerocketConfig, error) {
	c := &BottlerocketConfig{}
	if userdata == nil {
		return c, nil
	}
	if err := c.UnmarshalTOML([]byte(*userdata)); err != nil {
		return c, err
	}
	return c, nil
}

// BottlerocketConfig is the root of the bottlerocket config, see more here https://github.com/bottlerocket-os/bottlerocket#using-user-data
type BottlerocketConfig struct {
	SettingsRaw map[string]interface{} `toml:"settings"`
}

type BootstrapCommandMode string

const (
	BootstrapCommandModeAlways BootstrapCommandMode = "always"
	BootstrapCommandModeOnce   BootstrapCommandMode = "once"
	BootstrapCommandModeOff    BootstrapCommandMode = "off"
)

// BootstrapCommand model defined in the Bottlerocket Core Kit in
// https://github.com/bottlerocket-os/bottlerocket-core-kit/blob/fdf32c291ad18370de3a5fdc4c20a9588bc14177/sources/bootstrap-commands/src/main.rs#L57
type BootstrapCommand struct {
	Commands  [][]string           `toml:"commands"`
	Mode      BootstrapCommandMode `toml:"mode"`
	Essential bool                 `toml:"essential"`
}

func (c *BottlerocketConfig) UnmarshalTOML(data []byte) error {
	if err := toml.Unmarshal(data, c); err != nil {
		return err
	}
	return nil
}

func (c *BottlerocketConfig) MarshalTOML() ([]byte, error) {
	if c.SettingsRaw == nil {
		c.SettingsRaw = map[string]interface{}{}
	}
	return toml.Marshal(c)
}
