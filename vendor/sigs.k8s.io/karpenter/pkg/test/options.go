/*
Copyright The Kubernetes Authors.

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
	"time"

	"github.com/imdario/mergo"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/operator/options"
)

type OptionsFields struct {
	// Vendor Neutral
	ServiceName             *string
	MetricsPort             *int
	HealthProbePort         *int
	KubeClientQPS           *int
	KubeClientBurst         *int
	EnableProfiling         *bool
	DisableLeaderElection   *bool
	LeaderElectionName      *string
	LeaderElectionNamespace *string
	MemoryLimit             *int64
	CPURequests             *int64
	LogLevel                *string
	LogOutputPaths          *string
	LogErrorOutputPaths     *string
	PreferencePolicy        *options.PreferencePolicy
	MinValuesPolicy         *options.MinValuesPolicy
	BatchMaxDuration        *time.Duration
	BatchIdleDuration       *time.Duration
	FeatureGates            FeatureGates
}

type FeatureGates struct {
	NodeRepair              *bool
	ReservedCapacity        *bool
	SpotToSpotConsolidation *bool
}

func Options(overrides ...OptionsFields) *options.Options {
	opts := OptionsFields{}
	for _, override := range overrides {
		if err := mergo.Merge(&opts, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}

	return &options.Options{
		ServiceName:           lo.FromPtrOr(opts.ServiceName, ""),
		MetricsPort:           lo.FromPtrOr(opts.MetricsPort, 8080),
		HealthProbePort:       lo.FromPtrOr(opts.HealthProbePort, 8081),
		KubeClientQPS:         lo.FromPtrOr(opts.KubeClientQPS, 200),
		KubeClientBurst:       lo.FromPtrOr(opts.KubeClientBurst, 300),
		EnableProfiling:       lo.FromPtrOr(opts.EnableProfiling, false),
		DisableLeaderElection: lo.FromPtrOr(opts.DisableLeaderElection, false),
		MemoryLimit:           lo.FromPtrOr(opts.MemoryLimit, -1),
		CPURequests:           lo.FromPtrOr(opts.CPURequests, 5000), // use 5 threads to enforce parallelism
		LogLevel:              lo.FromPtrOr(opts.LogLevel, ""),
		LogOutputPaths:        lo.FromPtrOr(opts.LogOutputPaths, "stdout"),
		LogErrorOutputPaths:   lo.FromPtrOr(opts.LogErrorOutputPaths, "stderr"),
		BatchMaxDuration:      lo.FromPtrOr(opts.BatchMaxDuration, 10*time.Second),
		BatchIdleDuration:     lo.FromPtrOr(opts.BatchIdleDuration, time.Second),
		PreferencePolicy:      lo.FromPtrOr(opts.PreferencePolicy, options.PreferencePolicyRespect),
		MinValuesPolicy:       lo.FromPtrOr(opts.MinValuesPolicy, options.MinValuesPolicyStrict),
		FeatureGates: options.FeatureGates{
			NodeRepair:              lo.FromPtrOr(opts.FeatureGates.NodeRepair, false),
			ReservedCapacity:        lo.FromPtrOr(opts.FeatureGates.ReservedCapacity, true),
			SpotToSpotConsolidation: lo.FromPtrOr(opts.FeatureGates.SpotToSpotConsolidation, false),
		},
	}
}
