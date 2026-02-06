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

package instancestatus

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/utils/clock"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Category string

const (
	InstanceStatus = Category("InstanceStatus")
	SystemStatus   = Category("SystemStatus")
	// EventStatus is currently ignored since this is already consumed via EventBridge in the Interruption controller.
	// The handling of maintenance events is currently primitive where we treat all events as instance degradation
	// with an involuntary replacement. Only consuming events via EventBridge allows some users to opt-out of maintenance
	// event handling (https://github.com/aws/karpenter-provider-aws/issues/8524).
	EventStatus = Category("EventStatus")
	// EBSStatus check failures are currently ignored until we can differentiate which volumes affect the node vs pods w/ PVCs
	EBSStatus = Category("EBSStatus")
)

var (
	UnhealthyThreshold = 120 * time.Second
)

type Provider interface {
	List(context.Context) ([]HealthStatus, error)
}

type DefaultProvider struct {
	ec2api sdk.EC2API
	clk    clock.Clock
}

type HealthStatus struct {
	InstanceID    string
	Overall       ec2types.SummaryStatus
	ImpairedSince time.Time
	Details       []Details
}

type Details struct {
	Category      Category
	Name          string
	ImpairedSince time.Time
	Status        ec2types.StatusType
}

func NewDefaultProvider(ec2API sdk.EC2API, clk clock.Clock) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2API,
		clk:    clk,
	}
}

func (p DefaultProvider) List(ctx context.Context) ([]HealthStatus, error) {
	var statuses []ec2types.InstanceStatus
	pager := ec2.NewDescribeInstanceStatusPaginator(p.ec2api, &ec2.DescribeInstanceStatusInput{})

	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed describing ec2 instance status checks, %w", err)
		}
		statuses = append(statuses, out.InstanceStatuses...)
	}

	var healthStatuses []HealthStatus
	for _, statusChecks := range statuses {
		healthStatus := p.newHealthStatus(statusChecks)
		// Filter out statuses that we do not consider unhealthy or do not want to handle right now
		healthStatus.Details = lo.Filter(healthStatus.Details, func(details Details, _ int) bool {
			if details.Status != ec2types.StatusTypeFailed {
				return false
			}
			// ignore EBS and Scheduled Event health checks for now
			if details.Category == EBSStatus || details.Category == EventStatus {
				return false
			}
			// Do not evaluate against the unhealthy threshold when its a scheduled maintenance event.
			// Scheduled maintenance events often have a future scheduled time which makes a thershold
			// difficult to utilize. We take the stance that if there is a scheduled maintenance event,
			// then there is something wrong with the underlying host that warrants vacating immediately.
			// This matches how we process scheduled maintenance events from EventBridge.
			if details.Category == EventStatus {
				return true
			}
			return p.clk.Since(details.ImpairedSince) >= UnhealthyThreshold
		})
		if len(healthStatus.Details) == 0 {
			continue
		}
		healthStatus.ImpairedSince = slices.MinFunc(healthStatus.Details, func(a, b Details) int {
			return a.ImpairedSince.Compare(b.ImpairedSince)
		}).ImpairedSince
		healthStatuses = append(healthStatuses, healthStatus)
	}
	return healthStatuses, nil
}

// newHealthStatus constructs a more consumable version of Health Status Details from the different status checks
func (p DefaultProvider) newHealthStatus(statusChecks ec2types.InstanceStatus) HealthStatus {
	healthStatus := HealthStatus{
		InstanceID: *statusChecks.InstanceId,
		Overall:    ec2types.SummaryStatusImpaired,
	}
	if statusChecks.InstanceStatus != nil {
		healthStatus.Details = append(healthStatus.Details, lo.Map(statusChecks.InstanceStatus.Details, func(details ec2types.InstanceStatusDetails, _ int) Details {
			return p.newDetails(details, InstanceStatus)
		})...)
	}
	if statusChecks.SystemStatus != nil {
		healthStatus.Details = append(healthStatus.Details, lo.Map(statusChecks.SystemStatus.Details, func(details ec2types.InstanceStatusDetails, _ int) Details {
			return p.newDetails(details, SystemStatus)
		})...)
	}
	if statusChecks.AttachedEbsStatus != nil {
		healthStatus.Details = append(healthStatus.Details, lo.Map(statusChecks.AttachedEbsStatus.Details, func(details ec2types.EbsStatusDetails, _ int) Details {
			return p.newDetails(details, EBSStatus)
		})...)
	}
	healthStatus.Details = append(healthStatus.Details, lo.Map(statusChecks.Events, func(details ec2types.InstanceStatusEvent, _ int) Details {
		return p.newDetails(details, EventStatus)
	})...)
	return healthStatus
}

func (p DefaultProvider) newDetails(details any, category Category) Details {
	if ec2Details, ok := details.(ec2types.InstanceStatusDetails); ok {
		return Details{
			Category:      category,
			Name:          string(ec2Details.Name),
			Status:        ec2Details.Status,
			ImpairedSince: lo.FromPtr(ec2Details.ImpairedSince),
		}
	}
	if ec2Events, ok := details.(ec2types.InstanceStatusEvent); ok {
		return Details{
			Category: category,
			Name:     string(ec2Events.Code),
			// treat all scheduled maintenance events as failures
			Status:        ec2types.StatusTypeFailed,
			ImpairedSince: p.clk.Now(),
		}
	}
	ebsDetails := details.(ec2types.EbsStatusDetails)
	return Details{
		Category:      category,
		Name:          string(ebsDetails.Name),
		Status:        ebsDetails.Status,
		ImpairedSince: lo.FromPtr(ebsDetails.ImpairedSince),
	}
}
