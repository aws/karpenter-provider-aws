// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.24

package http3

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// A bodyWriter writes a request or response body to a stream
// as a series of DATA frames.
type bodyWriter struct {
	st     *stream
	remain int64  // -1 when content-length is not known
	flush  bool   // flush the stream after every write
	name   string // "request" or "response"
}

func (w *bodyWriter) Write(p []byte) (n int, err error) {
	if w.remain >= 0 && int64(len(p)) > w.remain {
		return 0, &streamError{
			code:    errH3InternalError,
			message: w.name + " body longer than specified content length",
		}
	}
	w.st.writeVarint(int64(frameTypeData))
	w.st.writeVarint(int64(len(p)))
	n, err = w.st.Write(p)
	if w.remain >= 0 {
		w.remain -= int64(n)
	}
	if w.flush && err == nil {
		err = w.st.Flush()
	}
	if err != nil {
		err = fmt.Errorf("writing %v body: %w", w.name, err)
	}
	return n, err
}

func (w *bodyWriter) Close() error {
	if w.remain > 0 {
		return errors.New(w.name + " body shorter than specified content length")
	}
	return nil
}

// A bodyReader reads a request or response body from a stream.
type bodyReader struct {
	st *stream

	mu     sync.Mutex
	remain int64
	err    error
}

func (r *bodyReader) Read(p []byte) (n int, err error) {
	// The HTTP/1 and HTTP/2 implementations both permit concurrent reads from a body,
	// in the sense that the race detector won't complain.
	// Use a mutex here to provide the same behavior.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	defer func() {
		if err != nil {
			r.err = err
		}
	}()
	if r.st.lim == 0 {
		// We've finished reading the previous DATA frame, so end it.
		if err := r.st.endFrame(); err != nil {
			return 0, err
		}
	}
	// Read the next DATA frame header,
	// if we aren't already in the middle of one.
	for r.st.lim < 0 {
		ftype, err := r.st.readFrameHeader()
		if err == io.EOF && r.remain > 0 {
			return 0, &streamError{
				code:    errH3MessageError,
				message: "body shorter than content-length",
			}
		}
		if err != nil {
			return 0, err
		}
		switch ftype {
		case frameTypeData:
			if r.remain >= 0 && r.st.lim > r.remain {
				return 0, &streamError{
					code:    errH3MessageError,
					message: "body longer than content-length",
				}
			}
			// Fall out of the loop and process the frame body below.
		case frameTypeHeaders:
			// This HEADERS frame contains the message trailers.
			if r.remain > 0 {
				return 0, &streamError{
					code:    errH3MessageError,
					message: "body shorter than content-length",
				}
			}
			// TODO: Fill in Request.Trailer.
			if err := r.st.discardFrame(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		default:
			if err := r.st.discardUnknownFrame(ftype); err != nil {
				return 0, err
			}
		}
	}
	// We are now reading the content of a DATA frame.
	// Fill the read buffer or read to the end of the frame,
	// whichever comes first.
	if int64(len(p)) > r.st.lim {
		p = p[:r.st.lim]
	}
	n, err = r.st.Read(p)
	if r.remain > 0 {
		r.remain -= int64(n)
	}
	return n, err
}

func (r *bodyReader) Close() error {
	// Unlike the HTTP/1 and HTTP/2 body readers (at the time of this comment being written),
	// calling Close concurrently with Read will interrupt the read.
	r.st.stream.CloseRead()
	return nil
}
