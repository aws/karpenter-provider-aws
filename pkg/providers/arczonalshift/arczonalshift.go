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
	arczonalshifttypes "github.com/aws/aws-sdk-go-v2/service/arczonalshift/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	IsZonalShifted(context.Context, string) bool
	UpdateZonalShifts(context.Context) error
	ShiftedZones() sets.Set[string]
}

type DefaultProvider struct {
	sync.RWMutex
	zonalShiftStatuses map[string]shiftStatus

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
		zonalShiftStatuses: make(map[string]shiftStatus), // map zoneid (string) -> shiftStatus
		clk:                clk,
		clusterArn:         clusterArn,
	}
}

func (p *DefaultProvider) UpdateZonalShifts(ctx context.Context) error {
	p.Lock()
	defer p.Unlock()

	input := &arczonalshift.GetManagedResourceInput{ResourceIdentifier: &p.clusterArn}
	result, err := p.arcZonalShiftAPI.GetManagedResource(ctx, input)
	if err != nil {
		return fmt.Errorf("getting zonal shifts: %w", err)
	}
	activeZonalShifts := result.ZonalShifts
	shiftStatuses := make(map[string]shiftStatus)

	for _, shift := range activeZonalShifts {
		shiftStatuses[*shift.AwayFrom] = shiftStatus{
			shiftExpiry: *shift.ExpiryTime,
			applied:     shift.AppliedStatus == arczonalshifttypes.AppliedStatusApplied,
		}
	}
	if len(shiftStatuses) == 0 {
		// If there are no shifts on the resource, reset the map
		p.zonalShiftStatuses = make(map[string]shiftStatus)
	} else {
		for zone, status := range shiftStatuses {
			p.zonalShiftStatuses[zone] = status
		}
	}
	log.FromContext(ctx).V(1).Info(fmt.Sprintf("successfully updated zonal shifts %#v", shiftStatuses))
	return nil
}

func (p *DefaultProvider) Reset() {
	p.Lock()
	defer p.Unlock()
	p.zonalShiftStatuses = make(map[string]shiftStatus)
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

func (p *DefaultProvider) ShiftedZones() sets.Set[string] {
	p.RLock()
	defer p.RUnlock()
	shifted := sets.New[string]()
	for zoneID, status := range p.zonalShiftStatuses {
		if status.shiftExpiry.After(p.clk.Now()) && status.applied {
			shifted.Insert(zoneID)
		}
	}
	return shifted
}
