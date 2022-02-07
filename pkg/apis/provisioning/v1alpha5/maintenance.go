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

package v1alpha5

import (
	"github.com/aws/karpenter/pkg/utils/timespan"
	"knative.dev/pkg/apis"
	"regexp"
	"time"
)

type MaintenanceWindows []MaintenanceWindow

type MaintenanceWindow struct {
	WeekDays  []string `json:"weekDays,omitempty"`
	StartTime string   `json:"startTime,omitempty"`
	TimeZone  string   `json:"timeZone,omitempty"`
	Duration  string   `json:"duration,omitempty"`
}

func (m *MaintenanceWindow) Validate() (errs *apis.FieldError) {
	return errs.Also(
		m.ValidateWeekDays(),
		m.ValidateStartTimes(),
		m.ValidateTimeZones(),
		m.ValidateDurations(),
	)
}
func (m *MaintenanceWindow) ValidateWeekDays() (errs *apis.FieldError) {
	if len(m.WeekDays) < 1 && m.WeekDays == nil {
		return errs.Also(apis.ErrMissingField("weekDays"))
	}
	for _, day := range m.WeekDays {
		if _, ok := timespan.WeekDaysToInt[day]; !ok {
			errs = errs.Also(apis.ErrInvalidValue(day, "weekDays", "'MON' | 'TUE' | 'WED' | 'THU' | 'FRI' | 'SAT' | 'SUN' are the only allowed values"))
		}
	}
	return errs
}
func (m *MaintenanceWindow) ValidateStartTimes() (errs *apis.FieldError) {
	if m.StartTime == "" {
		return errs.Also(apis.ErrMissingField("startTime"))
	}
	re := regexp.MustCompile(`^(0[0-9]|1[0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(m.StartTime) {
		errs = errs.Also(apis.ErrInvalidValue(m.StartTime, "startTime", "must be in HH:MM format"))
	}
	return errs
}
func (m *MaintenanceWindow) ValidateTimeZones() (errs *apis.FieldError) {
	if m.TimeZone == "" {
		return errs.Also(apis.ErrMissingField("TimeZone"))
	}
	if _, err := time.LoadLocation(m.TimeZone); err != nil {
		return errs.Also(apis.ErrInvalidValue(m.TimeZone, "timeZone", "timeZone format is invalid, check IANA Time Zone database"))
	}
	return errs
}
func (m *MaintenanceWindow) ValidateDurations() (errs *apis.FieldError) {
	if m.Duration == "" {
		return errs.Also(apis.ErrMissingField("duration"))
	}
	re := regexp.MustCompile(`^(24:00)$|^(0[0-9]|1[0-9]|2[0-3]):([0-5][0-9])$`)
	if !re.MatchString(m.Duration) {
		errs = errs.Also(apis.ErrInvalidValue(m.Duration, "duration", "must be in HH:MM format"))
	}
	return errs
}
