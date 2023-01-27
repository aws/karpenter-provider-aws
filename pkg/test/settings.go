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

package test

import (
	"fmt"

	"github.com/imdario/mergo"
	"github.com/samber/lo"

	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
)

type SettingOptions struct {
	ClusterName                *string
	ClusterEndpoint            *string
	DefaultInstanceProfile     *string
	EnablePodENI               *bool
	EnableENILimitedPodDensity *bool
	IsolatedVPC                *bool
	NodeNameConvention         *awssettings.NodeNameConvention
	VMMemoryOverheadPercent    *float64
	InterruptionQueueName      *string
	Tags                       map[string]string
}

func Settings(overrides ...SettingOptions) awssettings.Settings {
	options := SettingOptions{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}
	return awssettings.Settings{
		ClusterName:                lo.FromPtrOr(options.ClusterName, "test-cluster"),
		ClusterEndpoint:            lo.FromPtrOr(options.ClusterEndpoint, "https://test-cluster"),
		DefaultInstanceProfile:     lo.FromPtrOr(options.DefaultInstanceProfile, "test-instance-profile"),
		EnablePodENI:               lo.FromPtrOr(options.EnablePodENI, true),
		EnableENILimitedPodDensity: lo.FromPtrOr(options.EnableENILimitedPodDensity, true),
		IsolatedVPC:                lo.FromPtrOr(options.IsolatedVPC, false),
		NodeNameConvention:         lo.FromPtrOr(options.NodeNameConvention, awssettings.IPName),
		VMMemoryOverheadPercent:    lo.FromPtrOr(options.VMMemoryOverheadPercent, 0.075),
		InterruptionQueueName:      lo.FromPtrOr(options.InterruptionQueueName, ""),
		Tags:                       options.Tags,
	}
}
