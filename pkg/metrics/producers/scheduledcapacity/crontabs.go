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
	"fmt"
	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/robfig/cron/v3"
	"log"
	"strconv"
	"strings"
	"time"
)

type Crontab struct {
	string
	location *time.Location
}

// crontabFrom returns a Crontab equivalent to the strongly-typed schedule
func crontabFrom(pattern *v1alpha1.Pattern, location *time.Location) *Crontab {
	return &Crontab{
		string: fmt.Sprintf("%s %s %s %s %s", crontabFieldFrom(pattern.Minutes, "0"),
			crontabFieldFrom(pattern.Hours, "0"), crontabFieldFrom(pattern.Days, "*"),
			crontabFieldFrom(pattern.Months, "*"), crontabFieldFrom(pattern.Weekdays, "*")),
		location: location,
	}
}

// crontabFieldFrom returns a field of a strongly-typed format into a Crontab field
func crontabFieldFrom(field *string, nilDefault string) string {
	if field == nil {
		return nilDefault
	}
	elements := strings.Split(*field, ",")
	for i, val := range elements {
		if _, err := strconv.Atoi(val); err != nil {
			elements[i] = strings.ToUpper(val)
		} else {
			elements[i] = val
		}
		elements[i] = strings.Trim(val, " ")
	}
	return strings.Join(elements, ",")
}

// nextTime returns the next time that Crontab will match in its timezone location
func (tab *Crontab) nextTime() (time.Time, error) {
	c := cron.New(cron.WithLocation(tab.location))
	// AddFunc parses the Crontab into a job for the object to use below
	_, err := c.AddFunc(tab.string, func() { log.Printf("crontab %s has been initialized", tab.string) })
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse crontab: %w", err)
	}

	c.Start()
	defer c.Stop()
	nextTime := c.Entries()[0].Next

	return nextTime, nil
}
