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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type CapacityReservation struct {
	ec2Client *ec2.Client
}

func NewCapacityReservation(ec2Client *ec2.Client) *CapacityReservation {
	return &CapacityReservation{ec2Client: ec2Client}
}

func (cr *CapacityReservation) String() string {
	return "CapacityReservations"
}

func (cr *CapacityReservation) Global() bool {
	return false
}

func (cr *CapacityReservation) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	reservations, err := cr.getAllCapacityReservations(ctx, &ec2.DescribeCapacityReservationsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("state"),
				Values: []string{string(ec2types.CapacityReservationStateActive)},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	reservations = lo.Filter(reservations, func(reservation ec2types.CapacityReservation, _ int) bool {
		return lo.FromPtr(reservation.CreateDate).Before(expirationTime)
	})
	ids = lo.Map(reservations, func(reservation ec2types.CapacityReservation, _ int) string {
		return lo.FromPtr(reservation.CapacityReservationId)
	})

	return ids, err
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
	return ids, nil
}

// Cleanup cancels capacity reservations. For IODCRs, it sets the allocation to 0.
// For ODCRs the reservations are cancelled.
func (cr *CapacityReservation) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	out, err := cr.ec2Client.DescribeCapacityReservations(ctx, &ec2.DescribeCapacityReservationsInput{
		CapacityReservationIds: ids,
	})
	if err != nil {
		return nil, err
	}

	reservationMap := lo.SliceToMap(out.CapacityReservations, func(reservation ec2types.CapacityReservation) (string, ec2types.CapacityReservation) {
		return lo.FromPtr(reservation.CapacityReservationId), reservation
	})

	var iodcrIDs []string
	var odcrIDs []string
	for _, id := range ids {
		if reservation, exists := reservationMap[id]; exists {
			if reservation.Interruptible != nil && lo.FromPtr(reservation.Interruptible) {
				iodcrIDs = append(iodcrIDs, id)
			} else {
				odcrIDs = append(odcrIDs, id)
			}
		}
	}

	cleaned := make([]string, 0, len(ids))
	var errs []error

	for _, id := range iodcrIDs {
		_, err := cr.ec2Client.UpdateInterruptibleCapacityReservationAllocation(ctx, &ec2.UpdateInterruptibleCapacityReservationAllocationInput{
			CapacityReservationId: aws.String(id),
			TargetInstanceCount:   aws.Int32(0),
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cleaned = append(cleaned, id)
	}

	for _, id := range odcrIDs {
		_, err := cr.ec2Client.CancelCapacityReservation(ctx, &ec2.CancelCapacityReservationInput{
			CapacityReservationId: aws.String(id),
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		cleaned = append(cleaned, id)
	}

	return cleaned, multierr.Combine(errs...)
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
