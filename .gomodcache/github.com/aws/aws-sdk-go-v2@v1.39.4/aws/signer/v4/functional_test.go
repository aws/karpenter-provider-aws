package v4_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	v4Internal "github.com/aws/aws-sdk-go-v2/aws/signer/internal/v4"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/internal/awstesting/unit"
)

var standaloneSignCases = []struct {
	OrigURI                    string
	OrigQuery                  string
	Region, Service, SubDomain string
	ExpSig                     string
	EscapedURI                 string
}{
	{
		OrigURI:   `/logs-*/_search`,
		OrigQuery: `pretty=true`,
		Region:    "us-west-2", Service: "es", SubDomain: "hostname-clusterkey",
		EscapedURI: `/logs-%2A/_search`,
		ExpSig:     `AWS4-HMAC-SHA256 Credential=AKID/19700101/us-west-2/es/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=79d0760751907af16f64a537c1242416dacf51204a7dd5284492d15577973b91`,
	},
}

func TestStandaloneSign_CustomURIEscape(t *testing.T) {
	var expectSig = `AWS4-HMAC-SHA256 Credential=AKID/19700101/us-east-1/es/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=6601e883cc6d23871fd6c2a394c5677ea2b8c82b04a6446786d64cd74f520967`

	creds, err := unit.Config().Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	signer := v4.NewSigner(func(signer *v4.SignerOptions) {
		signer.DisableURIPathEscaping = true
	})

	host := "https://subdomain.us-east-1.es.amazonaws.com"
	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	req.URL.Path = `/log-*/_search`
	req.URL.Opaque = "//subdomain.us-east-1.es.amazonaws.com/log-%2A/_search"

	err = signer.SignHTTP(context.Background(), creds, req, v4Internal.EmptyStringSHA256, "es", "us-east-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}

	actual := req.Header.Get("Authorization")
	if e, a := expectSig, actual; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestStandaloneSign(t *testing.T) {
	creds, err := unit.Config().Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	signer := v4.NewSigner()

	for _, c := range standaloneSignCases {
		host := fmt.Sprintf("https://%s.%s.%s.amazonaws.com",
			c.SubDomain, c.Region, c.Service)

		req, err := http.NewRequest("GET", host, nil)
		if err != nil {
			t.Errorf("expected no error, but received %v", err)
		}

		// URL.EscapedPath() will be used by the signer to get the
		// escaped form of the request's URI path.
		req.URL.Path = c.OrigURI
		req.URL.RawQuery = c.OrigQuery

		err = signer.SignHTTP(context.Background(), creds, req, v4Internal.EmptyStringSHA256, c.Service, c.Region, time.Unix(0, 0))
		if err != nil {
			t.Errorf("expected no error, but received %v", err)
		}

		actual := req.Header.Get("Authorization")
		if e, a := c.ExpSig, actual; e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
		if e, a := c.OrigURI, req.URL.Path; e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
		if e, a := c.EscapedURI, req.URL.EscapedPath(); e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
	}
}

func TestStandaloneSign_RawPath(t *testing.T) {
	creds, err := unit.Config().Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("expect no error, got %v", err)
	}
	signer := v4.NewSigner()

	for _, c := range standaloneSignCases {
		host := fmt.Sprintf("https://%s.%s.%s.amazonaws.com",
			c.SubDomain, c.Region, c.Service)

		req, err := http.NewRequest("GET", host, nil)
		if err != nil {
			t.Errorf("expected no error, but received %v", err)
		}

		// URL.EscapedPath() will be used by the signer to get the
		// escaped form of the request's URI path.
		req.URL.Path = c.OrigURI
		req.URL.RawPath = c.EscapedURI
		req.URL.RawQuery = c.OrigQuery

		err = signer.SignHTTP(context.Background(), creds, req, v4Internal.EmptyStringSHA256, c.Service, c.Region, time.Unix(0, 0))
		if err != nil {
			t.Errorf("expected no error, but received %v", err)
		}

		actual := req.Header.Get("Authorization")
		if e, a := c.ExpSig, actual; e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
		if e, a := c.OrigURI, req.URL.Path; e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
		if e, a := c.EscapedURI, req.URL.EscapedPath(); e != a {
			t.Errorf("expected %v, but recieved %v", e, a)
		}
	}
}
