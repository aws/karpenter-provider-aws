// This test delay is based on kind of error
// e.g. HTTP response [Retry-After](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After)
package retry_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/assert"
)

type RetryAfterError struct {
	response http.Response
}

func (err RetryAfterError) Error() string {
	return fmt.Sprintf(
		"Request to %s fail %s (%d)",
		err.response.Request.RequestURI,
		err.response.Status,
		err.response.StatusCode,
	)
}

type SomeOtherError struct {
	err        string
	retryAfter time.Duration
}

func (err SomeOtherError) Error() string {
	return err.err
}

func TestCustomRetryFunctionBasedOnKindOfError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	}))
	defer ts.Close()

	var body []byte

	err := retry.Do(
		func() error {
			resp, err := http.Get(ts.URL)

			if err == nil {
				defer func() {
					if err := resp.Body.Close(); err != nil {
						panic(err)
					}
				}()
				body, err = ioutil.ReadAll(resp.Body)
			}

			return err
		},
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			switch e := err.(type) {
			case RetryAfterError:
				if t, err := parseRetryAfter(e.response.Header.Get("Retry-After")); err == nil {
					return time.Until(t)
				}
			case SomeOtherError:
				return e.retryAfter
			}

			//default is backoffdelay
			return retry.BackOffDelay(n, err, config)
		}),
	)

	assert.NoError(t, err)
	assert.NotEmpty(t, body)
}

// use https://github.com/aereal/go-httpretryafter instead
func parseRetryAfter(_ string) (time.Time, error) {
	return time.Now().Add(1 * time.Second), nil
}
