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
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
)

// fakeCapacityReservations models a source reservation whose interruptible reservation is only cancelled a few
// describes after the reclaim, and whose cancel keeps failing for a while after that
type fakeCapacityReservations struct {
	mu           sync.Mutex
	reservations map[string]*ec2types.CapacityReservation
	// reclaimPolls is how many describes of an interruptible reservation it stays active for after a reclaim
	reclaimPolls int
	// cancelLag is how many cancels of a reclaimed source fail before it can be cancelled
	cancelLag int
	// launchedAfterReclaim is how many instances launch into the source once the reclaim returns its capacity
	launchedAfterReclaim int32
	reclaimed            map[string]int
	calls                []string
}

func newFakeCapacityReservations(reservations ...ec2types.CapacityReservation) *fakeCapacityReservations {
	f := &fakeCapacityReservations{reservations: map[string]*ec2types.CapacityReservation{}, reclaimed: map[string]int{}}
	for _, r := range reservations {
		f.reservations[lo.FromPtr(r.CapacityReservationId)] = &r
	}
	return f
}

func (f *fakeCapacityReservations) DescribeCapacityReservations(_ context.Context, in *ec2.DescribeCapacityReservationsInput, _ ...func(*ec2.Options)) (*ec2.DescribeCapacityReservationsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advance()
	var out []ec2types.CapacityReservation
	if len(in.CapacityReservationIds) > 0 {
		for _, id := range in.CapacityReservationIds {
			r, ok := f.reservations[id]
			if !ok {
				return nil, fmt.Errorf("InvalidCapacityReservationId.NotFound: %s", id)
			}
			out = append(out, snapshot(r))
		}
	} else {
		for _, r := range f.reservations {
			out = append(out, snapshot(r))
		}
	}
	return &ec2.DescribeCapacityReservationsOutput{CapacityReservations: lo.Filter(out, func(r ec2types.CapacityReservation, _ int) bool {
		return lo.EveryBy(in.Filters, func(filter ec2types.Filter) bool {
			if lo.FromPtr(filter.Name) != "state" {
				panic(fmt.Sprintf("unsupported filter %s", lo.FromPtr(filter.Name)))
			}
			return slices.Contains(filter.Values, string(r.State))
		})
	})}, nil
}

// snapshot keeps responses from sharing the allocation that the fake mutates later
func snapshot(r *ec2types.CapacityReservation) ec2types.CapacityReservation {
	out := *r
	if a := r.InterruptibleCapacityAllocation; a != nil {
		out.InterruptibleCapacityAllocation = lo.ToPtr(*a)
	}
	return out
}

// advance moves each reclaim one describe further, cancelling the interruptible reservation once it's done
func (f *fakeCapacityReservations) advance() {
	for sourceID, polls := range f.reclaimed {
		source := f.reservations[sourceID]
		child := f.reservations[lo.FromPtr(source.InterruptibleCapacityAllocation.InterruptibleCapacityReservationId)]
		if child.State != ec2types.CapacityReservationStateActive {
			continue
		}
		if polls < f.reclaimPolls {
			f.reclaimed[sourceID]++
			continue
		}
		child.State = ec2types.CapacityReservationStateCancelled
		reclaimed := lo.FromPtr(source.InterruptibleCapacityAllocation.InstanceCount)
		source.TotalInstanceCount = lo.ToPtr(lo.FromPtr(source.TotalInstanceCount) + reclaimed)
		source.AvailableInstanceCount = lo.ToPtr(lo.FromPtr(source.AvailableInstanceCount) + reclaimed - f.launchedAfterReclaim)
		source.InterruptibleCapacityAllocation.InstanceCount = lo.ToPtr[int32](0)
		source.InterruptibleCapacityAllocation.Status = ec2types.InterruptibleCapacityReservationAllocationStatusActive
	}
}

