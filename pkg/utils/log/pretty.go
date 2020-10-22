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

package log

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

const (
	LinePrefix = ""
	IndentSize = "    "
)

func PrettyInfo(args ...interface{}) {
	var prettyArgs []interface{}
	for _, arg := range args {
		prettyArgs = append(prettyArgs, Pretty(arg))
	}
	zap.S().Info(prettyArgs...)
}

func PrettyInfof(formatter string, args ...interface{}) {
	var prettyArgs []interface{}
	for _, arg := range args {
		prettyArgs = append(prettyArgs, Pretty(arg))
	}
	zap.S().Infof(formatter, prettyArgs...)
}

func Pretty(object interface{}) string {
	if data, err := json.MarshalIndent(object, LinePrefix, IndentSize); err != nil {
		return fmt.Sprintf("failed to print pretty string for object, %v", err)
	} else {
		return string(data)
	}
}
