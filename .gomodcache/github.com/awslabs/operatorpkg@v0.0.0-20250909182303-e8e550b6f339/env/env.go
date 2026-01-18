package env

import (
	"os"
	"strconv"
	"time"
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

// WithDefaultDuration returns the duration value of the supplied environment variable or, if not present,
// the supplied default value.
func WithDefaultDuration(key string, def time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	parsedVal, err := time.ParseDuration(val)
	if err != nil {
		return def
	}
	return parsedVal
}
