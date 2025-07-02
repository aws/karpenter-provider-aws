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
	// replacementMap tracks instance profile replacements (old profile -> new profile)
	// replacementMap = sync.Map{}

	// creationTimeMap tracks when instance profiles were created
	CreationTimeMap = sync.Map{}
)

// TrackReplacement records that newProfile is replacing oldProfile
func TrackReplacement(oldProfile string) {
	// replacementMap.Store(oldProfile, newProfile)
	//creationTimeMap.Store(newProfile, time.Now())
	CreationTimeMap.Store(oldProfile, time.Now())
}

// GetCreationTime returns when an instance profile was created
func GetCreationTime(profileName string) (time.Time, bool) {
	if val, ok := CreationTimeMap.Load(profileName); ok {
		return val.(time.Time), true
	}
	return time.Time{}, false
}

// GetReplacement returns the profile that replaced the given profile
// func GetReplacement(profileName string) (string, bool) {
// 	if val, ok := replacementMap.Load(profileName); ok {
// 		return val.(string), true
// 	}
// 	return "", false
// }

// DeleteTracking removes tracking data for a profile
func DeleteTracking(profileName string) {
	//replacementMap.Delete(profileName)
	CreationTimeMap.Delete(profileName)
}
