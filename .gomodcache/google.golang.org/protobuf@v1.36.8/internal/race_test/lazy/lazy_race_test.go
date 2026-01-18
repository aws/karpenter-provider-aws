// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This test tests that races on lazy fields in opaque protos are detected by the race detector,
// even though the plain code uses atomic variables in a manner that would hide data races.
// This is essential, as concurrent writes or read-writes on a lazy field can cause undefined
// behaviours.
//
// Using exectest with the race detector to check that the code fails did not work,
// as the race error got propagated from the subprocess and failed the test case in the parent process.
// Instead we create the subprocess where the test is supposed to fail by ourselves.

// Lazy decoding is only available in the fast path, which the protoreflect tag disables.
//go:build !protoreflect

package lazy_race_test

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"google.golang.org/protobuf/internal/test/race"
	mixedpb "google.golang.org/protobuf/internal/testprotos/mixed"
	testopaquepb "google.golang.org/protobuf/internal/testprotos/testeditions/testeditions_opaque"
	"google.golang.org/protobuf/proto"
)

// To get some output from the subprocess, set this to true
const debug = false

func makeM2() *testopaquepb.TestAllTypes {
	return testopaquepb.TestAllTypes_builder{
		OptionalLazyNestedMessage: testopaquepb.TestAllTypes_NestedMessage_builder{
			A: proto.Int32(1),
			Corecursive: testopaquepb.TestAllTypes_builder{
				OptionalBool: proto.Bool(true),
			}.Build(),
		}.Build(),
		RepeatedNestedMessage: []*testopaquepb.TestAllTypes_NestedMessage{
			testopaquepb.TestAllTypes_NestedMessage_builder{
				A: proto.Int32(2),
				Corecursive: testopaquepb.TestAllTypes_builder{
					OptionalInt32: proto.Int32(32),
				}.Build(),
			}.Build(),
		},
	}.Build()
}

type testC struct {
	name string
	l1   func()
	l2   func()
}

const envVar = "GO_TESTING_IN_SUBPROCESS"

// TestRaceDetectionOnWrite tests that any combination involving concurrent
// read-write or write-write will trigger the race detector.
func TestRaceDetectionOnWrite(t *testing.T) {
	var x *testopaquepb.TestAllTypes
	var y *testopaquepb.TestAllTypes_NestedMessage
	var z int32
	// A table of test cases to expose to the race detector.
	// The name will be set in an environment variable, so don't use special characters or spaces.
	// Each entry in the table will be spawned into a sub process, where the actual execution will happen.
	cases := []testC{
		{
			name: "TestSetSet",
			l1:   func() { x.SetOptionalLazyNestedMessage(y) },
			l2:   func() { x.SetOptionalLazyNestedMessage(y) },
		},
		{
			name: "TestClearClear",
			l1:   func() { x.ClearOptionalLazyNestedMessage() },
			l2:   func() { x.ClearOptionalLazyNestedMessage() },
		},
		{
			name: "TestSetClear",
			l1:   func() { x.SetOptionalLazyNestedMessage(y) },
			l2:   func() { x.ClearOptionalLazyNestedMessage() },
		},
		{
			name: "TestSetGet",
			l1:   func() { x.SetOptionalLazyNestedMessage(y) },
			l2: func() {
				if x.GetOptionalLazyNestedMessage().GetCorecursive().GetOptionalBool() {
					z++
				}
			},
		},
		{
			name: "TestSetHas",
			l1:   func() { x.SetOptionalLazyNestedMessage(y) },
			l2: func() {
				if x.HasOptionalLazyNestedMessage() {
					z++
				}
			},
		},
		{
			name: "TestClearGet",
			l1:   func() { x.ClearOptionalLazyNestedMessage() },
			l2: func() {
				if x.GetOptionalLazyNestedMessage().GetCorecursive().GetOptionalBool() {
					z++
				}
			},
		},
		{
			name: "TestClearHas",
			l1:   func() { x.ClearOptionalLazyNestedMessage() },
			l2: func() {
				if x.HasOptionalLazyNestedMessage() {
					z++
				}
			},
		},
	}
	e := os.Getenv(envVar)
	if e != "" {
		// We're in the subprocess. As spawnCase will add filter for the subtest,
		// we will actually only execute one test in this subprocess even though
		// we call t.Run for all cases.
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				x = makeM2()
				y = x.GetOptionalLazyNestedMessage()
				z = 0
				execCase(t, tc)
				return
			})
		}
		return
	}
	// If we're not in a subprocess, spawn and check one for each entry in the table
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spawnCase(t)
		})
	}
}

// execCase actually executes the testcase when we're in a subprocess, it executes
// the two operations of tc in parallel and make sure tsan sees this as parallel
// execution.
func execCase(t *testing.T, tc testC) {
	t.Helper()
	c1 := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)
	// This is a very complicated but stable way of telling tsan that the
	// two operations are executed in parallel. I can only guess why this
	// works so I'll leave my speculations out of the comment but
	// experiments suggest that it works reliably.
	go func() {
		c1 <- struct{}{}
		tc.l1()
		<-c1
		tc.l1()
		wg.Done()
	}()
	go func() {
		<-c1
		tc.l2()
		c1 <- struct{}{}
		tc.l2()
		wg.Done()
	}()
	wg.Wait()
}

