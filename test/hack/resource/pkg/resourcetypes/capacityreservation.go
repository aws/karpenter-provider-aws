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

package resourcetypes

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

const (
	// Reclaimed interruptible instances get a 2 minute warning before they terminate
	capacityReclaimTimeout      = 10 * time.Minute
	capacityReclaimPollInterval = 15 * time.Second

	// Matched by message, like the e2e teardown does
	activeAllocationMessage   = "has an active interruptible capacity allocation"
	noActiveAllocationMessage = "have an active interruptible capacity allocation"
)

type capacityReservationAPI interface {
	ec2.DescribeCapacityReservationsAPIClient
	UpdateInterruptibleCapacityReservationAllocation(ctx context.Context, params *ec2.UpdateInterruptibleCapacityReservationAllocationInput, optFns ...func(*ec2.Options)) (*ec2.UpdateInterruptibleCapacityReservationAllocationOutput, error)
	CancelCapacityReservation(ctx context.Context, params *ec2.CancelCapacityReservationInput, optFns ...func(*ec2.Options)) (*ec2.CancelCapacityReservationOutput, error)
}

type CapacityReservation struct {
	ec2Client      capacityReservationAPI
	reclaimTimeout time.Duration
	pollInterval   time.Duration
}

func NewCapacityReservation(ec2Client capacityReservationAPI) *CapacityReservation {
	return &CapacityReservation{ec2Client: ec2Client, reclaimTimeout: capacityReclaimTimeout, pollInterval: capacityReclaimPollInterval}
}

func (cr *CapacityReservation) String() string {
	return "CapacityReservations"
}

func (cr *CapacityReservation) Global() bool {
	return false
}

// GetExpired only returns reservations tagged by the e2e suites, since other reservations in the account may be in use
func (cr *CapacityReservation) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	reservations, err := cr.getAllCapacityReservations(ctx, activeFilter())
	if err != nil {
		return ids, err
	}
	for _, r := range reservations {
		cluster := reservationCluster(r)
		if cluster == "" || slices.Contains(excludedClusters, cluster) || !lo.FromPtr(r.CreateDate).Before(expirationTime) {
			continue
		}
		ids = append(ids, lo.FromPtr(r.CapacityReservationId))
	}
	return ids, nil
}

func (cr *CapacityReservation) CountAll(ctx context.Context) (count int, err error) {
	reservations, err := cr.getAllCapacityReservations(ctx, &ec2.DescribeCapacityReservationsInput{
		Filters: []ec2types.Filter{
			{
				Name: lo.ToPtr("state"),
				Values: []string{
					string(ec2types.CapacityReservationStateActive),
					string(ec2types.CapacityReservationStatePending),
				},
			},
		},
	})
	if err != nil {
		return count, err
	}

	return len(reservations), err
}

func (cr *CapacityReservation) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	reservations, err := cr.getAllCapacityReservations(ctx, activeFilter())
	if err != nil {
		return ids, err
	}
	return lo.FilterMap(reservations, func(r ec2types.CapacityReservation, _ int) (string, bool) {
		return lo.FromPtr(r.CapacityReservationId), reservationCluster(r) == clusterName
	}), nil
}

// Cleanup cancels reservations. An interruptible reservation can't be cancelled directly, and its source can't be
// cancelled while it has an allocation, so the allocation is reclaimed through the source first.
func (cr *CapacityReservation) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	described, err := cr.describe(ctx, ids)
	if err != nil {
		return nil, err
	}
	var sourceIDs []string
	for _, id := range ids {
		if r, ok := described[id]; ok {
			sourceIDs = append(sourceIDs, sourceID(r))
		}
	}
	sourceIDs = lo.Uniq(lo.Compact(sourceIDs))
	sources, err := cr.describe(ctx, sourceIDs)
	if err != nil {
		return nil, err
	}
	childIDs := lo.Compact(lo.Map(lo.Values(sources), func(r ec2types.CapacityReservation, _ int) string { return allocationID(r) }))
	children, err := cr.describe(ctx, childIDs)
	if err != nil {
		return nil, err
	}

	var errs []error
	var cleaned, toCancel, reclaiming []string
	for _, id := range sourceIDs {
		source, ok := sources[id]
		if !ok || source.State != ec2types.CapacityReservationStateActive {
			continue
		}
		if n := usedInstances(source); n > 0 {
			errs = append(errs, &SkippedError{Reason: fmt.Sprintf("capacity reservation %s has %d instances running", id, n)})
			continue
		}
		child, hasChild := children[allocationID(source)]
		if !hasChild || child.State != ec2types.CapacityReservationStateActive {
			toCancel = append(toCancel, id)
			continue
		}
		childID := lo.FromPtr(child.CapacityReservationId)
		if n := usedInstances(child); n > 0 {
			errs = append(errs, &SkippedError{Reason: fmt.Sprintf("interruptible capacity reservation %s of %s has %d instances running", childID, id, n)})
			continue
		}
		// A reclaim that's already in flight, e.g. from a test's teardown, only needs to be waited on
		if lo.FromPtr(source.InterruptibleCapacityAllocation.TargetInstanceCount) != 0 {
			if _, err := cr.ec2Client.UpdateInterruptibleCapacityReservationAllocation(ctx, &ec2.UpdateInterruptibleCapacityReservationAllocationInput{
				CapacityReservationId: aws.String(id),
				TargetInstanceCount:   aws.Int32(0),
			}); err != nil && !strings.Contains(err.Error(), noActiveAllocationMessage) {
				errs = append(errs, fmt.Errorf("reclaiming interruptible capacity reservation %s from %s, %w", childID, id, err))
				continue
			}
		}
		reclaiming = append(reclaiming, childID)
		toCancel = append(toCancel, id)
	}

	deadline := time.Now().Add(cr.reclaimTimeout)
	pending, err := cr.awaitCancelled(ctx, reclaiming, deadline)
	if err != nil {
		return cleaned, multierr.Append(multierr.Combine(errs...), err)
	}
	cleaned = append(cleaned, lo.Without(reclaiming, pending...)...)
	toCancel = lo.Reject(toCancel, func(id string, _ int) bool {
		if childID := allocationID(sources[id]); slices.Contains(pending, childID) {
			errs = append(errs, fmt.Errorf("interruptible capacity reservation %s of %s wasn't cancelled within %s", childID, id, cr.reclaimTimeout))
			return true
		}
		return false
	})

	// Instances can launch into the capacity the reclaim returned to the source, so check its usage again
	sources, err = cr.describe(ctx, toCancel)
	if err != nil {
		return cleaned, multierr.Append(multierr.Combine(errs...), err)
	}
	for _, id := range toCancel {
		if n := usedInstances(sources[id]); n > 0 {
			errs = append(errs, &SkippedError{Reason: fmt.Sprintf("capacity reservation %s has %d instances running", id, n)})
			continue
		}
		if err := cr.cancel(ctx, id, deadline); err != nil {
			errs = append(errs, fmt.Errorf("cancelling capacity reservation %s, %w", id, err))
			continue
		}
		cleaned = append(cleaned, id)
	}
	return cleaned, multierr.Combine(errs...)
}

