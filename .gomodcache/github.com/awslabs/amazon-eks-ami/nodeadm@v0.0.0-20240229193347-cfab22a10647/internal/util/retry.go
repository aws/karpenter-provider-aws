package util

import "time"

func RetryExponentialBackoff(attempts int, initial time.Duration, f func() error) error {
	var err error
	wait := initial
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		time.Sleep(wait)
		wait *= 2
	}
	return err
}
