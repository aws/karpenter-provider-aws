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
	"fmt"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func Setup(opts ...controllerruntimezap.Opts) {
	logger := controllerruntimezap.NewRaw(opts...)
	controllerruntime.SetLogger(zapr.NewLogger(logger))
	zap.ReplaceGlobals(logger)
}

func InvariantViolated(reason string) {
	zap.S().Errorf("Invariant violated: %s. Is the validation webhook installed?", reason)
}

func PanicIfError(err error, formatter string, arguments ...interface{}) {
	if err != nil {
		zap.S().Panicf("%s, %w", fmt.Sprintf(formatter, arguments...), err)
	}
}