func (f *fakeCapacityReservations) UpdateInterruptibleCapacityReservationAllocation(_ context.Context, in *ec2.UpdateInterruptibleCapacityReservationAllocationInput, _ ...func(*ec2.Options)) (*ec2.UpdateInterruptibleCapacityReservationAllocationOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := lo.FromPtr(in.CapacityReservationId)
	f.calls = append(f.calls, fmt.Sprintf("UpdateInterruptibleCapacityReservationAllocation %s %d", id, lo.FromPtr(in.TargetInstanceCount)))
	r := f.reservations[id]
	if lo.FromPtr(r.Interruptible) {
		return nil, fmt.Errorf("InvalidParameterValue: %s is an interruptible Capacity Reservation", id)
	}
	if r.InterruptibleCapacityAllocation == nil || lo.FromPtr(r.InterruptibleCapacityAllocation.InstanceCount) == 0 {
		return nil, fmt.Errorf("InvalidParameterValue: The Capacity Reservation %s doesn't have an active interruptible capacity allocation", id)
	}
	r.InterruptibleCapacityAllocation.TargetInstanceCount = lo.ToPtr[int32](0)
	r.InterruptibleCapacityAllocation.Status = ec2types.InterruptibleCapacityReservationAllocationStatusUpdating
	f.reclaimed[id] = 0
	return &ec2.UpdateInterruptibleCapacityReservationAllocationOutput{}, nil
}

func (f *fakeCapacityReservations) CancelCapacityReservation(_ context.Context, in *ec2.CancelCapacityReservationInput, _ ...func(*ec2.Options)) (*ec2.CancelCapacityReservationOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := lo.FromPtr(in.CapacityReservationId)
	f.calls = append(f.calls, "CancelCapacityReservation "+id)
	r := f.reservations[id]
	if lo.FromPtr(r.Interruptible) {
		return nil, fmt.Errorf("InvalidParameterValue: %s is an interruptible Capacity Reservation", id)
	}
	if a := r.InterruptibleCapacityAllocation; a != nil && f.reservations[lo.FromPtr(a.InterruptibleCapacityReservationId)].State == ec2types.CapacityReservationStateActive {
		return nil, fmt.Errorf("IncorrectState: The Capacity Reservation %s currently has an active interruptible capacity allocation. You must reclaim your allocated capacity before canceling", id)
	}
	if f.cancelLag > 0 {
		f.cancelLag--
		return nil, fmt.Errorf("IncorrectState: The Capacity Reservation %s currently has an active interruptible capacity allocation. You must reclaim your allocated capacity before canceling", id)
	}
	r.State = ec2types.CapacityReservationStateCancelled
	return &ec2.CancelCapacityReservationOutput{}, nil
}

func (f *fakeCapacityReservations) state(id string) ec2types.CapacityReservationState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reservations[id].State
}

func (f *fakeCapacityReservations) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func odcr(id, cluster string, created time.Time, total, available int32) ec2types.CapacityReservation {
	r := ec2types.CapacityReservation{
		CapacityReservationId:  lo.ToPtr(id),
		State:                  ec2types.CapacityReservationStateActive,
		CreateDate:             lo.ToPtr(created),
		TotalInstanceCount:     lo.ToPtr(total),
		AvailableInstanceCount: lo.ToPtr(available),
		Interruptible:          lo.ToPtr(false),
	}
	if cluster != "" {
		r.Tags = []ec2types.Tag{{Key: lo.ToPtr(karpenterTestingTag), Value: lo.ToPtr(cluster)}}
	}
	return r
}

// interruptiblePair moves allocated of the source's capacity out of its total into an interruptible reservation, with
// used of it running
func interruptiblePair(sourceID, childID, cluster string, created time.Time, total, allocated, used int32) (ec2types.CapacityReservation, ec2types.CapacityReservation) {
	source := odcr(sourceID, cluster, created, total-allocated, total-allocated)
	source.InterruptibleCapacityAllocation = &ec2types.InterruptibleCapacityAllocation{
		InterruptibleCapacityReservationId: lo.ToPtr(childID),
		InstanceCount:                      lo.ToPtr(allocated),
		TargetInstanceCount:                lo.ToPtr(allocated),
		Status:                             ec2types.InterruptibleCapacityReservationAllocationStatusActive,
	}
	child := odcr(childID, cluster, created.Add(time.Minute), allocated, allocated-used)
	child.Interruptible = lo.ToPtr(true)
	child.InterruptionInfo = &ec2types.InterruptionInfo{SourceCapacityReservationId: lo.ToPtr(sourceID)}
	return source, child
}

