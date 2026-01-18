// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.24

package http3

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/net/quic"
)

// A Server is an HTTP/3 server.
// The zero value for Server is a valid server.
type Server struct {
	// Handler to invoke for requests, http.DefaultServeMux if nil.
	Handler http.Handler

	// Config is the QUIC configuration used by the server.
	// The Config may be nil.
	//
	// ListenAndServe may clone and modify the Config.
	// The Config must not be modified after calling ListenAndServe.
	Config *quic.Config

	initOnce sync.Once
}

func (s *Server) init() {
	s.initOnce.Do(func() {
		s.Config = initConfig(s.Config)
		if s.Handler == nil {
			s.Handler = http.DefaultServeMux
		}
	})
}

// ListenAndServe listens on the UDP network address addr
// and then calls Serve to handle requests on incoming connections.
func (s *Server) ListenAndServe(addr string) error {
	s.init()
	e, err := quic.Listen("udp", addr, s.Config)
	if err != nil {
		return err
	}
	return s.Serve(e)
}

// Serve accepts incoming connections on the QUIC endpoint e,
// and handles requests from those connections.
func (s *Server) Serve(e *quic.Endpoint) error {
	s.init()
	for {
		qconn, err := e.Accept(context.Background())
		if err != nil {
			return err
		}
		go newServerConn(qconn)
	}
}

type serverConn struct {
	qconn *quic.Conn

	genericConn // for handleUnidirectionalStream
	enc         qpackEncoder
	dec         qpackDecoder
}

func newServerConn(qconn *quic.Conn) {
	sc := &serverConn{
		qconn: qconn,
	}
	sc.enc.init()

	// Create control stream and send SETTINGS frame.
	// TODO: Time out on creating stream.
	controlStream, err := newConnStream(context.Background(), sc.qconn, streamTypeControl)
	if err != nil {
		return
	}
	controlStream.writeSettings()
	controlStream.Flush()

	sc.acceptStreams(sc.qconn, sc)
}

func (sc *serverConn) handleControlStream(st *stream) error {
	// "A SETTINGS frame MUST be sent as the first frame of each control stream [...]"
	// https://www.rfc-editor.org/rfc/rfc9114.html#section-7.2.4-2
	if err := st.readSettings(func(settingsType, settingsValue int64) error {
		switch settingsType {
		case settingsMaxFieldSectionSize:
			_ = settingsValue // TODO
		case settingsQPACKMaxTableCapacity:
			_ = settingsValue // TODO
		case settingsQPACKBlockedStreams:
			_ = settingsValue // TODO
		default:
			// Unknown settings types are ignored.
		}
		return nil
	}); err != nil {
		return err
	}

	for {
		ftype, err := st.readFrameHeader()
		if err != nil {
			return err
		}
		switch ftype {
		case frameTypeCancelPush:
			// "If a server receives a CANCEL_PUSH frame for a push ID
			// that has not yet been mentioned by a PUSH_PROMISE frame,
			// this MUST be treated as a connection error of type H3_ID_ERROR."
			// https://www.rfc-editor.org/rfc/rfc9114.html#section-7.2.3-8
			return &connectionError{
				code:    errH3IDError,
				message: "CANCEL_PUSH for unsent push ID",
			}
		case frameTypeGoaway:
			return errH3NoError
		default:
			// Unknown frames are ignored.
			if err := st.discardUnknownFrame(ftype); err != nil {
				return err
			}
		}
	}
}

func (sc *serverConn) handleEncoderStream(*stream) error {
	// TODO
	return nil
}

func (sc *serverConn) handleDecoderStream(*stream) error {
	// TODO
	return nil
}

func (sc *serverConn) handlePushStream(*stream) error {
	// "[...] if a server receives a client-initiated push stream,
	// this MUST be treated as a connection error of type H3_STREAM_CREATION_ERROR."
	// https://www.rfc-editor.org/rfc/rfc9114.html#section-6.2.2-3
	return &connectionError{
		code:    errH3StreamCreationError,
		message: "client created push stream",
	}
}

func (sc *serverConn) handleRequestStream(st *stream) error {
	// TODO
	return nil
}

// abort closes the connection with an error.
func (sc *serverConn) abort(err error) {
	if e, ok := err.(*connectionError); ok {
		sc.qconn.Abort(&quic.ApplicationError{
			Code:   uint64(e.code),
			Reason: e.message,
		})
	} else {
		sc.qconn.Abort(err)
	}
}
