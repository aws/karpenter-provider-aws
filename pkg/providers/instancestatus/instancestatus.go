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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Category string

const (
	InstanceStatus = Category("InstanceStatus")
	SystemStatus   = Category("SystemStatus")
	// EventStatus surfaces scheduled maintenance events. These are also consumed via EventBridge
	// in the Interruption controller when an SQS queue is configured. The handling of maintenance events is
	// currently primitive where we treat all events as instance degradation with an involuntary replacement.
	EventStatus = Category("EventStatus")
)

const (
	ConditionTypeEC2StatusReady      corev1.NodeConditionType = "EC2StatusReady"
	ReasonInstanceReachabilityFailed string                   = "InstanceReachabilityFailed"
	ReasonSystemReachabilityFailed   string                   = "SystemReachabilityFailed"
	ReasonNoImpairmentReported       string                   = "NoImpairmentReported"
)

var (
	UnhealthyThreshold = 120 * time.Second

	// instanceStatusFilters are the server-side filters used to scope DescribeInstanceStatus calls.
	// We use separate filter sets because DescribeInstanceStatus ANDs filters across different
	// filter names, and we need to OR across instance status failures, system status failures,
	// and scheduled maintenance events.
	// EBS status check failures are ignored for now.
	instanceStatusFilters = map[Category][]ec2types.Filter{
		InstanceStatus: {{Name: lo.ToPtr("instance-status.status"), Values: []string{string(ec2types.SummaryStatusImpaired)}}},
		SystemStatus:   {{Name: lo.ToPtr("system-status.status"), Values: []string{string(ec2types.SummaryStatusImpaired)}}},
		EventStatus: {{Name: lo.ToPtr("event.code"), Values: []string{
			string(ec2types.EventCodeInstanceReboot),
			string(ec2types.EventCodeSystemReboot),
			string(ec2types.EventCodeSystemMaintenance),
			string(ec2types.EventCodeInstanceRetirement),
			string(ec2types.EventCodeInstanceStop),
		}}},
	}
)

type Provider interface {
	List(context.Context, ...Category) ([]HealthStatus, error)
}

type DefaultProvider struct {
	ec2api sdk.EC2API
	clk    clock.Clock
}

type HealthStatus struct {
	InstanceID    string
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

// List returns instances that fail the given categories' status checks, with only the failed details of those categories.
// Failures are returned as soon as EC2 reports them; callers decide how long a failure must persist before acting on it.
func (p DefaultProvider) List(ctx context.Context, categories ...Category) ([]HealthStatus, error) {
	seen := map[string]struct{}{}
	var statuses []ec2types.InstanceStatus
	for _, category := range categories {
		filters, ok := instanceStatusFilters[category]
		if !ok {
			panic(fmt.Sprintf("unsupported EC2 instance status category %q", category))
		}
		pager := ec2.NewDescribeInstanceStatusPaginator(p.ec2api, &ec2.DescribeInstanceStatusInput{
			Filters: filters,
		})
		for pager.HasMorePages() {
			out, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed describing ec2 %s checks, %w", category, err)
			}
			for _, s := range out.InstanceStatuses {
				if _, ok := seen[*s.InstanceId]; !ok {
					seen[*s.InstanceId] = struct{}{}
					statuses = append(statuses, s)
				}
			}
		}
	}

	var healthStatuses []HealthStatus
	for _, statusChecks := range statuses {
		healthStatus := p.newHealthStatus(statusChecks)
		// Filter out statuses that we do not consider unhealthy or do not want to handle right now
		healthStatus.Details = lo.Filter(healthStatus.Details, func(details Details, _ int) bool {
			return details.Status == ec2types.StatusTypeFailed && lo.Contains(categories, details.Category)
		})
		if len(healthStatus.Details) == 0 {
			continue
		}
		healthStatus.ImpairedSince = EarliestImpairedSince(healthStatus.Details)
		healthStatuses = append(healthStatuses, healthStatus)
	}
	return healthStatuses, nil
}

// EarliestImpairedSince returns the earliest ImpairedSince reported for the details, or the zero time when EC2 reported none.
func EarliestImpairedSince(details []Details) time.Time {
	impairedSince := lo.FilterMap(details, func(d Details, _ int) (time.Time, bool) { return d.ImpairedSince, !d.ImpairedSince.IsZero() })
	return lo.MinBy(impairedSince, func(a, b time.Time) bool { return a.Before(b) })
}

// newHealthStatus constructs a more consumable version of Health Status Details from the different status checks
func (p DefaultProvider) newHealthStatus(statusChecks ec2types.InstanceStatus) HealthStatus {
	healthStatus := HealthStatus{
		InstanceID: *statusChecks.InstanceId,
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
	ec2Events := details.(ec2types.InstanceStatusEvent)
	return Details{
		Category: category,
		Name:     string(ec2Events.Code),
		// treat all scheduled maintenance events as failures
		Status:        ec2types.StatusTypeFailed,
		ImpairedSince: p.clk.Now(),
	}
}
