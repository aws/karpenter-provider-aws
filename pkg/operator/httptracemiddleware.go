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

package operator

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
)

var (
	// connBacklog tracks the number of requests currently waiting to acquire a connection.
	// Incremented in GetConn, decremented in GotConn.
	// A sustained high value indicates connection pool saturation.
	connBacklog atomic.Int64
)

// traceLogger returns a logr.Logger that carries only the traceID and operation,
// deliberately dropping controller-runtime metadata (controller, namespace, name, reconcileID)
// to keep aws-trace logs clean.
func traceLogger(traceID, operation string) logr.Logger {
	return log.Log.WithValues("traceID", traceID, "operation", operation)
}

// tracedOperations is the set of EC2 operations we want httptrace diagnostics on.
var tracedOperations = map[string]struct{}{
	"TerminateInstances": {},
	"DescribeInstances":  {},
}

// generateTraceID returns a short random hex string to correlate all log lines for a single request.
func generateTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// traceIDKey is a context key used to propagate the trace ID from the HTTP trace middleware
// to the SigV4 log middleware so both share the same correlation ID.
type traceIDKeyType struct{}

var traceIDKey = traceIDKeyType{}

// HTTPTraceMiddleware adds net/http/httptrace instrumentation to specific EC2 API calls
// so we can observe DNS, TLS, connection reuse, and timing at the HTTP/1.1 level.
// It also logs request headers and SigV4 signature details after signing is complete.
// All log lines for a single request share a traceID for easy correlation.
func HTTPTraceMiddleware(stack *smithymiddleware.Stack) error {
	if err := stack.Finalize.Add(&httpTraceMiddleware{}, smithymiddleware.Before); err != nil {
		return err
	}
	// Insert after signing so we can see the Authorization header and X-Amz-Date
	return stack.Finalize.Insert(&sigV4LogMiddleware{}, "Signing", smithymiddleware.After)
}

type httpTraceMiddleware struct{}

func (*httpTraceMiddleware) ID() string { return "HTTPTraceMiddleware" }

func (*httpTraceMiddleware) HandleFinalize(ctx context.Context, in smithymiddleware.FinalizeInput, next smithymiddleware.FinalizeHandler) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
	op := awsmiddleware.GetOperationName(ctx)
	if _, ok := tracedOperations[op]; !ok {
		return next.HandleFinalize(ctx, in)
	}

	traceID := generateTraceID()
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	logger := traceLogger(traceID, op)

	var (
		dnsStart      time.Time
		connStart     time.Time
		tlsStart      time.Time
		connWaitStart time.Time
		gotFirstByte  time.Time
		// Track connection attempts to detect retries/reconnects within a single SDK call
		connAttempt atomic.Int32
		// Track the first remote address to detect endpoint changes (e.g. DNS failover)
		firstRemoteAddr atomic.Value
	)

	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			connWaitStart = time.Now()
			waiting := connBacklog.Add(1)
			logger.V(1).Info("[aws-trace] waiting for connection", "hostPort", hostPort, "connBacklog", waiting)
		},
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dur := time.Since(dnsStart)
			if info.Err != nil {
				logger.V(1).Info("[aws-trace] DNS lookup failed", "dnsLatency", dur, "error", info.Err)
			} else {
				addrs := make([]string, len(info.Addrs))
				for i, a := range info.Addrs {
					addrs[i] = a.String()
				}
				logger.V(1).Info("[aws-trace] DNS resolved", "dnsLatency", dur, "resolvedAddrs", addrs)
			}
		},
		ConnectStart: func(network, addr string) {
			attempt := connAttempt.Add(1)
			connStart = time.Now()
			logger.V(1).Info("[aws-trace] TCP connecting", "network", network, "remoteAddr", addr, "connAttempt", attempt)
		},
		ConnectDone: func(network, addr string, err error) {
			dur := time.Since(connStart)
			if err != nil {
				logger.V(1).Info("[aws-trace] TCP connect failed", "network", network, "remoteAddr", addr, "connectLatency", dur, "error", err)
			} else {
				logger.V(1).Info("[aws-trace] TCP connected", "network", network, "remoteAddr", addr, "connectLatency", dur)
			}
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			dur := time.Since(tlsStart)
			if err != nil {
				logger.V(1).Info("[aws-trace] TLS handshake failed", "tlsLatency", dur, "error", err)
			} else {
				logger.V(1).Info("[aws-trace] TLS handshake complete",
					"tlsLatency", dur,
					"tlsVersion", fmt.Sprintf("0x%04x", state.Version),
					"cipherSuite", tls.CipherSuiteName(state.CipherSuite),
				)
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			backlog := connBacklog.Add(-1)
			connWait := time.Since(connWaitStart)
			remoteAddr := info.Conn.RemoteAddr().String()
			endpointChanged := false
			if prev, ok := firstRemoteAddr.Load().(string); ok && prev != "" {
				endpointChanged = prev != remoteAddr
			} else {
				firstRemoteAddr.Store(remoteAddr)
			}
			logger.V(1).Info("[aws-trace] connection acquired",
				"connReused", info.Reused,
				"wasIdle", info.WasIdle,
				"idleTime", info.IdleTime,
				"connWait", connWait,
				"connBacklog", backlog,
				"localAddr", info.Conn.LocalAddr(),
				"remoteAddr", remoteAddr,
				"endpointChanged", endpointChanged,
			)
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	ctx = httptrace.WithClientTrace(ctx, trace)
	start := time.Now()
	if req, ok := in.Request.(*smithyhttp.Request); ok {
		startKV := []any{"method", req.Method, "url", req.URL.String()}
		// Log connection pool configuration from the underlying transport
		if transport, ok := extractTransport(); ok {
			startKV = append(startKV,
				"maxIdleConns", transport.MaxIdleConns,
				"maxIdleConnsPerHost", transport.MaxIdleConnsPerHost,
				"maxConnsPerHost", transport.MaxConnsPerHost,
				"idleConnTimeout", transport.IdleConnTimeout,
				"tlsHandshakeTimeout", transport.TLSHandshakeTimeout,
				"expectContinueTimeout", transport.ExpectContinueTimeout,
			)
		}
		logger.V(1).Info("[aws-trace] >>> request start", startKV...)
	}

	out, md, err := next.HandleFinalize(ctx, in)
	total := time.Since(start)
	ttfb := time.Duration(0)
	if !gotFirstByte.IsZero() {
		ttfb = gotFirstByte.Sub(start)
	}

	reqID, _ := awsmiddleware.GetRequestIDMetadata(md)

	logKV := []any{
		"requestID", reqID,
		"totalLatency", total,
		"ttfb", ttfb,
		"connAttempts", connAttempt.Load(),
	}

	// Extract HTTP status code from the response
	if resp, ok := out.Result.(*smithyhttp.Response); ok && resp != nil {
		logKV = append(logKV, "httpStatus", resp.StatusCode)
	}

	// Log clock skew if detected by the SDK
	if skew, ok := awsmiddleware.GetAttemptSkew(md); ok && skew != 0 {
		logKV = append(logKV, "clockSkew", skew)
	}

	if err != nil {
		logger.V(1).Info("[aws-trace] <<< request FAILED", append(logKV, "error", err)...)
	} else {
		logger.V(1).Info("[aws-trace] <<< request OK", logKV...)
	}
	return out, md, err
}

// extractTransport returns the *http.Transport the SDK is actually using.
// The AWS SDK v2 defaults to http.DefaultTransport unless a custom HTTPClient is set.
// This lets us log the real connection pool settings without overriding anything.
func extractTransport() (*http.Transport, bool) {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		return t, true
	}
	return nil, false
}

