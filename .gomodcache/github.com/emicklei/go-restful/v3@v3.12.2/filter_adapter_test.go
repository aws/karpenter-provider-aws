package restful

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "admin" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func newTrace(logger io.Writer) HttpMiddlewareHandler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceId := "TRACE-ID-01234"
			w.Header().Set("x-trace-id", traceId)
			next.ServeHTTP(w, r)
			io.WriteString(logger, traceId)
		})
	}

}

func listUsers(request *Request, response *Response) {
	io.WriteString(response, "alice,bob")
}

func TestHttpMiddlewareHandlerToFilter(t *testing.T) {
	ws := new(WebService)
	ws.Route(ws.GET("/users").Filter(
		HttpMiddlewareHandlerToFilter(auth),
	).To(listUsers))

	var testLogger = bytes.NewBuffer(nil)
	ws.Route(ws.GET("/v2/users").Filter(
		HttpMiddlewareHandlerToFilter(newTrace(testLogger)),
	).Filter(
		HttpMiddlewareHandlerToFilter(auth),
	).To(listUsers))
	container := NewContainer()
	container.Add(ws)

	// test /users, chain: auth
	r, _ := http.NewRequest("GET", "/users", io.NopCloser(nil))
	r.Header.Set("Authorization", "guest")
	rw := httptest.NewRecorder()
	container.ServeHTTP(rw, r)
	if rw.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, but got %d", http.StatusUnauthorized, rw.Code)
	}

	r, _ = http.NewRequest("GET", "/users", io.NopCloser(nil))
	r.Header.Set("Authorization", "admin")
	rw = httptest.NewRecorder()
	container.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK {
		t.Errorf("expected status code %d, but got %d", http.StatusOK, rw.Code)
	}
	if rw.Body.String() != "alice,bob" {
		t.Errorf("expected response body %q, but got %q", "alice,bob", rw.Body.String())
	}

	// test /v2/users, chain: trace + auth
	r, _ = http.NewRequest("GET", "/v2/users", io.NopCloser(nil))
	r.Header.Set("Authorization", "admin")
	rw = httptest.NewRecorder()
	container.ServeHTTP(rw, r)
	if rw.Code != http.StatusOK {
		t.Errorf("expected status code %d, but got %d", http.StatusOK, rw.Code)
	}
	if rw.Body.String() != "alice,bob" {
		t.Errorf("expected response body %q, but got %q", "alice,bob", rw.Body.String())
	}
	if rw.Header().Get("x-trace-id") != "TRACE-ID-01234" {
		t.Errorf("expected trace id %q, but got %q", "TRACE-ID-01234", rw.Header().Get("x-trace-id"))
	}

	loggerOutput := testLogger.String()
	if loggerOutput != "TRACE-ID-01234" {
		t.Errorf("expected logger %q, but got %q", "TRACE-ID-01234", loggerOutput)
	}
}
