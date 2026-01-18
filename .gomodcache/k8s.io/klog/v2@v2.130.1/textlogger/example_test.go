/*
Copyright 2023 The Kubernetes Authors.

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

package textlogger_test

import (
	"bytes"
	"fmt"
	"regexp"

	"k8s.io/klog/v2/textlogger"
)

var headerRe = regexp.MustCompile(`([IE])[[:digit:]]{4} [[:digit:]]{2}:[[:digit:]]{2}:[[:digit:]]{2}\.[[:digit:]]{6}[[:space:]]+[[:digit:]]+ example_test.go:[[:digit:]]+\] `)

func ExampleConfig_Verbosity() {
	var buffer bytes.Buffer
	config := textlogger.NewConfig(textlogger.Verbosity(1), textlogger.Output(&buffer))
	logger := textlogger.NewLogger(config)

	logger.Info("initial verbosity", "v", config.Verbosity().String())
	logger.V(2).Info("now you don't see me")
	if err := config.Verbosity().Set("2"); err != nil {
		logger.Error(err, "setting verbosity to 2")
	}
	logger.V(2).Info("now you see me")
	if err := config.Verbosity().Set("1"); err != nil {
		logger.Error(err, "setting verbosity to 1")
	}
	logger.V(2).Info("now I'm gone again")

	fmt.Print(headerRe.ReplaceAllString(buffer.String(), "${1}...] "))

	// Output:
	// I...] "initial verbosity" v="1"
	// I...] "now you see me"
}