func testCapacityReservation(f *fakeCapacityReservations) *CapacityReservation {
	return &CapacityReservation{ec2Client: f, reclaimTimeout: time.Second, pollInterval: time.Millisecond}
}

func TestCapacityReservationGetExpired(t *testing.T) {
	expiration := time.Now().Add(-12 * time.Hour)
	old, young := expiration.Add(-time.Hour), expiration.Add(time.Hour)
	source, child := interruptiblePair("cr-source", "cr-child", "gone-1", old, 2, 1, 0)
	cancelled := odcr("cr-cancelled", "gone-1", old, 1, 1)
	cancelled.State = ec2types.CapacityReservationStateCancelled
	f := newFakeCapacityReservations(
		source, child, cancelled,
		odcr("cr-old", "gone-1", old, 1, 1),
		odcr("cr-young", "gone-1", young, 1, 1),
		odcr("cr-untagged", "", old, 1, 1),
		odcr("cr-excluded", "excluded-1", old, 1, 1),
	)
	ids, err := testCapacityReservation(f).GetExpired(context.Background(), expiration, []string{"excluded-1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cr-child", "cr-old", "cr-source"}; !slices.Equal(slices.Sorted(slices.Values(ids)), want) {
		t.Errorf("GetExpired() = %v, want %v", ids, want)
	}
}

func TestCapacityReservationCleanupReclaimsThroughTheSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{name: "source and interruptible", ids: []string{"cr-source", "cr-child"}},
		{name: "interruptible only", ids: []string{"cr-child"}},
		{name: "source only", ids: []string{"cr-source"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, child := interruptiblePair("cr-source", "cr-child", "gone-1", time.Now().Add(-24*time.Hour), 2, 1, 0)
			f := newFakeCapacityReservations(source, child)
			f.reclaimPolls, f.cancelLag = 3, 2

			cleaned, err := testCapacityReservation(f).Cleanup(context.Background(), tc.ids)
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{"cr-child", "cr-source"}; !slices.Equal(slices.Sorted(slices.Values(cleaned)), want) {
				t.Errorf("cleaned = %v, want %v", cleaned, want)
			}
			want := []string{
				"UpdateInterruptibleCapacityReservationAllocation cr-source 0",
				"CancelCapacityReservation cr-source",
				"CancelCapacityReservation cr-source",
				"CancelCapacityReservation cr-source",
			}
			if got := f.writes(); !slices.Equal(got, want) {
				t.Errorf("calls = %v, want %v", got, want)
			}
			if f.state("cr-source") != ec2types.CapacityReservationStateCancelled || f.state("cr-child") != ec2types.CapacityReservationStateCancelled {
				t.Errorf("source is %s and interruptible is %s, want both cancelled", f.state("cr-source"), f.state("cr-child"))
			}
		})
	}
}

