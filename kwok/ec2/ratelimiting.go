// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2

import (
	"math"

	"k8s.io/client-go/util/flowcontrol"
)

type RateLimiterProvider interface {
	DescribeLaunchTemplates() flowcontrol.PassiveRateLimiter
	CreateFleet() flowcontrol.PassiveRateLimiter
	TerminateInstances() flowcontrol.PassiveRateLimiter
	DescribeInstances() flowcontrol.PassiveRateLimiter
	RunInstances() flowcontrol.PassiveRateLimiter
	CreateTags() flowcontrol.PassiveRateLimiter
	CreateLaunchTemplate() flowcontrol.PassiveRateLimiter
	DeleteLaunchTemplate() flowcontrol.PassiveRateLimiter
}

type NopRateLimiter struct{}

func (*NopRateLimiter) TryAccept() bool {
	return true
}

func (*NopRateLimiter) Stop() {}

func (*NopRateLimiter) QPS() float32 {
	return math.MaxFloat32
}

type NopRateLimiterProvider struct {
	nopRateLimiter flowcontrol.PassiveRateLimiter
}

func NewNopRateLimiterProvider() *NopRateLimiterProvider {
	return &NopRateLimiterProvider{
		nopRateLimiter: &NopRateLimiter{},
	}
}

func (p NopRateLimiterProvider) DescribeLaunchTemplates() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) CreateFleet() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) TerminateInstances() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) DescribeInstances() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) RunInstances() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) CreateTags() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) CreateLaunchTemplate() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

func (p NopRateLimiterProvider) DeleteLaunchTemplate() flowcontrol.PassiveRateLimiter {
	return p.nopRateLimiter
}

type DefaultRateLimiterProvider struct {
	nonMutating flowcontrol.PassiveRateLimiter
	mutating    flowcontrol.PassiveRateLimiter

	runInstances       flowcontrol.PassiveRateLimiter
	createTags         flowcontrol.PassiveRateLimiter
	terminateInstances flowcontrol.PassiveRateLimiter
}

func NewDefaultRateLimiterProvider() *DefaultRateLimiterProvider {
	return &DefaultRateLimiterProvider{
		nonMutating: flowcontrol.NewTokenBucketPassiveRateLimiter(20, 100),
		mutating:    flowcontrol.NewTokenBucketPassiveRateLimiter(5, 50),

		runInstances:       flowcontrol.NewTokenBucketPassiveRateLimiter(2, 5),
		terminateInstances: flowcontrol.NewTokenBucketPassiveRateLimiter(5, 100),
		createTags:         flowcontrol.NewTokenBucketPassiveRateLimiter(10, 100),
	}
}

func (p *DefaultRateLimiterProvider) DescribeLaunchTemplates() flowcontrol.PassiveRateLimiter {
	return p.nonMutating
}

func (p *DefaultRateLimiterProvider) CreateFleet() flowcontrol.PassiveRateLimiter {
	return p.mutating
}

func (p *DefaultRateLimiterProvider) TerminateInstances() flowcontrol.PassiveRateLimiter {
	return p.terminateInstances
}

func (p *DefaultRateLimiterProvider) DescribeInstances() flowcontrol.PassiveRateLimiter {
	return p.nonMutating
}

func (p *DefaultRateLimiterProvider) RunInstances() flowcontrol.PassiveRateLimiter {
	return p.runInstances
}

func (p *DefaultRateLimiterProvider) CreateTags() flowcontrol.PassiveRateLimiter {
	return p.createTags
}

func (p *DefaultRateLimiterProvider) CreateLaunchTemplate() flowcontrol.PassiveRateLimiter {
	return p.mutating
}

func (p *DefaultRateLimiterProvider) DeleteLaunchTemplate() flowcontrol.PassiveRateLimiter {
	return p.mutating
}