// spawnCase reruns this executable to execute t.Name() with the sub-case tn in the environment variable
func spawnCase(t *testing.T) {
	// If we get here, we are in the parent process and should execute ourselves, but filter on the test that called us.
	ep, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to find my own executable: %v", err)
	}
	c := exec.Command(ep, "--test.run="+t.Name())
	// Set the environment variable so that we know we're in a subproceess when re-executed
	c.Env = append(c.Env, envVar+"=true")
	out, err := c.CombinedOutput()
	// If we do not get an error, we fail in the parent process, otherwise we're good
	if race.Enabled && err == nil {
		t.Errorf("Got success, want error under race detector:\n-----------\n%s\n-------------\n", string(out))
	}
	if !race.Enabled && err != nil {
		t.Errorf("Got error, want success without race detector:\n-----------\n%s\n-------------\n", string(out))
	}
	if debug {
		fmt.Fprintf(os.Stderr, "Subprocess output:\n-----------\n%s\n-------------\n", string(out))
	}
}

// TestNoRaceDetection should not fail under race detector (or otherwise)
func TestNoRaceDetection(t *testing.T) {
	x := makeM2()
	var y int32
	var z int32
	c := make(chan struct{})
	go func() {
		for i := 0; i < 10000; i++ {
			y += x.GetRepeatedNestedMessage()[0].GetA()
		}
		close(c)
	}()
	for i := 0; i < 10000; i++ {
		z += x.GetRepeatedNestedMessage()[0].GetA()
	}
	<-c
	if z != y {
		t.Errorf("The two go-routines did not calculate the same: %d != %d", z, y)
	}
}

func TestNoRaceOnGetsOfSlices(t *testing.T) {
	x := makeM2()
	b, err := proto.Marshal(x)
	if err != nil {
		t.Fatalf("Error while marshaling: %v", err)
	}

	var y int32
	var z int32
	d := make(chan int)

	// Check that there are no races when we do concurrent lazy gets of a field
	// containing a slice of message pointers.
	for i := 0; i < 10000; i++ {
		err := proto.Unmarshal(b, x)
		if err != nil {
			t.Fatalf("Error while unmarshaling: %v", err)
		}
		go func() {
			y += x.GetRepeatedNestedMessage()[0].GetA()
			d <- 1
		}()
		go func() {
			z += x.GetRepeatedNestedMessage()[0].GetA()
			d <- 1
		}()
		<-d
		<-d
	}
	if z != y {
		t.Errorf("The two go-routines did not calculate the same: %d != %d", z, y)
	}
	close(d)
}

func TestNoRaceOnGetsOfMessages(t *testing.T) {
	x := makeM2()
	b, err := proto.Marshal(x)
	if err != nil {
		t.Fatalf("Error while marshaling: %v", err)
	}

	var y int32
	var z int32
	d := make(chan int)

	// Check that there is no race when we do concurrent lazy gets of a field
	// pointing to a sub-message.
	for i := 0; i < 10000; i++ {
		err := proto.Unmarshal(b, x)
		if err != nil {
			t.Fatalf("Error while unmarshaling: %v", err)
		}
		go func() {
			if x.GetOptionalLazyNestedMessage().GetA() > 0 {
				y++
			}
			d <- 1
		}()
		go func() {
			if x.GetOptionalLazyNestedMessage().GetA() > 0 {
				z++
			}
			d <- 1
		}()
		<-d
		<-d
	}
	if z != y {
		t.Errorf("The two go-routines did not calculate the same: %d != %d", z, y)
	}

	close(d)
}

func fillRequiredLazy() *testopaquepb.TestRequiredLazy {
	return testopaquepb.TestRequiredLazy_builder{
		OptionalLazyMessage: testopaquepb.TestRequired_builder{
			RequiredField: proto.Int32(23),
		}.Build(),
	}.Build()
}

func expandedLazy(m *testopaquepb.TestRequiredLazy) bool {
	v := reflect.ValueOf(m).Elem()
	rf := v.FieldByName("xxx_hidden_OptionalLazyMessage")
	rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
	return rf.Pointer() != 0
}

