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

package instanceprofile

import (
	"sync"
	"time"
)

var (
	// creationTimeMap tracks when instance profiles were created
	CreationTimeMap = sync.Map{}
)

// TrackReplacement records that newProfile is replacing oldProfile and time of replacement
func TrackReplacement(oldProfile string) {
	CreationTimeMap.Store(oldProfile, time.Now())
}

// GetCreationTime returns when an instance profile was created
func GetCreationTime(profileName string) (time.Time, bool) {
	if val, ok := CreationTimeMap.Load(profileName); ok {
		return val.(time.Time), true
	}
	return time.Time{}, false
}

// DeleteTracking removes tracking data for a profile
func DeleteTracking(profileName string) {
	CreationTimeMap.Delete(profileName)
}
