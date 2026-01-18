/*
Copyright 2014 Google Inc. All rights reserved.
Copyright 2014 The gofuzz Authors.
Copyright 2025 The Kubernetes Authors.

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

package randfill

import (
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFillBasic(t *testing.T) {
	obj := struct {
		I    int
		I8   int8
		I16  int16
		I32  int32
		I64  int64
		U    uint
		U8   uint8
		U16  uint16
		U32  uint32
		U64  uint64
		Uptr uintptr
		S    string
		B    bool
		T    time.Time
		C64  complex64
		C128 complex128
	}{}

	tryFill(t, New(), &obj, func() Stages {
		s := DeclareStages(16)
		s.Stage(0, obj.I != 0)
		s.Stage(1, obj.I8 != 0)
		s.Stage(2, obj.I16 != 0)
		s.Stage(3, obj.I32 != 0)
		s.Stage(4, obj.I64 != 0)
		s.Stage(5, obj.U != 0)
		s.Stage(6, obj.U8 != 0)
		s.Stage(7, obj.U16 != 0)
		s.Stage(8, obj.U32 != 0)
		s.Stage(9, obj.U64 != 0)
		s.Stage(10, obj.Uptr != 0)
		s.Stage(11, obj.S != "")
		s.Stage(12, obj.B == true)
		s.Stage(13, !obj.T.IsZero())
		s.Stage(14, obj.C64 != 0)
		s.Stage(15, obj.C128 != 0)
		return s
	})
}

func TestFillStructPtr(t *testing.T) {
	obj := struct {
		A *struct {
			S string
		}
	}{}

	f := New().NilChance(.5)
	tryFill(t, f, &obj, func() Stages {
		s := DeclareStages(3)
		s.Stage(0, obj.A == nil)
		if s.Stage(1, obj.A != nil) {
			s.Stage(2, obj.A.S != "")
		}
		return s
	})
}

// tryFill tries filling until check returns passed == true, or up to 100
// times. Fail if check() never passes, report the highest stage it ever got
// to.
func tryFill(t *testing.T, f filler, obj interface{}, check func() Stages) {
	t.Helper()
	var s Stages
	for i := 0; i < 100; i++ {
		f.Fill(obj)
		got := check()
		s = s.OrPasses(got)
		if s.AllPassed() {
			return
		}
	}
	t.Errorf("Not all stages passed:\n%s", s)
}

// Stages tracks pass/fail for up to 63 stages
type Stages struct {
	mask   uint64
	passed uint64
	failed uint64
}

func DeclareStages(N uint) Stages {
	if N >= 63 {
		panic("the math below only works up to 63")
	}
	return Stages{
		// Exercise for the reader: make this work for all 64 bits.
		mask: (uint64(1) << N) - 1,
	}
}

func (s Stages) String() string {
	explain := map[uint64]string{
		0: "never passed",
		1: "passed",
	}
	var parts []string
	for u := uint(0); s.mask != 0; u++ {
		parts = append(parts, fmt.Sprintf("stage %v: %v", u, explain[s.passed&1]))
		s.mask >>= 1
		s.passed >>= 1
	}
	return strings.Join(parts, "\n")
}

func (s Stages) OrPasses(s2 Stages) Stages {
	if s.mask == 0 {
		// Allow accumulating without knowing the number of stages in
		// advance
		s.mask = s2.mask
	}
	if s.mask != s2.mask {
		panic("coding error, stages are differently typed")
	}
	s.passed |= (s2.passed & ^s2.failed)
	return s
}

func (s Stages) AllPassed() bool {
	return (s.passed & ^s.failed) == s.mask
}

// Stage records pass/fail for stage n, and returns `pass` so it can be used to
// do conditional stages in an `if` statement.  If called multiple times for
// the same stage, every call for that stage must pass.
//
// Don't use Stage for things where seeing a failure once should fail the whole
// test regardless of retries.
func (s *Stages) Stage(n uint, pass bool) bool {
	if pass {
		s.passed |= 1 << n
		if s.passed > s.mask {
			panic("passed a stage that wasn't declared?")
		}
	} else {
		s.failed |= 1 << n
		if s.failed > s.mask {
			panic("passed a stage that wasn't declared?")
		}
	}
	return pass
}

type filler interface {
	Fill(obj interface{})
}

func TestFillStructMap(t *testing.T) {
	obj := &struct {
		A map[struct {
			S string
		}]struct {
			S2 string
		}
		B map[string]string
	}{}

	tryFill(t, New(), obj, func() Stages {
		s := DeclareStages(8)
		s.Stage(0, obj.A != nil)
		s.Stage(1, len(obj.A) != 0)
		for k, v := range obj.A {
			s.Stage(2, k.S != "")
			s.Stage(3, v.S2 != "")
		}

		s.Stage(4, obj.B != nil)
		s.Stage(5, len(obj.B) != 0)
		for k, v := range obj.B {
			s.Stage(6, k != "")
			s.Stage(7, v != "")
		}
		return s
	})
}

func TestFillStructSlice(t *testing.T) {
	obj := &struct {
		A []struct {
			S string
		}
		B []string
	}{}

	tryFill(t, New(), obj, func() Stages {
		s := DeclareStages(6)
		s.Stage(0, obj.A != nil)
		s.Stage(1, len(obj.A) != 0)
		for _, v := range obj.A {
			s.Stage(2, v.S != "")
		}

		s.Stage(3, obj.B != nil)
		s.Stage(4, len(obj.B) != 0)
		for _, v := range obj.B {
			s.Stage(5, v != "")
		}
		return s
	})
}

func TestFillStructArray(t *testing.T) {
	obj := &struct {
		A [3]struct {
			S string
		}
		B [2]int
	}{}

	tryFill(t, New(), obj, func() Stages {
		s := DeclareStages(2)
		for _, v := range obj.A {
			s.Stage(0, v.S != "")
		}

		for _, v := range obj.B {
			s.Stage(1, v != 0)
		}
		return s
	})
}

func TestFillCustom(t *testing.T) {
	obj := &struct {
		A string
		B *string
		C map[string]string
		D *map[string]string
	}{}

	testPhrase := "gotcalled"
	testMap := map[string]string{"C": "D"}
	f := New().Funcs(
		func(s *string, c Continue) {
			*s = testPhrase
		},
		func(m map[string]string, c Continue) {
			m["C"] = "D"
		},
	)

	tryFill(t, f, obj, func() Stages {
		s := DeclareStages(6)
		s.Stage(0, obj.A == testPhrase)
		if s.Stage(1, obj.B != nil) {
			s.Stage(2, *obj.B == testPhrase)
		}
		s.Stage(3, reflect.DeepEqual(testMap, obj.C))
		if s.Stage(4, obj.D != nil) {
			s.Stage(5, reflect.DeepEqual(testMap, *obj.D))
		}
		return s
	})
}

const selfFillerTestPhrase = "was randfilled"

type nativeSelfFiller string

// Implement randfill.NativeSelfFiller.
func (sf *nativeSelfFiller) RandFill(c Continue) {
	*sf = selfFillerTestPhrase
}

func TestNativeSelfFiller(t *testing.T) {
	f := New()

	var obj1 nativeSelfFiller
	tryFill(t, f, &obj1, func() Stages {
		s := DeclareStages(1)
		s.Stage(0, obj1 == selfFillerTestPhrase)
		return s
	})

	var obj2 map[int]nativeSelfFiller
	tryFill(t, f, &obj2, func() Stages {
		s := DeclareStages(1)
		for _, v := range obj2 {
			s.Stage(0, v == selfFillerTestPhrase)
		}
		return s
	})
}

func TestNativeSelfFillerAndFunc(t *testing.T) {
	const privateTestPhrase = "private phrase"
	f := New().Funcs(
		// This should take precedence over nativeSelfFiller.RandFill().
		func(s *nativeSelfFiller, c Continue) {
			*s = privateTestPhrase
		},
	)

	var obj1 nativeSelfFiller
	tryFill(t, f, &obj1, func() Stages {
		s := DeclareStages(1)
		s.Stage(0, obj1 == privateTestPhrase)
		return s
	})

	var obj2 map[int]nativeSelfFiller
	tryFill(t, f, &obj2, func() Stages {
		s := DeclareStages(1)
		for _, v := range obj2 {
			s.Stage(0, v == privateTestPhrase)
		}
		return s
	})
}

type simpleSelfFiller string

// Implement randfill.SimpleSelfFiller.
func (sf *simpleSelfFiller) RandFill(r *rand.Rand) {
	*sf = selfFillerTestPhrase
}

func TestSimpleSelfFiller(t *testing.T) {
	f := New()

	var obj1 simpleSelfFiller
	tryFill(t, f, &obj1, func() Stages {
		s := DeclareStages(1)
		s.Stage(0, obj1 == selfFillerTestPhrase)
		return s
	})

	var obj2 map[int]simpleSelfFiller
	tryFill(t, f, &obj2, func() Stages {
		s := DeclareStages(1)
		for _, v := range obj2 {
			s.Stage(0, v == selfFillerTestPhrase)
		}
		return s
	})
}

func TestSimpleSelfFillerAndFunc(t *testing.T) {
	const privateTestPhrase = "private phrase"
	f := New().Funcs(
		// This should take precedence over simpleSelfFiller.RandFill().
		func(s *simpleSelfFiller, c Continue) {
			*s = privateTestPhrase
		},
	)

	var obj1 simpleSelfFiller
	tryFill(t, f, &obj1, func() Stages {
		s := DeclareStages(1)
		s.Stage(0, obj1 == privateTestPhrase)
		return s
	})

	var obj2 map[int]simpleSelfFiller
	tryFill(t, f, &obj2, func() Stages {
		s := DeclareStages(1)
		for _, v := range obj2 {
			s.Stage(0, v == privateTestPhrase)
		}
		return s
	})
}

func TestFillNoCustom(t *testing.T) {
	type Inner struct {
		Str string
	}
	type Outer struct {
		Str string
		In  Inner
	}

	testPhrase := "gotcalled"
	f := New().Funcs(
		func(outer *Outer, c Continue) {
			outer.Str = testPhrase
			c.Fill(&outer.In)
		},
		func(inner *Inner, c Continue) {
			inner.Str = testPhrase
		},
	)
	c := Continue{fc: &fillerContext{filler: f}, Rand: f.r}

	// Filler.Fill()
	obj1 := Outer{}
	f.Fill(&obj1)
	if obj1.Str != testPhrase {
		t.Errorf("expected Outer custom function to have been called")
	}
	if obj1.In.Str != testPhrase {
		t.Errorf("expected Inner custom function to have been called")
	}

	// Continue.Fill()
	obj2 := Outer{}
	c.Fill(&obj2)
	if obj2.Str != testPhrase {
		t.Errorf("expected Outer custom function to have been called")
	}
	if obj2.In.Str != testPhrase {
		t.Errorf("expected Inner custom function to have been called")
	}

	// Filler.FillNoCustom()
	obj3 := Outer{}
	f.FillNoCustom(&obj3)
	if obj3.Str == testPhrase {
		t.Errorf("expected Outer custom function to not have been called")
	}
	if obj3.In.Str != testPhrase {
		t.Errorf("expected Inner custom function to have been called")
	}

	// Continue.FillNoCustom()
	obj4 := Outer{}
	c.FillNoCustom(&obj4)
	if obj4.Str == testPhrase {
		t.Errorf("expected Outer custom function to not have been called")
	}
	if obj4.In.Str != testPhrase {
		t.Errorf("expected Inner custom function to have been called")
	}
}

func TestContinueFillWithReflectValue(t *testing.T) {
	type obj struct {
		Str string
	}

	f := New()
	c := Continue{fc: &fillerContext{filler: f}, Rand: f.r}

	o := obj{}
	v := reflect.ValueOf(&o)

	tryFill(t, c, v, func() Stages {
		s := DeclareStages(1)
		s.Stage(0, o.Str != "")
		return s
	})
}

func TestFillNumElements(t *testing.T) {
	f := New().NilChance(0).NumElements(0, 1)
	obj := &struct {
		A []int
	}{}

	tryFill(t, f, obj, func() Stages {
		s := DeclareStages(3)
		s.Stage(0, obj.A != nil)
		s.Stage(1, len(obj.A) == 0)
		s.Stage(2, len(obj.A) == 1)

		if len(obj.A) > 1 {
			t.Errorf("we should never see more than 1 element, saw %v", len(obj.A))
		}
		return s
	})
}

func TestFillMaxdepth(t *testing.T) {
	type S struct {
		S *S
	}

	f := New().NilChance(0)

	f.MaxDepth(1)
	for i := 0; i < 100; i++ {
		obj := S{}
		f.Fill(&obj)

		if obj.S != nil {
			t.Errorf("Expected nil")
		}
	}

	f.MaxDepth(3) // field, ptr
	for i := 0; i < 100; i++ {
		obj := S{}
		f.Fill(&obj)

		if obj.S == nil {
			t.Errorf("Expected obj.S not nil")
		} else if obj.S.S != nil {
			t.Errorf("Expected obj.S.S nil")
		}
	}

	f.MaxDepth(5) // field, ptr, field, ptr
	for i := 0; i < 100; i++ {
		obj := S{}
		f.Fill(&obj)

		if obj.S == nil {
			t.Errorf("Expected obj.S not nil")
		} else if obj.S.S == nil {
			t.Errorf("Expected obj.S.S not nil")
		} else if obj.S.S.S != nil {
			t.Errorf("Expected obj.S.S.S nil")
		}
	}
}

func TestFillerAllowUnexportedFields(t *testing.T) {
	type S struct {
		stringField *string
	}

	f := New().NilChance(0)

	obj := S{}
	f.Fill(&obj)
	if obj.stringField != nil {
		t.Errorf("Expected obj.stringField to be nil")
	}

	f.AllowUnexportedFields(true)
	obj = S{}
	f.Fill(&obj)
	if obj.stringField == nil {
		t.Errorf("Expected stringField not nil")
	}

	f.AllowUnexportedFields(false)
	obj = S{}
	f.Fill(&obj)
	if obj.stringField != nil {
		t.Errorf("Expected obj.stringField to be empty")
	}
}

func TestFillSkipPattern(t *testing.T) {
	obj := &struct {
		S1    string
		S2    string
		XXX_S string
		S_XXX string
		In    struct {
			Str    string
			XXX_S1 string
			S2_XXX string
		}
	}{}

	f := New().NilChance(0).SkipFieldsWithPattern(regexp.MustCompile(`^XXX_`))
	f.Fill(obj)

	tryFill(t, f, obj, func() Stages {
		s := DeclareStages(2)
		s.Stage(0, obj.S_XXX != "")
		s.Stage(1, obj.In.S2_XXX != "")
		if a := obj.XXX_S; a != "" {
			t.Errorf("XXX_S not skipped, got %v", a)
		}
		if a := obj.In.XXX_S1; a != "" {
			t.Errorf("In.XXX_S not skipped, got %v", a)
		}
		return s
	})
}

func TestFillNilChanceZero(t *testing.T) {
	// This data source for random will result in the following four values
	// being sampled (the first, 0, being the most interesting case):
	//   0; 0.8727288671879787; 0.5547307616625858; 0.021885026049502695
	data := []byte("H0000000\x00")
	f := NewFromGoFuzz(data).NilChance(0)

	var fancyStruct struct {
		A, B, C, D *string
	}
	f.Fill(&fancyStruct) // None of the pointers should be nil, as NilChance is 0

	if fancyStruct.A == nil {
		t.Error("First value in struct was nil")
	}

	if fancyStruct.B == nil {
		t.Error("Second value in struct was nil")
	}

	if fancyStruct.C == nil {
		t.Error("Third value in struct was nil")
	}

	if fancyStruct.D == nil {
		t.Error("Fourth value in struct was nil")
	}
}

type int63mode int

const (
	modeRandom int63mode = iota
	modeFirst
	modeLast
)

type customInt63 struct {
	mode int63mode
}

func (c customInt63) Int63n(n int64) int64 {
	switch c.mode {
	case modeFirst:
		return 0
	case modeLast:
		return n - 1
	default:
		return rand.Int63n(n)
	}
}

func TestCharRangeChoose(t *testing.T) {
	lowercaseLetters := UnicodeRange{'a', 'z'}

	t.Run("Picks first", func(t *testing.T) {
		r := customInt63{mode: modeFirst}
		letter := lowercaseLetters.choose(r)
		if letter != 'a' {
			t.Errorf("Expected a, got %v", letter)
		}
	})

	t.Run("Picks last", func(t *testing.T) {
		r := customInt63{mode: modeLast}
		letter := lowercaseLetters.choose(r)
		if letter != 'z' {
			t.Errorf("Expected z, got %v", letter)
		}
	})
}

func TestUnicodeRangeCustomStringFillFunc(t *testing.T) {
	a2z := "abcdefghijklmnopqrstuvwxyz"

	unicodeRange := UnicodeRange{'a', 'z'}
	f := New().Funcs(unicodeRange.CustomStringFillFunc(0))
	var myString string
	f.Fill(&myString)

	t.Run("Picks a-z string", func(t *testing.T) {
		for i := range myString {
			if !strings.ContainsRune(a2z, rune(myString[i])) {
				t.Errorf("Expected a-z, got %v", string(myString[i]))
			}
		}
	})
}

func TestUnicodeRangeCheck(t *testing.T) {
	unicodeRange := UnicodeRange{'a', 'z'}

	unicodeRange.check()
}

func TestUnicodeRangesCustomStringFillFunc(t *testing.T) {
	a2z0to9 := "abcdefghijklmnopqrstuvwxyz0123456789"

	unicodeRanges := UnicodeRanges{
		{'a', 'z'},
		{'0', '9'},
	}
	f := New().Funcs(unicodeRanges.CustomStringFillFunc(0))
	var myString string
	f.Fill(&myString)

	t.Run("Picks a-z0-9 string", func(t *testing.T) {
		for i := range myString {
			if !strings.ContainsRune(a2z0to9, rune(myString[i])) {
				t.Errorf("Expected a-z0-9, got %v", string(myString[i]))
			}
		}
	})
}

func TestNoUnserializableTimes(t *testing.T) {
	var ti time.Time
	f := New()

	// This test is technically non-deterministic, however even before the fix it
	// would pass when the random uint64 used for nanoseconds was less than 1
	// billion. That is an extremely low false-negative rate but still, we try it
	// a few times to be really sure it's not passing by accident.
	for i := 0; i < 100; i++ {
		f.Fill(&ti)

		// Attempt to serialize the string to binary
		_, err := ti.MarshalBinary()
		if err != nil {
			// Before the fix this fails consistently (at least in BST timezone) with:
			//   Time.MarshalBinary: unexpected zone offset
			t.Fatalf("failed to serialise randfilled time: %s", err)
		}
	}
}

// TestFillThreadSafety lets the racedetector find races
func TestFillThreadSafety(t *testing.T) {
	f := New()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() { f.Fill(&[]string{}); wg.Done() }()
	go func() { f.Fill(&[]string{}); wg.Done() }()
	wg.Wait()
}

func TestNewFromGoFuzz(t *testing.T) {
	t.Parallel()

	input := []byte{1, 2, 3}

	var got int64
	NewFromGoFuzz(input).Fill(&got)

	if want := int64(5563767293437588600); want != got {
		t.Errorf("Fill(%q) = %d, want: %d", input, got, want)
	}
}

func BenchmarkRandBool(b *testing.B) {
	rs := rand.New(rand.NewSource(123))

	for i := 0; i < b.N; i++ {
		randBool(rs)
	}
}

func BenchmarkRandString(b *testing.B) {
	rs := rand.New(rand.NewSource(123))

	for i := 0; i < b.N; i++ {
		randString(rs, 0)
	}
}

func BenchmarkUnicodeRangeRandString(b *testing.B) {
	unicodeRange := UnicodeRange{'a', 'z'}

	rs := rand.New(rand.NewSource(123))

	for i := 0; i < b.N; i++ {
		unicodeRange.randString(rs, 0)
	}
}

func BenchmarkUnicodeRangesRandString(b *testing.B) {
	unicodeRanges := UnicodeRanges{
		{'a', 'z'},
		{'0', '9'},
	}

	rs := rand.New(rand.NewSource(123))

	for i := 0; i < b.N; i++ {
		unicodeRanges.randString(rs, 0)
	}
}