// This test verifies all assumptions of TestParallellMarshalWithRequired
// are (still) valid, to prevent the test from becoming a no-op (again).
func TestParallellMarshalWithRequiredAssumptions(t *testing.T) {
	b, err := proto.Marshal(fillRequiredLazy())
	if err != nil {
		t.Fatal(err)
	}

	ml := &testopaquepb.TestRequiredLazy{}
	// Specifying AllowPartial: true at unmarshal time is required, otherwise
	// the Marshal call will skip the required field check.
	if err := (proto.UnmarshalOptions{AllowPartial: true}).Unmarshal(b, ml); err != nil {
		t.Fatal(err)
	}
	if expandedLazy(ml) {
		t.Fatalf("lazy message unexpectedly decoded")
	}

	// Marshaling with AllowPartial: true means the no decoding is needed,
	// because no required field checks are done.
	if _, err := (proto.MarshalOptions{AllowPartial: true}).Marshal(ml); err != nil {
		t.Fatal(err)
	}
	if expandedLazy(ml) {
		t.Fatalf("lazy message unexpectedly decoded")
	}

	// Whereas marshaling with AllowPartial: false (default) means the message
	// will be decoded to check if any required fields are not set.
	if _, err := (proto.MarshalOptions{AllowPartial: false}).Marshal(ml); err != nil {
		t.Fatal(err)
	}
	if !expandedLazy(ml) {
		t.Fatalf("lazy message unexpectedly not decoded")
	}
}

// TestParallellMarshalWithRequired runs two goroutines that marshal the same
// message. Marshaling a message can result in lazily decoding said message,
// provided the message contains any required fields. This test ensures that
// said lazy decoding can happen without causing races in the other goroutine
// that marshals the same message.
func TestParallellMarshalWithRequired(t *testing.T) {
	m := fillRequiredLazy()
	b, err := proto.MarshalOptions{}.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	partial := false
	for i := 0; i < 1000; i++ {
		partial = !partial
		ml := &testopaquepb.TestRequiredLazy{}
		d := make(chan bool)
		err := proto.UnmarshalOptions{AllowPartial: true}.Unmarshal(b, ml)
		if err != nil {
			t.Fatalf("Error while unmarshaling: %v", err)
		}

		go func() {
			b2, err := proto.MarshalOptions{AllowPartial: partial}.Marshal(ml)
			if err != nil {
				t.Errorf("Marshal error: %v", err)
				d <- false
				return
			}
			m := &testopaquepb.TestRequiredLazy{}
			if err := (proto.UnmarshalOptions{}).Unmarshal(b2, m); err != nil {
				t.Errorf("Unmarshal error: %v", err)
				d <- false
				return
			}
			if !proto.Equal(ml, m) {
				t.Errorf("Unmarshal roundtrip - protos not equal")
				d <- false
				return
			}
			d <- true
		}()
		go func() {
			b2, err := proto.MarshalOptions{AllowPartial: partial}.Marshal(ml)
			if err != nil {
				t.Errorf("Marshal error: %v", err)
				d <- false
				return
			}
			m := &testopaquepb.TestRequiredLazy{}
			if err := (proto.UnmarshalOptions{}).Unmarshal(b2, m); err != nil {
				if !proto.Equal(ml, m) {
					t.Errorf("Unmarshal roundtrip - protos not equal")
					d <- false
					return
				}
				if !proto.Equal(ml, m) {
					t.Errorf("Unmarshal roundtrip - protos not equal")
					d <- false
					return
				}
			}
			d <- true
		}()
		x := <-d
		y := <-d
		if !x || !y {
			t.Fatalf("Worker reported error")
		}
	}
}

func fillMixedOpaqueLazy() *mixedpb.OpaqueLazy {
	return mixedpb.OpaqueLazy_builder{
		Opaque: mixedpb.OpaqueLazy_builder{
			OptionalInt32: proto.Int32(23),
			Hybrid: mixedpb.HybridLazy_builder{
				OptionalInt32: proto.Int32(42),
			}.Build(),
		}.Build(),
		Hybrid: mixedpb.HybridLazy_builder{
			OptionalInt32: proto.Int32(5),
		}.Build(),
	}.Build()
}

func TestParallellMarshalMixed(t *testing.T) {
	m := fillMixedOpaqueLazy()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		ml := &mixedpb.OpaqueLazy{}
		d := make(chan bool)
		if err := proto.Unmarshal(b, ml); err != nil {
			t.Fatalf("Error while unmarshaling: %v", err)
		}

		go func() {
			b2, err := proto.Marshal(ml)
			if err != nil {
				t.Errorf("Marshal error: %v", err)
				d <- false
				return
			}
			m := &mixedpb.OpaqueLazy{}
			if err := proto.Unmarshal(b2, m); err != nil {
				t.Errorf("Unmarshal error: %v", err)
				d <- false
				return
			}
			if !proto.Equal(ml, m) { // This is what expands all fields of ml
				t.Errorf("Unmarshal roundtrip - protos not equal")
				d <- false
				return
			}
			d <- true
		}()
		go func() {
			b2, err := proto.Marshal(ml)
			if err != nil {
				t.Errorf("Marshal error: %v", err)
				d <- false
				return
			}
			m := &mixedpb.OpaqueLazy{}
			if err := proto.Unmarshal(b2, m); err != nil {
				t.Errorf("Unmarshal error: %v", err)
				d <- false
				return
			}
			if !proto.Equal(ml, m) { // This is what expands all fields of ml
				t.Errorf("Unmarshal roundtrip - protos not equal")
				d <- false
				return
			}
			d <- true
		}()
		x := <-d
		y := <-d
		if !x || !y {
			t.Fatalf("Worker reported error")
		}
	}
}
