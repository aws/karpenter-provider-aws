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

	"k8s.io/apimachinery/pkg/util/sets"
)

type NoopProvider interface {
	IsZonalShifted(context.Context, string) bool
	UpdateZonalShifts(context.Context) error
	ShiftedZones() sets.Set[string]
}

type DefaultNoopProvider struct {
}

func NewNoopProvider() *DefaultNoopProvider {
	return &DefaultNoopProvider{}
}

func (p *DefaultNoopProvider) UpdateZonalShifts(ctx context.Context) error {
	return nil
}

func (p *DefaultNoopProvider) IsZonalShifted(ctx context.Context, zoneId string) bool {
	return false
}

func (p *DefaultNoopProvider) ShiftedZones() sets.Set[string] {
	return sets.New[string]()
}
