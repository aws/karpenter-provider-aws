package endpointdiscovery

import (
	"net/url"
	"testing"
	"time"
)

func TestEndpointCache_Get_prune(t *testing.T) {
	c := NewEndpointCache(2)
	c.Add(Endpoint{
		Key: "foo",
		Addresses: []WeightedAddress{
			{
				URL: &url.URL{
					Host: "foo.amazonaws.com",
				},
				Expired: time.Now().Add(5 * time.Minute),
			},
			{
				URL: &url.URL{
					Host: "bar.amazonaws.com",
				},
				Expired: time.Now().Add(5 * -time.Minute),
			},
		},
	})

	load, _ := c.endpoints.Load("foo")
	if ev := load.(Endpoint); len(ev.Addresses) != 2 {
		t.Errorf("expected two weighted addresses")
	}

	weightedAddress, ok := c.Get("foo")
	if !ok {
		t.Errorf("expect weighted address, got none")
	}
	if e, a := "foo.amazonaws.com", weightedAddress.URL.Host; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}

	load, _ = c.endpoints.Load("foo")
	if ev := load.(Endpoint); len(ev.Addresses) != 1 {
		t.Errorf("expected one weighted address")
	}
}
