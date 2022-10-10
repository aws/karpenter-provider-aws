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

package scheduledchange

import (
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
)

// AWSEvent contains the properties defined in AWS EventBridge schema
// aws.health@AWSHealthEvent v1.
type AWSEvent struct {
	event.AWSMetadata

	Detail AWSHealthEventDetail `json:"detail"`
}

type AWSHealthEventDetail struct {
	EventARN          string             `json:"eventArn"`
	EventTypeCode     string             `json:"eventTypeCode"`
	Service           string             `json:"service"`
	EventDescription  []EventDescription `json:"eventDescription"`
	StartTime         string             `json:"startTime"`
	EndTime           string             `json:"endTime"`
	EventTypeCategory string             `json:"eventTypeCategory"`
	AffectedEntities  []AffectedEntity   `json:"affectedEntities"`
}

type EventDescription struct {
	LatestDescription string `json:"latestDescription"`
	Language          string `json:"language"`
}

type AffectedEntity struct {
	EntityValue string `json:"entityValue"`
}
