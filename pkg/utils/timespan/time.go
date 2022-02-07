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

package timespan

import (
	"strconv"
	"strings"
	"time"
)

var (
	WeekDaysToInt = map[string]int{
		strings.ToUpper(time.Sunday.String()[0:3]):    int(time.Sunday),
		strings.ToUpper(time.Monday.String()[0:3]):    int(time.Monday),
		strings.ToUpper(time.Tuesday.String()[0:3]):   int(time.Tuesday),
		strings.ToUpper(time.Wednesday.String()[0:3]): int(time.Wednesday),
		strings.ToUpper(time.Thursday.String()[0:3]):  int(time.Thursday),
		strings.ToUpper(time.Friday.String()[0:3]):    int(time.Friday),
		strings.ToUpper(time.Saturday.String()[0:3]):  int(time.Saturday),
	}
	WeekEnd = ParseStringTimeInDuration("SAT", "24:00")
)

func WithInTimeRange(start, end, current time.Duration) bool {
	return start <= current && current <= end
}

func FindMinTimeDuration(durations []time.Duration) time.Duration {
	min := durations[0]
	for _, duration := range durations {
		if duration < min {
			min = duration
		}
	}

	return min
}
func ParseStringTimeInDuration(weekDay, hoursMinutes string) time.Duration {
	timeArr := strings.Split(hoursMinutes, ":")
	hour, _ := strconv.Atoi(timeArr[0])
	minutes, _ := strconv.Atoi(timeArr[1])
	return parseWeekDay(WeekDaysToInt[weekDay]) + parseHoursMinutes(hour, minutes)
}

func ParseTimeWithZoneInDuration(t time.Time, zone string) time.Duration {
	loc, _ := time.LoadLocation(zone)
	now := t.In(loc)
	return parseWeekDay(int(now.Weekday())) + parseHoursMinutes(now.Hour(), now.Minute())
}

func parseWeekDay(weekday int) time.Duration {
	return intToDuration(weekday*24, time.Hour)
}

func parseHoursMinutes(hours, minutes int) time.Duration {
	return intToDuration(hours, time.Hour) + intToDuration(minutes, time.Minute)
}

func intToDuration(t int, duration time.Duration) time.Duration {
	return time.Duration(int64(t)) * duration
}
