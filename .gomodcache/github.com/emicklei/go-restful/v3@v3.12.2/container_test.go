package restful

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

// go test -v -test.run TestContainer_computeAllowedMethods ...restful
func TestContainer_computeAllowedMethods(t *testing.T) {
	wc := NewContainer()
	ws1 := new(WebService).Path("/users")
	ws1.Route(ws1.GET("{i}").To(dummy))
	ws1.Route(ws1.POST("{i}").To(dummy))
	wc.Add(ws1)
	httpRequest, _ := http.NewRequest("GET", "http://api.his.com/users/1", nil)
	rreq := Request{Request: httpRequest}
	m := wc.computeAllowedMethods(&rreq)
	if len(m) != 2 {
		t.Errorf("got %d expected 2 methods, %v", len(m), m)
	}
}

func TestContainer_HandleWithFilter(t *testing.T) {
	prefilterCalled := false
	postfilterCalled := false
	httpHandlerCalled := false

	contextAvailable := false

	wc := NewContainer()
	wc.Filter(func(request *Request, response *Response, chain *FilterChain) {
		prefilterCalled = true
		request.Request = request.Request.WithContext(context.WithValue(request.Request.Context(), "prefilterContextSet", "true"))
		chain.ProcessFilter(request, response)
	})
	wc.HandleWithFilter("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		httpHandlerCalled = true
		_, ok1 := req.Context().Value("prefilterContextSet").(string)
		_, ok2 := req.Context().Value("postfilterContextSet").(string)
		if ok1 && ok2 {
			contextAvailable = true
		}

		w.Write([]byte("ok"))
	}))
	wc.Filter(func(request *Request, response *Response, chain *FilterChain) {
		postfilterCalled = true
		request.Request = request.Request.WithContext(context.WithValue(request.Request.Context(), "postfilterContextSet", "true"))
		chain.ProcessFilter(request, response)
	})

	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/", nil)
	wc.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("unexpected code %d", recorder.Code)
	}
	if recorder.Body.String() != "ok" {
		t.Errorf("unexpected body %s", recorder.Body.String())
	}
	if !prefilterCalled {
		t.Errorf("filter added before calling HandleWithFilter wasn't called")
	}
	if !postfilterCalled {
		t.Errorf("filter added after calling HandleWithFilter wasn't called")
	}
	if !httpHandlerCalled {
		t.Errorf("handler added by calling HandleWithFilter wasn't called")
	}
	if !contextAvailable {
		t.Errorf("Context not available in http handler")
	}
}

func TestContainerAddAndRemove(t *testing.T) {
	ws1 := new(WebService).Path("/")
	ws2 := new(WebService).Path("/users")
	wc := NewContainer()
	wc.Add(ws1)
	wc.Add(ws2)
	wc.Remove(ws2)
	if len(wc.webServices) != 1 {
		t.Errorf("expected one webservices")
	}
	if !wc.isRegisteredOnRoot {
		t.Errorf("expected on root registered")
	}
	wc.Remove(ws1)
	if len(wc.webServices) > 0 {
		t.Errorf("expected zero webservices")
	}
	if wc.isRegisteredOnRoot {
		t.Errorf("expected not on root registered")
	}
}

func TestContainerCompressResponse(t *testing.T) {
	wc := NewContainer()
	ws := new(WebService).Path("/")
	ws.Route(ws.GET("/").To(dummy))
	wc.Add(ws)

	// no accept header, encoding disabled
	{
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/", nil)
		wc.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("unexpected code %d", recorder.Code)
		}
		if recorder.Body.String() != "dummy" {
			t.Errorf("unexpected body %s", recorder.Body.String())
		}
	}

	// with gzip accept header, encoding disabled
	{
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/", nil)
		request.Header.Set("accept-encoding", "gzip")
		wc.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("unexpected code %d", recorder.Code)
		}
		if recorder.Body.String() != "dummy" {
			t.Errorf("unexpected body %s", recorder.Body.String())
		}
	}

	// no accept header, encoding enabled
	{
		wc.EnableContentEncoding(true)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/", nil)
		wc.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("unexpected code %d", recorder.Code)
		}
		if recorder.Body.String() != "dummy" {
			t.Errorf("unexpected body %s", recorder.Body.String())
		}
	}

	// with accept gzip header, encoding enabled
	{
		wc.EnableContentEncoding(true)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/", nil)
		request.Header.Set("accept-encoding", "gzip")
		wc.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("unexpected code %d", recorder.Code)
		}
		if hex.EncodeToString(recorder.Body.Bytes()) == hex.EncodeToString(gzippedDummy()) {
			t.Errorf("unexpected body %v", recorder.Body.Bytes())
		}
	}
	// response says it is already compressed
	{
		wc.EnableContentEncoding(true)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/", nil)
		request.Header.Set("accept-encoding", "gzip")
		recorder.HeaderMap.Set("content-encoding", "gzip")
		wc.ServeHTTP(recorder, request)
		if recorder.Body.String() != "dummy" {
			t.Errorf("unexpected body %s", recorder.Body.String())
		}
	}
}
