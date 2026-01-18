//go:build !go1.21 && !go1.21
// +build !go1.21,!go1.21

/*
Copyright 2023 The logr Authors.

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
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// These implementations only exist to allow the tests to compile. Test cases
// that depend on slog support get skipped at runtime.

func hasSlog() bool {
	return false
}

func slogInt(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func slogString(key string, value string) zap.Field {
	return zap.String(key, value)
}

func slogGroup(key string, values ...zap.Field) zap.Field {
	return zap.Object(key, zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
		for _, value := range values {
			value.AddTo(encoder)
		}
		return nil
	}))
}

func slogValue(value interface{}) string {
	return fmt.Sprintf("%v", value)
}

func slogValuer(value interface{}) interface{} {
	return value
}

func logWithSlog(_ logr.Logger, _ string, _, _ []interface{}) {
}
