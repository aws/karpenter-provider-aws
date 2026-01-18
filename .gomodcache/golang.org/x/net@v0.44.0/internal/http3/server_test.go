// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.24 && goexperiment.synctest

package http3

import (
	"net/netip"
	"testing"
	"testing/synctest"

	"golang.org/x/net/internal/quic/quicwire"
	"golang.org/x/net/quic"
)

func TestServerReceivePushStream(t *testing.T) {
	// "[...] if a server receives a client-initiated push stream,
	// this MUST be treated as a connection error of type H3_STREAM_CREATION_ERROR."
	// https://www.rfc-editor.org/rfc/rfc9114.html#section-6.2.2-3
	runSynctest(t, func(t testing.TB) {
		ts := newTestServer(t)
		tc := ts.connect()
		tc.newStream(streamTypePush)
		tc.wantClosed("invalid client-created push stream", errH3StreamCreationError)
	})
}

func TestServerCancelPushForUnsentPromise(t *testing.T) {
	runSynctest(t, func(t testing.TB) {
		ts := newTestServer(t)
		tc := ts.connect()
		tc.greet()

		const pushID = 100
		tc.control.writeVarint(int64(frameTypeCancelPush))
		tc.control.writeVarint(int64(quicwire.SizeVarint(pushID)))
		tc.control.writeVarint(pushID)
		tc.control.Flush()

		tc.wantClosed("client canceled never-sent push ID", errH3IDError)
	})
}

type testServer struct {
	t  testing.TB
	s  *Server
	tn testNet
	*testQUICEndpoint

	addr netip.AddrPort
}

type testQUICEndpoint struct {
	t testing.TB
	e *quic.Endpoint
}

func (te *testQUICEndpoint) dial() {
}

type testServerConn struct {
	ts *testServer

	*testQUICConn
	control *testQUICStream
}

func newTestServer(t testing.TB) *testServer {
	t.Helper()
	ts := &testServer{
		t: t,
		s: &Server{
			Config: &quic.Config{
				TLSConfig: testTLSConfig,
			},
		},
	}
	e := ts.tn.newQUICEndpoint(t, ts.s.Config)
	ts.addr = e.LocalAddr()
	go ts.s.Serve(e)
	return ts
}

func (ts *testServer) connect() *testServerConn {
	ts.t.Helper()
	config := &quic.Config{TLSConfig: testTLSConfig}
	e := ts.tn.newQUICEndpoint(ts.t, nil)
	qconn, err := e.Dial(ts.t.Context(), "udp", ts.addr.String(), config)
	if err != nil {
		ts.t.Fatal(err)
	}
	tc := &testServerConn{
		ts:           ts,
		testQUICConn: newTestQUICConn(ts.t, qconn),
	}
	synctest.Wait()
	return tc
}

// greet performs initial connection handshaking with the server.
func (tc *testServerConn) greet() {
	// Client creates a control stream.
	tc.control = tc.newStream(streamTypeControl)
	tc.control.writeVarint(int64(frameTypeSettings))
	tc.control.writeVarint(0) // size
	tc.control.Flush()
	synctest.Wait()
}
