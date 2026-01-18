//go:build go1.21
// +build go1.21

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
	"log/slog"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/slogr"
)

func hasSlog() bool {
	return true
}

func (m *marshaler) LogValue() slog.Value {
	if m == nil {
		// TODO: simulate crash once slog handles it.
		return slog.StringValue("msg=<nil>")
	}
	return slog.StringValue("msg=" + m.msg)
}

var _ slog.LogValuer = &marshaler{}

func slogInt(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

func slogString(key string, value string) slog.Attr {
	return slog.String(key, value)
}

func slogGroup(key string, values ...interface{}) slog.Attr {
	return slog.Group(key, values...)
}

func slogValue(value interface{}) slog.Value {
	return slog.AnyValue(value)
}

func slogValuer(value interface{}) slog.LogValuer {
	return valuer{value: value}
}

type valuer struct {
	value interface{}
}

func (v valuer) LogValue() slog.Value {
	return slog.AnyValue(v.value)
}

var _ slog.LogValuer = valuer{}

func logWithSlog(l logr.Logger, msg string, withKeysValues, keysValues []interface{}) {
	logger := slog.New(slogr.NewSlogHandler(l))
	if withKeysValues != nil {
		logger = logger.With(withKeysValues...)
	}
	logger.Info(msg, keysValues...)
}
