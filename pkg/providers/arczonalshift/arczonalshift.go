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

package arczonalshift

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/arczonalshift"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	IsZonalShifted(context.Context, string) bool
	UpdateZonalShifts(context.Context) error
}

type DefaultProvider struct {
	sync.RWMutex
	zonalShiftStatuses map[string]shiftStatus // map zoneid (string) -> shiftStatus

	arcZonalShiftAPI sdk.ARCZonalShiftAPI
	clk              clock.Clock
	clusterArn       string
}

type shiftStatus struct {
	shiftExpiry time.Time
	applied     bool
}

func NewProvider(client sdk.ARCZonalShiftAPI, clk clock.Clock, clusterArn string) *DefaultProvider {
	return &DefaultProvider{
		arcZonalShiftAPI:   client,
		zonalShiftStatuses: make(map[string]shiftStatus),
		clk:                clk,
		clusterArn:         clusterArn,
	}
}

func (p *DefaultProvider) ListZonalShifts(ctx context.Context) (map[string]shiftStatus, error) {
	// Take a write lock over the entire operation to ensure minimize duplicate GetManagedResource calls
	p.Lock()
	defer p.Unlock()

	input := &arczonalshift.GetManagedResourceInput{ResourceIdentifier: &p.clusterArn}
	result, err := p.arcZonalShiftAPI.GetManagedResource(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("getting zonal shifts: %w", err)
	}
	activeZonalShifts := result.ZonalShifts
	activeAutoShifts := result.Autoshifts
	shiftStatuses := make(map[string]shiftStatus)

	if len(activeZonalShifts) > 0 {
		for _, shift := range activeZonalShifts {
			shiftStatuses[*shift.AwayFrom] = shiftStatus{
				shiftExpiry: *shift.ExpiryTime,
				applied:     shift.AppliedStatus == "APPLIED",
			}
		}
	}
	if len(activeAutoShifts) > 0 {
		for _, autoShift := range activeAutoShifts {
			shiftStatuses[*autoShift.AwayFrom] = shiftStatus{
				// Autoshifts do not have an expiration time. Setting expiration to 24 hrs from current timestamp
				// this will get updated on every poll to keep the autoshift expiry updated.
				shiftExpiry: time.Now().Add(24 * time.Hour),
				applied:     autoShift.AppliedStatus == "APPLIED",
			}
		}
	}
	return shiftStatuses, nil
}

func (p *DefaultProvider) UpdateZonalShifts(ctx context.Context) error {
	shiftStatuses, err := p.ListZonalShifts(ctx)
	if err != nil {
		return fmt.Errorf("retrieving zonal shift statues, %w", err)
	}
	if len(shiftStatuses) == 0 {
		p.zonalShiftStatuses = make(map[string]shiftStatus)
	}
	for zone, status := range shiftStatuses {
		p.zonalShiftStatuses[zone] = status
	}
	log.FromContext(ctx).V(1).Info(fmt.Sprintf("successfully updated zonal shifts %#v", shiftStatuses))
	return nil
}

func (p *DefaultProvider) IsZonalShifted(ctx context.Context, zoneId string) bool {
	p.RLock()
	defer p.RUnlock()

	if shift, ok := p.zonalShiftStatuses[zoneId]; ok {
		if shift.shiftExpiry.After(p.clk.Now()) && shift.applied {
			return true
		}
	}

	return false
}
