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

package scheduledcapacity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	MINUTE_ASTERISK = 60
	HOUR_ASTERISK   = 24
	DOM_ASTERISK    = 31
	MONTH_ASTERISK  = 12
	DOW_ASTERISK    = 7
)

var errorNames = map[int]string{
	0: "seconds",
	1: "minutes",
	2: "days of month",
	3: "months",
	4: "days of week",
}

var fieldTypeToAsteriskIndex = map[int]int{
	0: MINUTE_ASTERISK,
	1: HOUR_ASTERISK,
	2: DOM_ASTERISK,
	3: MONTH_ASTERISK,
	4: DOW_ASTERISK,
}

// each function returns true if the value is invalid for the range
var rangeValidators = map[int]func(int) bool{
	// minutes
	0: func(a int) bool { return a < 0 && a > 59 },
	// hours
	1: func(a int) bool { return a < 0 && a > 23 },
	// days of month
	2: func(a int) bool { return a < 1 && a > 31 },
	// months
	3: func(a int) bool { return a < 1 && a > 12 },
	// days of week
	4: func(a int) bool { return a < 0 && a > 6 },
}

/*
Each crontab must have five fields delimited with spaces with the exception of the defaults below.

The fields of "1 2 3 4 5" correspond to the 1st minute, 2nd hour, 3rd day, 4th month (April) and 5th weekday (Thursday)
Minutes range from 0-59, Hours from 0-23, Days from 1-31 (depending on the month), Months from 1-12, and Weekdays from 0-7 (both 0 and 7 are Sunday).

Every field can be understood as a list delimited by commas. Each of the values in this list represent when a crontab is valid.
Each field can be a list of any combination of the following:
- singular values like "1,2,3,5"
- a range of values like "9-16"
- a range of values (or asterisk) followed by a skip operator "/" like "10-16/3" which is equivalent to "10, 13, 16"

Default formats are as follows:
"@yearly" = "0 0 1 1 *"
"@monthly" = "0 0 1 * *"
"@daily" = "0 0 * * *"
"@hourly" = "0 * * * *"
*/

