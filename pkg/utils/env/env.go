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

package env

import (
	"os"
	"strconv"
)

// WithDefaultInt returns the int value of the supplied environment variable or, if not present,
// the supplied default value. If the int conversion fails, returns the default
func WithDefaultInt(key string, def int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return i
}

// WithDefaultInt64 returns the int value of the supplied environment variable or, if not present,
// the supplied default value. If the int conversion fails, returns the default
func WithDefaultInt64(key string, def int64) int64 {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return def
	}
	return i
}

// WithDefaultFloat64 returns the float64 value of the supplied environment variable or, if not present,
// the supplied default value. If the float64 conversion fails, returns the default
func WithDefaultFloat64(key string, def float64) float64 {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return f
}

// WithDefaultString returns the string value of the supplied environment variable or, if not present,
// the supplied default value.
func WithDefaultString(key string, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	return val
}

// WithDefaultBool returns the boolean value of the supplied environment variable or, if not present,
// the supplied default value.
func WithDefaultBool(key string, def bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	parsedVal, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return parsedVal
}