// sigV4LogMiddleware logs request headers and SigV4 signature details after signing.
type sigV4LogMiddleware struct{}

func (*sigV4LogMiddleware) ID() string { return "SigV4LogMiddleware" }

func (*sigV4LogMiddleware) HandleFinalize(ctx context.Context, in smithymiddleware.FinalizeInput, next smithymiddleware.FinalizeHandler) (smithymiddleware.FinalizeOutput, smithymiddleware.Metadata, error) {
	op := awsmiddleware.GetOperationName(ctx)
	if _, ok := tracedOperations[op]; !ok {
		return next.HandleFinalize(ctx, in)
	}

	// Recover the traceID set by httpTraceMiddleware so all lines correlate
	traceID, _ := ctx.Value(traceIDKey).(string)
	logger := traceLogger(traceID, op)

	if req, ok := in.Request.(*smithyhttp.Request); ok {
		amzDate := req.Header.Get("X-Amz-Date")
		authHeader := req.Header.Get("Authorization")
		amzSecurityToken := req.Header.Get("X-Amz-Security-Token")
		// amz-sdk-request contains attempt number and ttl, e.g. "attempt=1; max=3"
		sdkRequestHeader := req.Header.Get("Amz-Sdk-Request")

		logKV := []any{
			"host", req.Header.Get("Host"),
			"contentType", req.Header.Get("Content-Type"),
			"amzDate", amzDate,
			"hasSecurityToken", amzSecurityToken != "",
			"sdkRequest", sdkRequestHeader,
		}

		// Parse the SigV4 Authorization header to extract credential scope and signed headers
		// Format: AWS4-HMAC-SHA256 Credential=.../20260302/us-west-2/ec2/aws4_request, SignedHeaders=..., Signature=...
		if strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
			for _, part := range strings.Split(authHeader, ", ") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "AWS4-HMAC-SHA256 Credential=") {
					logKV = append(logKV, "credential", strings.TrimPrefix(part, "AWS4-HMAC-SHA256 "))
				} else if strings.HasPrefix(part, "Credential=") {
					logKV = append(logKV, "credential", part)
				} else if strings.HasPrefix(part, "SignedHeaders=") {
					logKV = append(logKV, "signedHeaders", strings.TrimPrefix(part, "SignedHeaders="))
				} else if strings.HasPrefix(part, "Signature=") {
					// Log only first 12 chars of signature for identification without leaking full sig
					sig := strings.TrimPrefix(part, "Signature=")
					if len(sig) > 12 {
						sig = sig[:12] + "..."
					}
					logKV = append(logKV, "signaturePrefix", sig)
				}
			}
		}

		// Log the signing timestamp vs current wall clock to detect clock drift or credential staleness
		if amzDate != "" {
			if signTime, parseErr := time.Parse("20060102T150405Z", amzDate); parseErr == nil {
				signAge := time.Since(signTime)
				logKV = append(logKV, "signAge", signAge)
				// Flag if sign age is suspiciously high (>5s could indicate credential caching or clock issues)
				if signAge > 5*time.Second {
					logKV = append(logKV, "signAgeWarning", "sign age exceeds 5s, possible clock drift or stale credentials")
				}
			}
		}
		logger.V(1).Info("[aws-trace] SigV4 signing details", logKV...)
	}
	return next.HandleFinalize(ctx, in)
}