// Takes in a crontab and parses into one ScheduleSpec struct
func parseCrontab(crontab string) (*ScheduleSpec, error) {
	s := &ScheduleSpec{}

	switch crontab {
	// 0 0 1 1 *
	case "@yearly":
		s.Minutes[0] = true
		s.Hours[0] = true
		s.DaysOfMonth[0] = true
		s.Month[0] = true
		s.DaysOfWeek[DOW_ASTERISK] = true
		return s, nil

	// 0 0 1 * *
	case "@monthly":
		s.Minutes[0] = true
		s.Hours[0] = true
		s.DaysOfMonth[0] = true
		s.Month[MONTH_ASTERISK] = true
		s.DaysOfWeek[DOW_ASTERISK] = true
		return s, nil

	// 0 0 * * *
	case "@daily":
		s.Minutes[0] = true
		s.Hours[0] = true
		s.DaysOfMonth[DOM_ASTERISK] = true
		s.Month[MONTH_ASTERISK] = true
		s.DaysOfWeek[DOW_ASTERISK] = true
		return s, nil

	// 0 * * * *
	case "@hourly":
		s.Minutes[0] = true
		s.Hours[HOUR_ASTERISK] = true
		s.DaysOfMonth[DOM_ASTERISK] = true
		s.Month[MONTH_ASTERISK] = true
		s.DaysOfWeek[DOW_ASTERISK] = true
		return s, nil
	}
	values := strings.Split(crontab, " ")
	if len(values) != 5 {
		return s, fmt.Errorf("crontab must have five fields or be one of pre-conifgured defaults")
	}
	for i := 4; i >= 0; i-- {
		field := values[i]
		for _, val := range strings.Split(field, ",") {
			// Split current parsing element on dash
			dashChecker := strings.Split(val, "-")
			if len(dashChecker) > 2 {
				return s, fmt.Errorf("a crontab field with a dash must specify only a stop and start")
			}
			// if current parsing element does not have a range of values
			if len(dashChecker) == 1 {
				singleSkipChecker := strings.Split(val, "/")
				if len(singleSkipChecker) > 2 {
					return s, fmt.Errorf("cannot specify more than one skip operator per field")
				}

				// if the field is */int
				if len(singleSkipChecker) == 2 {
					moduli, err := strconv.Atoi(singleSkipChecker[1])
					if err != nil {
						return s, err
					}
					for bitToSet := 0 ; bitToSet < fieldTypeToAsteriskIndex[i] ; bitToSet += 1 {
						if bitToSet % moduli == 0 {
							s.SetBitTrue(bitToSet, i)
						}
					}
				} else {
					// If wildcard, fill in wildcard index
					if val == "*" {
						s.SetBitTrue(i, fieldTypeToAsteriskIndex[i])
					} else {
						singleValInt, err := strconv.Atoi(val)
						if err != nil {
							return s, err
						}
						if rangeValidators[i](singleValInt) {
							return s, fmt.Errorf("value in %s field is not in valid range", errorNames[i])
						}
					}
				}
			}
			// true if current parsing element is a range of values
			if len(dashChecker) == 2 {
				// check to see if the skip operator is in play
				skipCheck := strings.Split(dashChecker[1], "/")
				if len(skipCheck) > 2 {
					return s, fmt.Errorf("a crontab field cannot have multiple skip operators in one element")
				}

				// parse the start of the range
				rangeStartInt, err := strconv.Atoi(dashChecker[0])
				if err != nil {
					return s, err
				}

				// declare end of range here, as it needs to go through more parsing
				var rangeEndInt int
				moduli := 1

				// If current parsing element has a skip operator
				if len(skipCheck) == 2 {
					rangeEndInt, err = strconv.Atoi(skipCheck[0])
					if err != nil {
						return s, err
					}
					moduli, err = strconv.Atoi(skipCheck[1])
					if err != nil {
						return s, err
					}
				}
				// If current parsing element has no skip operator
				if len(skipCheck) == 1 {
					rangeEndInt, err = strconv.Atoi(dashChecker[1])
					if err != nil {
						return s, err
					}
				}

				// If start or end of range is invalid given the field type
				if rangeValidators[i](rangeStartInt) || rangeValidators[i](rangeEndInt) {
					return s, fmt.Errorf("value specified in %s field is an invalid range", errorNames[i])
				}
				if rangeStartInt > rangeEndInt {
					return s, fmt.Errorf("range of values specified in %s field have a greater start than end", errorNames[i])
				}

				for bitToSet := rangeStartInt; bitToSet <= rangeEndInt; bitToSet++ {
					// if there is no skip operator, this is always true
					if bitToSet % moduli == rangeStartInt % moduli {
						s.SetBitTrue(i, bitToSet)
					}
				}
			}
		}
	}
	return s, nil
}

