// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protopack"

	lnwtpb "google.golang.org/protobuf/internal/testprotos/lazy"
)

func unmarshalsTheSame(b []byte, expected *lnwtpb.FTop) error {
	unmarshaledTop := &lnwtpb.FTop{}
	if err := proto.Unmarshal(b, unmarshaledTop); err != nil {
		return err
	}
	if !proto.Equal(unmarshaledTop, expected) {
		return fmt.Errorf("!proto.Equal")
	}
	return nil
}

func bytesTag(num protowire.Number) protopack.Tag {
	return protopack.Tag{
		Number: num,
		Type:   protopack.BytesType,
	}
}

func varintTag(num protowire.Number) protopack.Tag {
	return protopack.Tag{
		Number: num,
		Type:   protopack.VarintType,
	}
}

// Constructs a message encoded in denormalized (non-minimal) wire format, but
// using two levels of nesting: A top-level message with a child message which
// in turn has a grandchild message.
func denormalizedTwoLevelField() ([]byte, *lnwtpb.FTop, error) {
	expectedMessage := &lnwtpb.FTop{
		A: proto.Uint32(2342),
		Child: &lnwtpb.FSub{
			Grandchild: &lnwtpb.FSub{
				B: proto.Uint32(1337),
			},
		},
	}

	fullMessage := protopack.Message{
		varintTag(1), protopack.Varint(2342),
		// Child
		bytesTag(2), protopack.LengthPrefix(protopack.Message{
			// Grandchild
			bytesTag(4), protopack.LengthPrefix(protopack.Message{
				// The first occurrence of B matches expectedMessage:
				varintTag(2), protopack.Varint(1337),
				// This second duplicative occurrence of B is spec'd in Protobuf:
				// https://github.com/protocolbuffers/protobuf/issues/9257
				varintTag(2), protopack.Varint(1337),
			}),
		}),
	}.Marshal()

	return fullMessage, expectedMessage, nil
}

func TestInvalidWireFormat(t *testing.T) {
	fullMessage, expectedMessage, err := denormalizedTwoLevelField()
	if err != nil {
		t.Fatal(err)
	}

	top := &lnwtpb.FTop{}
	if err := proto.Unmarshal(fullMessage, top); err != nil {
		t.Fatal(err)
	}

	// Access the top-level submessage, but not the grandchild.
	// This populates the size cache in the top-level message.
	top.GetChild()

	marshal1, err := proto.MarshalOptions{
		UseCachedSize: true,
	}.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	if err := unmarshalsTheSame(marshal1, expectedMessage); err != nil {
		t.Error(err)
	}

	// Call top.GetChild().GetGrandchild() to unmarshal the lazy message,
	// which will normalize it: the size cache shrinks from 6 bytes to 3.
	// Notably, top.GetChild()’s size cache is not updated!
	top.GetChild().GetGrandchild()
	marshal2, err := proto.MarshalOptions{
		// GetGrandchild+UseCachedSize is one way to trigger this bug.
		// The other way is to call GetGrandchild in another goroutine,
		// after proto.Marshal has called proto.Size but
		// before proto.Marshal started encoding.
		UseCachedSize: true,
	}.Marshal(top)
	if err != nil {
		if strings.Contains(err.Error(), "size mismatch") {
			// This is the expected failure mode: proto.Marshal() detects the
			// combination of non-minimal wire format and lazy decoding and
			// returns an error, prompting the user to disable lazy decoding.
			return
		}
		t.Fatal(err)
	}
	if err := unmarshalsTheSame(marshal2, expectedMessage); err != nil {
		t.Error(err)
	}
}

func TestIdenticalOverAccessWhenDeterministic(t *testing.T) {
	fullMessage, _, err := denormalizedTwoLevelField()
	if err != nil {
		t.Fatal(err)
	}

	top := &lnwtpb.FTop{}
	if err := proto.Unmarshal(fullMessage, top); err != nil {
		t.Fatal(err)
	}

	deterministic := proto.MarshalOptions{
		Deterministic: true,
	}
	marshal1, err := deterministic.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}

	// Call top.GetChild().GetGrandchild() to unmarshal the lazy message,
	// which will normalize it.
	top.GetChild().GetGrandchild()

	marshal2, err := deterministic.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(marshal1, marshal2) {
		t.Errorf("MarshalOptions{Deterministic: true}.Marshal() not identical over accessing a non-minimal wire format lazy message:\nbefore:\n%x\nafter:\n%x", marshal1, marshal2)
	}
}