// awaitCancelled returns the reservations that are still not cancelled at the deadline
func (cr *CapacityReservation) awaitCancelled(ctx context.Context, ids []string, deadline time.Time) ([]string, error) {
	pending := ids
	for len(pending) > 0 {
		described, err := cr.describe(ctx, pending)
		if err != nil {
			return pending, err
		}
		pending = lo.Filter(pending, func(id string, _ int) bool {
			return described[id].State != ec2types.CapacityReservationStateCancelled
		})
		if len(pending) == 0 || time.Now().After(deadline) {
			break
		}
		if err := sleep(ctx, cr.pollInterval); err != nil {
			return pending, err
		}
	}
	return pending, nil
}

// cancel retries while EC2 still reports the reclaimed allocation as active, which lags the child's cancellation
func (cr *CapacityReservation) cancel(ctx context.Context, id string, deadline time.Time) error {
	for {
		_, err := cr.ec2Client.CancelCapacityReservation(ctx, &ec2.CancelCapacityReservationInput{CapacityReservationId: aws.String(id)})
		if err == nil || !strings.Contains(err.Error(), activeAllocationMessage) || time.Now().After(deadline) {
			return err
		}
		if err := sleep(ctx, cr.pollInterval); err != nil {
			return err
		}
	}
}

func (cr *CapacityReservation) describe(ctx context.Context, ids []string) (map[string]ec2types.CapacityReservation, error) {
	if len(ids) == 0 {
		return map[string]ec2types.CapacityReservation{}, nil
	}
	reservations, err := cr.getAllCapacityReservations(ctx, &ec2.DescribeCapacityReservationsInput{CapacityReservationIds: ids})
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(reservations, func(r ec2types.CapacityReservation) (string, ec2types.CapacityReservation) {
		return lo.FromPtr(r.CapacityReservationId), r
	}), nil
}

func (cr *CapacityReservation) getAllCapacityReservations(ctx context.Context, params *ec2.DescribeCapacityReservationsInput) (reservations []ec2types.CapacityReservation, err error) {
	paginator := ec2.NewDescribeCapacityReservationsPaginator(cr.ec2Client, params)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return reservations, err
		}

		reservations = append(reservations, page.CapacityReservations...)
	}

	return reservations, nil
}

func activeFilter() *ec2.DescribeCapacityReservationsInput {
	return &ec2.DescribeCapacityReservationsInput{
		Filters: []ec2types.Filter{{Name: lo.ToPtr("state"), Values: []string{string(ec2types.CapacityReservationStateActive)}}},
	}
}

func reservationCluster(r ec2types.CapacityReservation) string {
	tag, _ := lo.Find(r.Tags, func(t ec2types.Tag) bool { return lo.FromPtr(t.Key) == karpenterTestingTag })
	return lo.FromPtr(tag.Value)
}

func sourceID(r ec2types.CapacityReservation) string {
	if lo.FromPtr(r.Interruptible) {
		return lo.FromPtr(lo.FromPtr(r.InterruptionInfo).SourceCapacityReservationId)
	}
	return lo.FromPtr(r.CapacityReservationId)
}

// allocationID ignores allocations with no capacity left, whose interruptible reservation is already cancelled and
// may no longer be described
func allocationID(r ec2types.CapacityReservation) string {
	a := r.InterruptibleCapacityAllocation
	if a == nil || a.Status == ec2types.InterruptibleCapacityReservationAllocationStatusCanceled || lo.FromPtr(a.InstanceCount) == 0 {
		return ""
	}
	return lo.FromPtr(a.InterruptibleCapacityReservationId)
}

// usedInstances needs no allocation adjustment, since EC2 moves allocated capacity out of the source's total
func usedInstances(r ec2types.CapacityReservation) int32 {
	return max(lo.FromPtr(r.TotalInstanceCount)-lo.FromPtr(r.AvailableInstanceCount), 0)
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