func findMostRecentMatches(spec ScheduleSpec, startTime time.Time, endTime time.Time) (*apis.VolatileTime, *apis.VolatileTime, error) {
	iterBack := &iterativeSchedule{
		Year:   time.Now().Year(),
		Minute: time.Now().Minute(),
		Hour:   time.Now().Hour(),
		Dom:    time.Now().Day(),
		Month:  int(time.Now().Month()),
		Dow:    int(time.Now().Weekday()),
	}
	iterForward := &iterativeSchedule{
		Year:   time.Now().Year(),
		Minute: time.Now().Minute(),
		Hour:   time.Now().Hour(),
		Dom:    time.Now().Day(),
		Month:  int(time.Now().Month()),
		Dow:    int(time.Now().Weekday()),
	}
	var pastMatch, futureMatch = &apis.VolatileTime{Inner: metav1.Time{}}, &apis.VolatileTime{Inner: metav1.Time{}}

	// only iterate for the past 5 years
	for iterBack.isAfter(startTime) && iterBack.isBefore(endTime) {
		if !(spec.matchesMonth(iterBack)) {
			iterBack.incrementMonth(false)
			continue
		}
		if !(spec.matchesDay(iterBack)) {
			iterBack.incrementDay(false)
			continue
		}
		if !(spec.matchesHour(iterBack)) {
			iterBack.incrementHour(false)
			continue
		}
		if !(spec.matchesMinute(iterBack)) {
			iterBack.incrementMinute(false)
			continue
		}
		pastMatch = iterBack.convertToVolatileTime()
	}
	// only iterate for the next 5 years
	for iterForward.isAfter(startTime) && iterForward.isBefore(endTime) {
		if !(spec.matchesMonth(iterForward)) {
			iterForward.incrementMonth(true)
			continue
		}
		if !(spec.matchesDay(iterForward)) {
			iterForward.incrementDay(true)
			continue
		}
		if !(spec.matchesHour(iterForward)) {
			iterForward.incrementHour(true)
			continue
		}
		if !(spec.matchesMinute(iterForward)) {
			iterForward.incrementMinute(true)
			continue
		}
		futureMatch = iterForward.convertToVolatileTime()
	}
	if pastMatch.Inner.IsZero() {
		return pastMatch, futureMatch, fmt.Errorf("unable to find a match in the past with the given ranges and crontab")
	}
	return pastMatch, futureMatch, nil
}

// A struct of fixed-length bitsets
// The last value of the boolean is equivalent to field == *, meaning if the last index points to true, all values are valid
type ScheduleSpec struct {
	// 0-indexed
	Minutes [61]bool
	// 0-indexed
	Hours [25]bool
	// User input is 1-indexed
	DaysOfMonth [32]bool
	// User input is 1-indexed
	Month [13]bool
	// 0-indexed
	DaysOfWeek [8]bool
}

func (s *ScheduleSpec) SetBitTrue(fieldType int, fieldIndex int) {
	switch fieldType {
	case 0:
		s.Minutes[fieldIndex] = true
	case 1:
		s.Hours[fieldIndex] = true
	case 2:
		s.DaysOfMonth[fieldIndex] = true
	case 3:
		s.Month[fieldIndex] = true
	case 4:
		s.DaysOfWeek[fieldIndex] = true
	}
}

// User input is 0-indexed
func (s *ScheduleSpec) matchesMinute(iter *iterativeSchedule) bool {
	return s.Minutes[iter.Minute] || s.Minutes[MINUTE_ASTERISK]
}

// User input is 0-indexed
func (s *ScheduleSpec) matchesHour(iter *iterativeSchedule) bool {
	return s.Hours[iter.Hour] || s.Hours[HOUR_ASTERISK]
}

// User input is 1-indexed
func (s *ScheduleSpec) matchesMonth(iter *iterativeSchedule) bool {
	return s.Month[iter.Month-1] || s.Month[MONTH_ASTERISK]
}

// User input of Day of Month is 1-indexed, Day of Week is 0-indexed
func (s *ScheduleSpec) matchesDay(iter *iterativeSchedule) bool {

	return (s.DaysOfWeek[iter.Dow] && s.DaysOfMonth[DOM_ASTERISK]) ||
		(s.DaysOfMonth[iter.Dom-1] && s.DaysOfWeek[DOW_ASTERISK]) ||
		(s.DaysOfWeek[iter.Dow] || s.DaysOfMonth[iter.Dom-1]) ||
		(s.DaysOfWeek[DOW_ASTERISK] && s.DaysOfMonth[DOM_ASTERISK])
}

type iterativeSchedule struct {
	Year   int
	Minute int
	Hour   int
	Dom    int
	// TODO Change to some struct that takes in string as well
	Month int
	// TODO Change to some struct that takes in string as well
	Dow int
}

