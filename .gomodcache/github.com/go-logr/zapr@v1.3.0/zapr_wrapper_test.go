/*
Copyright 2021 The logr Authors.

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

package zapr_test

import (
	"github.com/go-logr/logr"
)

func myInfo(logger logr.Logger, msg string, keysAndValues ...interface{}) {
	myInfo2(logger, msg, keysAndValues...)
}

func myInfo2(logger logr.Logger, msg string, keysAndValues ...interface{}) {
	logger.WithCallDepth(2).Info(msg, keysAndValues...)
}

func myInfoInc(logger logr.Logger, msg string, keysAndValues ...interface{}) {
	myInfoInc2(logger.WithCallDepth(1), msg, keysAndValues...)
}

func myInfoInc2(logger logr.Logger, msg string, keysAndValues ...interface{}) {
	logger.WithCallDepth(1).Info(msg, keysAndValues...)
}
