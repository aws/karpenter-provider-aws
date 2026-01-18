// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Whether a function is inlineable is dependent on the Go compiler version
// and also relies on the presence of the Go toolchain itself being installed.
// This test is disabled by default and explicitly enabled with an
// environment variable that is specified in our integration tests,
// which have fine control over exactly which Go version is being tested.
var testInline = os.Getenv("TEST_INLINE") != ""

func TestInline(t *testing.T) {
	if !testInline {
		t.SkipNow()
	}

	fncs := func() map[string]bool {
		m := make(map[string]bool)
		for _, s := range []string{
			"Encoder.needFlush",
			"Decoder.ReadValue", // thin wrapper over Decoder.readValue
			"decodeBuffer.needMore",
			"consumeWhitespace",
			"consumeNull",
			"consumeFalse",
			"consumeTrue",
			"consumeSimpleString",
			"consumeString", // thin wrapper over consumeStringResumable
			"consumeSimpleNumber",
			"consumeNumber",         // thin wrapper over consumeNumberResumable
			"unescapeStringMayCopy", // thin wrapper over unescapeString
			"hasSuffixByte",
			"trimSuffixByte",
			"trimSuffixString",
			"trimSuffixWhitespace",
			"stateMachine.appendLiteral",
			"stateMachine.appendNumber",
			"stateMachine.appendString",
			"stateMachine.depth",
			"stateMachine.reset",
			"stateMachine.mayAppendDelim",
			"stateMachine.needDelim",
			"stateMachine.popArray",
			"stateMachine.popObject",
			"stateMachine.pushArray",
			"stateMachine.pushObject",
			"stateEntry.increment",
			"stateEntry.decrement",
			"stateEntry.isArray",
			"stateEntry.isObject",
			"stateEntry.length",
			"stateEntry.needImplicitColon",
			"stateEntry.needImplicitComma",
			"stateEntry.needObjectName",
			"stateEntry.needObjectValue",
			"objectNameStack.reset",
			"objectNameStack.length",
			"objectNameStack.getUnquoted",
			"objectNameStack.push",
			"objectNameStack.replaceLastQuotedOffset",
			"objectNameStack.replaceLastUnquotedName",
			"objectNameStack.pop",
			"objectNameStack.ensureCopiedBuffer",
			"objectNamespace.insertQuoted",   // thin wrapper over objectNamespace.insert
			"objectNamespace.insertUnquoted", // thin wrapper over objectNamespace.insert
			"Token.String",                   // thin wrapper over Token.string
			"foldName",                       // thin wrapper over appendFoldedName
			"hash64",
		} {
			m[s] = true
		}
		return m
	}()

	cmd := exec.Command("go", "build", "-gcflags=-m")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec.Command error: %v\n\n%s", err, b)
	}
	for _, line := range strings.Split(string(b), "\n") {
		const phrase = ": can inline "
		if i := strings.Index(line, phrase); i >= 0 {
			fnc := line[i+len(phrase):]
			fnc = strings.ReplaceAll(fnc, "(", "")
			fnc = strings.ReplaceAll(fnc, "*", "")
			fnc = strings.ReplaceAll(fnc, ")", "")
			delete(fncs, fnc)
		}
	}
	for fnc := range fncs {
		t.Errorf("%v is not inlineable, expected it to be", fnc)
	}
}