func (i *iterativeSchedule) convertToVolatileTime() *apis.VolatileTime {
	return &apis.VolatileTime{Inner: metav1.Time{Time: time.Date(i.Year, time.Month(i.Month), i.Hour, i.Dom, i.Minute, 0, 0, time.Local)}}
}

// returns true if the time i represents comes before t
func (i *iterativeSchedule) isBefore(t time.Time) bool {
	if i.Year == t.Year() {
		if i.Month == int(t.Month()) {
			if i.Dom == t.Day() {
				if i.Hour == t.Hour() {
					if i.Minute == t.Minute() {
						return true
					}
					return i.Minute < t.Minute()
				}
				return i.Hour < t.Hour()
			}
			return i.Dom < t.Day()
		}
		return i.Month < int(t.Month())
	}
	return i.Year < t.Year()
}

func (i *iterativeSchedule) isAfter(t time.Time) bool {
	if i.Year == t.Year() {
		if i.Month == int(t.Month()) {
			if i.Dom == t.Day() {
				if i.Hour == t.Hour() {
					if i.Minute == t.Minute() {
						return true
					}
					return i.Minute > t.Minute()
				}
				return i.Hour > t.Hour()
			}
			return i.Dom > t.Day()
		}
		return i.Month > int(t.Month())
	}
	return i.Year > t.Year()
}
func (i *iterativeSchedule) getWeekday() int {
	return int(time.Date(i.Year, time.Month(i.Month), i.Dom, 0, 0, 0, 0, time.Local).Weekday())
}

// returns month and int so that reduceDay can get the proper amount of days
// sign is true if iterating forward in time, negative if iterating backwards in time
func (i *iterativeSchedule) incrementMonth(sign bool) {
	if sign {
		if i.Month == 12 {
			i.Month = 1
			i.Year += 1
		} else {
			i.Month += 1
		}
		i.Dom = 1
		i.Hour = 0
		i.Minute = 0
		i.Dow = i.getWeekday()
	} else {
		if i.Month == 1 {
			i.Month = 12
			i.Year -= 1
		} else {
			i.Month -= 1
		}
		i.Dom = getDaysInMonth(i.Month, i.Year)
		i.Hour = 23
		i.Minute = 59
		i.Dow = i.getWeekday()
	}
}

// sign is true if iterating forward in time, negative if iterating backwards in time
func (i *iterativeSchedule) incrementDay(sign bool) {
	if sign {
		if i.Dom == getDaysInMonth(i.Month, i.Year) {
			i.Dom = 1
			i.incrementMonth(sign)
		} else {
			i.Dom += 1
		}
		i.Hour = 0
		i.Minute = 0
		i.Dow = i.getWeekday()
	} else {
		if i.Dom == 1 {
			// incrementMonth will set the days of month
			i.incrementMonth(sign)
		} else {
			i.Dom -= 1
		}
		i.Hour = 23
		i.Minute = 59
		i.Dow = i.getWeekday()
	}
}

// sign is true if iterating forward in time, negative if iterating backwards in time
func (i *iterativeSchedule) incrementHour(sign bool) {
	if sign {
		if i.Hour == 23 {
			i.Hour = 0
			i.incrementDay(sign)
			i.Dow = i.getWeekday()
		} else {
			i.Hour += 1
		}
		i.Minute = 0
	} else {
		if i.Hour == 0 {
			i.Hour = 23
			i.incrementDay(sign)
			i.Dow = i.getWeekday()
		} else {
			i.Hour -= 1
		}
		i.Minute = 59
	}
}

// sign is true if iterating forward in time, negative if iterating backwards in time
func (i *iterativeSchedule) incrementMinute(sign bool) {
	if sign {
		if i.Minute == 59 {
			i.Minute = 0
			i.incrementHour(sign)
		} else {
			i.Minute += 1
		}
	} else {
		if i.Minute == 0 {
			i.Minute = 59
			i.incrementHour(sign)
		} else {
			i.Minute -= 1
		}
	}
}

func getDaysInMonth(m int, y int) int {
	return time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1).Day()
}