func TestCapacityReservationCleanupWaitsForAnInFlightReclaim(t *testing.T) {
	source, child := interruptiblePair("cr-source", "cr-child", "gone-1", time.Now().Add(-24*time.Hour), 2, 1, 0)
	f := newFakeCapacityReservations(source, child)
	f.reclaimPolls = 2
	if _, err := f.UpdateInterruptibleCapacityReservationAllocation(context.Background(), &ec2.UpdateInterruptibleCapacityReservationAllocationInput{
		CapacityReservationId: lo.ToPtr("cr-source"), TargetInstanceCount: lo.ToPtr[int32](0),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := testCapacityReservation(f).Cleanup(context.Background(), []string{"cr-source"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"UpdateInterruptibleCapacityReservationAllocation cr-source 0", "CancelCapacityReservation cr-source"}
	if got := f.writes(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestCapacityReservationCleanupSkipsReservationsInUse(t *testing.T) {
	old := time.Now().Add(-24 * time.Hour)
	for _, tc := range []struct {
		name         string
		reservations func() []ec2types.CapacityReservation
		ids          []string
	}{
		{name: "plain", ids: []string{"cr-1"}, reservations: func() []ec2types.CapacityReservation {
			return []ec2types.CapacityReservation{odcr("cr-1", "gone-1", old, 2, 1)}
		}},
		{name: "interruptible", ids: []string{"cr-child"}, reservations: func() []ec2types.CapacityReservation {
			source, child := interruptiblePair("cr-source", "cr-child", "gone-1", old, 2, 1, 1)
			return []ec2types.CapacityReservation{source, child}
		}},
		{name: "source of an interruptible", ids: []string{"cr-source"}, reservations: func() []ec2types.CapacityReservation {
			source, child := interruptiblePair("cr-source", "cr-child", "gone-1", old, 3, 1, 0)
			source.AvailableInstanceCount = lo.ToPtr(lo.FromPtr(source.AvailableInstanceCount) - 1)
			return []ec2types.CapacityReservation{source, child}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeCapacityReservations(tc.reservations()...)
			cleaned, err := testCapacityReservation(f).Cleanup(context.Background(), tc.ids)
			if len(cleaned) != 0 || len(f.writes()) != 0 {
				t.Errorf("cleaned = %v, calls = %v, want none", cleaned, f.writes())
			}
			var skipped *SkippedError
			if !errors.As(err, &skipped) || Fatal(err) {
				t.Errorf("err = %v, want a non-fatal SkippedError", err)
			}
		})
	}
}

func TestCapacityReservationCleanupSkipsASourceInUseAfterTheReclaim(t *testing.T) {
	source, child := interruptiblePair("cr-source", "cr-child", "gone-1", time.Now().Add(-24*time.Hour), 2, 1, 0)
	f := newFakeCapacityReservations(source, child)
	f.reclaimPolls, f.launchedAfterReclaim = 1, 1

	cleaned, err := testCapacityReservation(f).Cleanup(context.Background(), []string{"cr-source"})
	if !slices.Equal(cleaned, []string{"cr-child"}) || slices.Contains(f.writes(), "CancelCapacityReservation cr-source") {
		t.Errorf("cleaned = %v, calls = %v, want only the interruptible reservation reclaimed", cleaned, f.writes())
	}
	var skipped *SkippedError
	if !errors.As(err, &skipped) || Fatal(err) {
		t.Errorf("err = %v, want a non-fatal SkippedError", err)
	}
}

func TestCapacityReservationCleanupFailsWhenTheReclaimDoesNotFinish(t *testing.T) {
	source, child := interruptiblePair("cr-source", "cr-child", "gone-1", time.Now().Add(-24*time.Hour), 2, 1, 0)
	plain := odcr("cr-plain", "gone-1", time.Now().Add(-24*time.Hour), 1, 1)
	f := newFakeCapacityReservations(source, child, plain)
	f.reclaimPolls = 1 << 30
	cr := testCapacityReservation(f)
	cr.reclaimTimeout = 20 * time.Millisecond

	cleaned, err := cr.Cleanup(context.Background(), []string{"cr-source", "cr-plain"})
	if !Fatal(err) {
		t.Errorf("err = %v, want a fatal error", err)
	}
	if !slices.Equal(cleaned, []string{"cr-plain"}) {
		t.Errorf("cleaned = %v, want [cr-plain]", cleaned)
	}
	if slices.Contains(f.writes(), "CancelCapacityReservation cr-source") {
		t.Errorf("calls = %v, cancelled the source while its allocation was active", f.writes())
	}
}
