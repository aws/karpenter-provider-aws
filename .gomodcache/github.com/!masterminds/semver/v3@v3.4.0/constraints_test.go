package semver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		in  string
		f   cfunc
		v   string
		err bool
	}{
		{">= 1.2", constraintGreaterThanEqual, "1.2.0", false},
		{"1.0", constraintTildeOrEqual, "1.0.0", false},
		{"foo", nil, "", true},
		{"<= 1.2", constraintLessThanEqual, "1.2.0", false},
		{"=< 1.2", constraintLessThanEqual, "1.2.0", false},
		{"=> 1.2", constraintGreaterThanEqual, "1.2.0", false},
		{"v1.2", constraintTildeOrEqual, "1.2.0", false},
		{"=1.5", constraintTildeOrEqual, "1.5.0", false},
		{"> 1.3", constraintGreaterThan, "1.3.0", false},
		{"< 1.4.1", constraintLessThan, "1.4.1", false},
		{"< 40.50.10", constraintLessThan, "40.50.10", false},
	}

	for _, tc := range tests {
		c, err := parseConstraint(tc.in)
		if tc.err && err == nil {
			t.Errorf("Expected error for %s didn't occur", tc.in)
		} else if !tc.err && err != nil {
			t.Errorf("Unexpected error for %s", tc.in)
		}

		// If an error was expected continue the loop and don't try the other
		// tests as they will cause errors.
		if tc.err {
			continue
		}

		if tc.v != c.con.String() {
			t.Errorf("Incorrect version found on %s", tc.in)
		}

		f1 := reflect.ValueOf(tc.f)
		f2 := reflect.ValueOf(constraintOps[c.origfunc])
		if f1 != f2 {
			t.Errorf("Wrong constraint found for %s", tc.in)
		}
	}
}

func TestConstraintCheck(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		check      bool
	}{
		{"=2.0.0", "1.2.3", false},
		{"=2.0.0", "2.0.0", true},
		{"=2.0", "1.2.3", false},
		{"=2.0", "2.0.0", true},
		{"=2.0", "2.0.1", true},
		{"4.1", "4.1.0", true},
		{"!=4.1.0", "4.1.0", false},
		{"!=4.1.0", "4.1.1", true},
		{"!=4.1", "4.1.0", false},
		{"!=4.1", "4.1.1", false},
		{"!=4.1", "5.1.0-alpha.1", false},
		{"!=4.1.0", "5.1.0-alpha.1", false},
		{"!=4.1-alpha", "4.1.0", true},
		{"!=4.1", "5.1.0", true},
		{"<11", "0.1.0", true},
		{"<11", "11.1.0", false},
		{"<1.1", "0.1.0", true},
		{"<1.1", "1.1.0", false},
		{"<1.1", "1.1.1", false},
		{"<=11", "1.2.3", true},
		{"<=11", "12.2.3", false},
		{"<=11", "11.2.3", true},
		{"<=1.1", "1.2.3", false},
		{"<=1.1", "0.1.0", true},
		{"<=1.1", "1.1.0", true},
		{"<=1.1", "1.1.1", true},
		{">1.1", "4.1.0", true},
		{">1.1", "1.1.0", false},
		{">0", "0", false},
		{">0", "1", true},
		{">0", "0.0.1-alpha", false},
		{">0.0", "0.0.1-alpha", false},
		{">0-0", "0.0.1-alpha", false},
		{">0.0-0", "0.0.1-alpha", false},
		{">0", "0.0.0-alpha", false},
		{">0-0", "0.0.0-alpha", false},
		{">0.0.0-0", "0.0.0-alpha", true},
		{">1.2.3-alpha.1", "1.2.3-alpha.2", true},
		{">1.2.3-alpha.1", "1.3.3-alpha.2", true},
		{">11", "11.1.0", false},
		{">11.1", "11.1.0", false},
		{">11.1", "11.1.1", false},
		{">11.1", "11.2.1", true},
		{">=11", "11.1.2", true},
		{">=11.1", "11.1.2", true},
		{">=11.1", "11.0.2", false},
		{">=1.1", "4.1.0", true},
		{">=1.1", "1.1.0", true},
		{">=1.1", "0.0.9", false},
		{">=0", "0.0.1-alpha", false},
		{">=0.0", "0.0.1-alpha", false},
		{">=0-0", "0.0.1-alpha", true},
		{">=0.0-0", "0.0.1-alpha", true},
		{">=0", "0.0.0-alpha", false},
		{">=0-0", "0.0.0-alpha", true},
		{">=0.0.0-0", "0.0.0-alpha", true},
		{">=0.0.0-0", "1.2.3", true},
		{">=0.0.0-0", "3.4.5-beta.1", true},
		{"<0", "0.0.0-alpha", false},
		{"<0-z", "0.0.0-alpha", true},
		{">=0", "0", true},
		{"=0", "1", false},
		{"*", "1", true},
		{"*", "4.5.6", true},
		{"*", "1.2.3-alpha.1", false},
		{"*-0", "1.2.3-alpha.1", true},
		{"2.*", "1", false},
		{"2.*", "3.4.5", false},
		{"2.*", "2.1.1", true},
		{"2.1.*", "2.1.1", true},
		{"2.1.*", "2.2.1", false},
		{"", "1", true}, // An empty string is treated as * or wild card
		{"", "4.5.6", true},
		{"", "1.2.3-alpha.1", false},
		{"2", "1", false},
		{"2", "3.4.5", false},
		{"2", "2.1.1", true},
		{"2.1", "2.1.1", true},
		{"2.1", "2.2.1", false},
		{"~1.2.3", "1.2.4", true},
		{"~1.2.3", "1.3.4", false},
		{"~1.2", "1.2.4", true},
		{"~1.2", "1.3.4", false},
		{"~1", "1.2.4", true},
		{"~1", "2.3.4", false},
		{"~0.2.3", "0.2.5", true},
		{"~0.2.3", "0.3.5", false},
		{"~1.2.3-beta.2", "1.2.3-beta.4", true},

		// This next test is a case that is different from npm/js semver handling.
		// Their prereleases are only range scoped to patch releases. This is
		// technically not following semver as docs note. In our case we are
		// following semver.
		{"~1.2.3-beta.2", "1.2.4-beta.2", true},
		{"~1.2.3-beta.2", "1.3.4-beta.2", false},
		{"^1.2.3", "1.8.9", true},
		{"^1.2.3", "2.8.9", false},
		{"^1.2.3", "1.2.1", false},
		{"^1.1.0", "2.1.0", false},
		{"^1.2.0", "2.2.1", false},
		{"^1.2.0", "1.2.1-alpha.1", false},
		{"^1.2.0-alpha.0", "1.2.1-alpha.1", true},
		{"^1.2.0-alpha.0", "1.2.1-alpha.0", true},
		{"^1.2.0-alpha.2", "1.2.0-alpha.1", false},
		{"^1.2", "1.8.9", true},
		{"^1.2", "2.8.9", false},
		{"^1", "1.8.9", true},
		{"^1", "2.8.9", false},
		{"^0.2.3", "0.2.5", true},
		{"^0.2.3", "0.5.6", false},
		{"^0.2", "0.2.5", true},
		{"^0.2", "0.5.6", false},
		{"^0.0.3", "0.0.3", true},
		{"^0.0.3", "0.0.4", false},
		{"^0.0", "0.0.3", true},
		{"^0.0", "0.1.4", false},
		{"^0.0", "1.0.4", false},
		{"^0", "0.2.3", true},
		{"^0", "1.1.4", false},
		{"^0.2.3-beta.2", "0.2.3-beta.4", true},

		// This next test is a case that is different from npm/js semver handling.
		// Their prereleases are only range scoped to patch releases. This is
		// technically not following semver as docs note. In our case we are
		// following semver.
		{"^0.2.3-beta.2", "0.2.4-beta.2", true},
		{"^0.2.3-beta.2", "0.3.4-beta.2", false},
		{"^0.2.3-beta.2", "0.2.3-beta.2", true},
	}

	var hasPre bool
	for _, tc := range tests {
		c, err := parseConstraint(tc.constraint)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		hasPre = false
		if c.con.pre != "" {
			hasPre = true
		}

		a, _ := c.check(v, hasPre)
		if a != tc.check {
			t.Errorf("Constraint %q failing with %q", tc.constraint, tc.version)
		}
	}
}

func TestNewConstraint(t *testing.T) {
	tests := []struct {
		input string
		ors   int
		count int
		err   bool
	}{
		{">= 1.1", 1, 1, false},
		{">40.50.60, < 50.70", 1, 2, false},
		{"2.0", 1, 1, false},
		{"v2.3.5-20161202202307-sha.e8fc5e5", 1, 1, false},
		{">= bar", 0, 0, true},
		{"BAR >= 1.2.3", 0, 0, true},

		// Test with space separated AND

		{">= 1.2.3 < 2.0", 1, 2, false},
		{">= 1.2.3 < 2.0 || => 3.0 < 4", 2, 2, false},

		// Test with commas separating AND
		{">= 1.2.3, < 2.0", 1, 2, false},
		{">= 1.2.3, < 2.0 || => 3.0, < 4", 2, 2, false},

		// The 3 - 4 should be broken into 2 by the range rewriting
		{"3 - 4 || => 3.0, < 4", 2, 2, false},

		// Due to having 4 parts these should produce an error. See
		// https://github.com/Masterminds/semver/issues/185 for the reason for
		// these tests.
		{"12.3.4.1234", 0, 0, true},
		{"12.23.4.1234", 0, 0, true},
		{"12.3.34.1234", 0, 0, true},
		{"12.3.34 ~1.2.3", 1, 2, false},
		{"12.3.34~ 1.2.3", 0, 0, true},

		{"1.0.0 - 2.0.0, <=2.0.0", 1, 3, false},
	}

	for _, tc := range tests {
		v, err := NewConstraint(tc.input)
		if tc.err && err == nil {
			t.Errorf("expected but did not get error for: %s", tc.input)
			continue
		} else if !tc.err && err != nil {
			t.Errorf("unexpectederror for input %s: %s", tc.input, err)
			continue
		}
		if tc.err {
			continue
		}

		l := len(v.constraints)
		if tc.ors != l {
			t.Errorf("Expected %s to have %d ORs but got %d",
				tc.input, tc.ors, l)
		}

		l = len(v.constraints[0])
		if tc.count != l {
			t.Errorf("Expected %s to have %d constraints but got %d",
				tc.input, tc.count, l)
		}
	}
}

func TestConstraintsCheck(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		check      bool
	}{
		{"*", "1.2.3", true},
		{"~0.0.0", "1.2.3", true},
		{"0.x.x", "1.2.3", false},
		{"0.0.x", "1.2.3", false},
		{"0.0.0", "1.2.3", false},
		{"*", "1.2.3", true},
		{"^0.0.0", "1.2.3", false},
		{"= 2.0", "1.2.3", false},
		{"= 2.0", "2.0.0", true},
		{"4.1", "4.1.0", true},
		{"4.1.x", "4.1.3", true},
		{"1.x", "1.4", true},
		{"!=4.1", "4.1.0", false},
		{"!=4.1-alpha", "4.1.0-alpha", false},
		{"!=4.1-alpha", "4.1.1-alpha", false},
		{"!=4.1-alpha", "4.1.0", true},
		{"!=4.1", "5.1.0", true},
		{"!=4.x", "5.1.0", true},
		{"!=4.x", "4.1.0", false},
		{"!=4.1.x", "4.2.0", true},
		{"!=4.2.x", "4.2.3", false},
		{">1.1", "4.1.0", true},
		{">1.1", "1.1.0", false},
		{"<1.1", "0.1.0", true},
		{"<1.1", "1.1.0", false},
		{"<1.1", "1.1.1", false},
		{"<1.x", "1.1.1", false},
		{"<1.x", "0.1.1", true},
		{"<1.x", "2.0.0", false},
		{"<1.1.x", "1.2.1", false},
		{"<1.1.x", "1.1.500", false},
		{"<1.1.x", "1.0.500", true},
		{"<1.2.x", "1.1.1", true},
		{">=1.1", "4.1.0", true},
		{">=1.1", "4.1.0-beta", false},
		{">=1.1", "1.1.0", true},
		{">=1.1", "0.0.9", false},
		{"<=1.1", "0.1.0", true},
		{"<=1.1", "0.1.0-alpha", false},
		{"<=1.1-a", "0.1.0-alpha", true},
		{"<=1.1", "1.1.0", true},
		{"<=1.x", "1.1.0", true},
		{"<=2.x", "3.0.0", false},
		{"<=1.1", "1.1.1", true},
		{"<=1.1.x", "1.2.500", false},
		{"<=4.5", "3.4.0", true},
		{"<=4.5", "3.7.0", true},
		{"<=4.5", "4.6.3", false},
		{">1.1, <2", "1.1.1", false},
		{">1.1, <2", "1.2.1", true},
		{">1.1, <3", "4.3.2", false},
		{">=1.1, <2, !=1.2.3", "1.2.3", false},
		{">1.1 <2", "1.1.1", false},
		{">1.1 <2", "1.2.1", true},
		{">1.1    <3", "4.3.2", false},
		{">=1.1    <2    !=1.2.3", "1.2.3", false},
		{">=1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1, <2, !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{">=1.1 <2 !=1.2.3", "1.2.3", false},
		{">=1.1 <2 !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1 <2 !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1 <2 !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1 <2 !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1 <2 !=1.2.3 || > 3", "1.2.3", false},
		{"> 1.1, <     2", "1.1.1", false},
		{">   1.1, <2", "1.2.1", true},
		{">1.1, <  3", "4.3.2", false},
		{">= 1.1, <     2, !=1.2.3", "1.2.3", false},
		{"> 1.1 < 2", "1.1.1", false},
		{">1.1 < 2", "1.2.1", true},
		{"> 1.1    <3", "4.3.2", false},
		{">=1.1    < 2    != 1.2.3", "1.2.3", false},
		{">= 1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">= 1.1, <2, != 1.2.3 || > 3", "3.1.2", false},
		{">= 1.1, <2, != 1.2.3 || >= 3", "3.0.0", true},
		{">= 1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">= 1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{">= 1.1 <2 != 1.2.3", "1.2.3", false},
		{">= 1.1 <2 != 1.2.3 || > 3", "4.1.2", true},
		{">= 1.1 <2 != 1.2.3 || > 3", "3.1.2", false},
		{">= 1.1 <2 != 1.2.3 || >= 3", "3.0.0", true},
		{">= 1.1 < 2 !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1 < 2 !=1.2.3 || > 3", "1.2.3", false},
		{">= 1.0.0  <= 2.0.0-beta", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta", "1.0.1", true},
		{">= 1.0.0  <= 2.0.0-beta", "3.0.0", false},
		{">= 1.0.0  <= 2.0.0-beta || > 3", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta || > 3", "3.0.1-beta", false},
		{">= 1.0.0  <= 2.0.0-beta != 1.0.1 || > 3", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta != 1.0.1-beta || > 3", "1.0.1-beta", false},
		{">= 1.0.0-0  <= 2.0.0", "1.0.1-beta", true},
		{"1.1 - 2", "1.1.1", true},
		{"1.5.0 - 4.5", "3.7.0", true},
		{"1.1-3", "4.3.2", false},
		{"^1.1", "1.1.1", true},
		{"^1.1", "4.3.2", false},
		{"^1.x", "1.1.1", true},
		{"^2.x", "1.1.1", false},
		{"^1.x", "2.1.1", false},
		{"^1.x", "1.1.1-beta1", false},
		{"^1.1.2-alpha", "1.2.1-beta1", true},
		{"^1.2.x-alpha", "1.1.1-beta1", false},
		{"^0.0.1", "0.0.1", true},
		{"^0.0.1", "0.3.1", false},
		{"~*", "2.1.1", true},
		{"~1", "2.1.1", false},
		{"~1", "1.3.5", true},
		{"~1", "1.4", true},
		{"~1.x", "2.1.1", false},
		{"~1.x", "1.3.5", true},
		{"~1.x", "1.4", true},
		{"~1.1", "1.1.1", true},
		{"~1.1", "1.1.1-alpha", false},
		{"~1.1-alpha", "1.1.1-beta", true},
		{"~1.1.1-beta", "1.1.1-alpha", false},
		{"~1.1.1-beta", "1.1.1", true},
		{"~1.2.3", "1.2.5", true},
		{"~1.2.3", "1.2.2", false},
		{"~1.2.3", "1.3.2", false},
		{"~1.1", "1.2.3", false},
		{"~1.3", "2.4.5", false},

		// Ranges should work in conjunction with other constraints anded together.
		{"1.0.0 - 2.0.0 <=2.0.0", "1.5.0", true},
		{"1.0.0 - 2.0.0, <=2.0.0", "1.5.0", true},
	}

	for _, tc := range tests {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		a := c.Check(v)
		if a != tc.check {
			t.Errorf("Constraint '%s' failing with '%s'", tc.constraint, tc.version)
		}
	}
}

func TestConstraintsCheckIncludePrerelease(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		check      bool
	}{
		// Test for including prereleases that normally would fail
		{">=1.1", "4.1.0-beta", true},
		{">1.1", "4.1.0-beta", true},
		{"<=1.1", "0.1.0-alpha", true},
		{"<1.1", "0.1.0-alpha", true},
		{"^1.x", "1.1.1-beta1", true},
		{"~1.1", "1.1.1-alpha", true},
		{"*", "1.2.3-alpha", true},
		{"= 2.0", "2.0.1-beta", true},

		// This next group of tests handles normal cases
		{"*", "1.2.3", true},
		{"~0.0.0", "1.2.3", true},
		{"0.x.x", "1.2.3", false},
		{"0.0.x", "1.2.3", false},
		{"0.0.0", "1.2.3", false},
		{"*", "1.2.3", true},
		{"^0.0.0", "1.2.3", false},
		{"= 2.0", "1.2.3", false},
		{"= 2.0", "2.0.0", true},
		{"4.1", "4.1.0", true},
		{"4.1.x", "4.1.3", true},
		{"1.x", "1.4", true},
		{"!=4.1", "4.1.0", false},
		{"!=4.1-alpha", "4.1.0-alpha", false},
		{"!=4.1-alpha", "4.1.1-alpha", false},
		{"!=4.1-alpha", "4.1.0", true},
		{"!=4.1", "5.1.0", true},
		{"!=4.x", "5.1.0", true},
		{"!=4.x", "4.1.0", false},
		{"!=4.1.x", "4.2.0", true},
		{"!=4.2.x", "4.2.3", false},
		{">1.1", "4.1.0", true},
		{">1.1", "1.1.0", false},
		{"<1.1", "0.1.0", true},
		{"<1.1", "1.1.0", false},
		{"<1.1", "1.1.1", false},
		{"<1.x", "1.1.1", false},
		{"<1.x", "0.1.1", true},
		{"<1.x", "2.0.0", false},
		{"<1.1.x", "1.2.1", false},
		{"<1.1.x", "1.1.500", false},
		{"<1.1.x", "1.0.500", true},
		{"<1.2.x", "1.1.1", true},
		{">=1.1", "4.1.0", true},
		{">=1.1", "1.1.0", true},
		{">=1.1", "0.0.9", false},
		{"<=1.1", "0.1.0", true},
		{"<=1.1-a", "0.1.0-alpha", true},
		{"<=1.1", "1.1.0", true},
		{"<=1.x", "1.1.0", true},
		{"<=2.x", "3.0.0", false},
		{"<=1.1", "1.1.1", true},
		{"<=1.1.x", "1.2.500", false},
		{"<=4.5", "3.4.0", true},
		{"<=4.5", "3.7.0", true},
		{"<=4.5", "4.6.3", false},
		{">1.1, <2", "1.1.1", false},
		{">1.1, <2", "1.2.1", true},
		{">1.1, <3", "4.3.2", false},
		{">=1.1, <2, !=1.2.3", "1.2.3", false},
		{">1.1 <2", "1.1.1", false},
		{">1.1 <2", "1.2.1", true},
		{">1.1    <3", "4.3.2", false},
		{">=1.1    <2    !=1.2.3", "1.2.3", false},
		{">=1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1, <2, !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{">=1.1 <2 !=1.2.3", "1.2.3", false},
		{">=1.1 <2 !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1 <2 !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1 <2 !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1 <2 !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1 <2 !=1.2.3 || > 3", "1.2.3", false},
		{"> 1.1, <     2", "1.1.1", false},
		{">   1.1, <2", "1.2.1", true},
		{">1.1, <  3", "4.3.2", false},
		{">= 1.1, <     2, !=1.2.3", "1.2.3", false},
		{"> 1.1 < 2", "1.1.1", false},
		{">1.1 < 2", "1.2.1", true},
		{"> 1.1    <3", "4.3.2", false},
		{">=1.1    < 2    != 1.2.3", "1.2.3", false},
		{">= 1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">= 1.1, <2, != 1.2.3 || > 3", "3.1.2", false},
		{">= 1.1, <2, != 1.2.3 || >= 3", "3.0.0", true},
		{">= 1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">= 1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{">= 1.1 <2 != 1.2.3", "1.2.3", false},
		{">= 1.1 <2 != 1.2.3 || > 3", "4.1.2", true},
		{">= 1.1 <2 != 1.2.3 || > 3", "3.1.2", false},
		{">= 1.1 <2 != 1.2.3 || >= 3", "3.0.0", true},
		{">= 1.1 < 2 !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1 < 2 !=1.2.3 || > 3", "1.2.3", false},
		{">= 1.0.0  <= 2.0.0-beta", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta", "1.0.1", true},
		{">= 1.0.0  <= 2.0.0-beta", "3.0.0", false},
		{">= 1.0.0  <= 2.0.0-beta || > 3", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta || > 3", "3.0.1-beta", false},
		{">= 1.0.0  <= 2.0.0-beta != 1.0.1 || > 3", "1.0.1-beta", true},
		{">= 1.0.0  <= 2.0.0-beta != 1.0.1-beta || > 3", "1.0.1-beta", false},
		{">= 1.0.0-0  <= 2.0.0", "1.0.1-beta", true},
		{"1.1 - 2", "1.1.1", true},
		{"1.5.0 - 4.5", "3.7.0", true},
		{"1.1-3", "4.3.2", false},
		{"^1.1", "1.1.1", true},
		{"^1.1", "4.3.2", false},
		{"^1.x", "1.1.1", true},
		{"^2.x", "1.1.1", false},
		{"^1.x", "2.1.1", false},
		{"^1.1.2-alpha", "1.2.1-beta1", true},
		{"^1.2.x-alpha", "1.1.1-beta1", false},
		{"^0.0.1", "0.0.1", true},
		{"^0.0.1", "0.3.1", false},
		{"~*", "2.1.1", true},
		{"~1", "2.1.1", false},
		{"~1", "1.3.5", true},
		{"~1", "1.4", true},
		{"~1.x", "2.1.1", false},
		{"~1.x", "1.3.5", true},
		{"~1.x", "1.4", true},
		{"~1.1", "1.1.1", true},
		{"~1.1-alpha", "1.1.1-beta", true},
		{"~1.1.1-beta", "1.1.1-alpha", false},
		{"~1.1.1-beta", "1.1.1", true},
		{"~1.2.3", "1.2.5", true},
		{"~1.2.3", "1.2.2", false},
		{"~1.2.3", "1.3.2", false},
		{"~1.1", "1.2.3", false},
		{"~1.3", "2.4.5", false},
		{"1.0.0 - 2.0.0 <=2.0.0", "1.5.0", true},
		{"1.0.0 - 2.0.0, <=2.0.0", "1.5.0", true},
	}

	for _, tc := range tests {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}
		// Include prereleases in searches
		c.IncludePrerelease = true

		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		a := c.Check(v)
		if a != tc.check {
			t.Errorf("Constraint '%s' failing with '%s'", tc.constraint, tc.version)
		}
	}
}

func TestRewriteRange(t *testing.T) {
	tests := []struct {
		c  string
		nc string
	}{
		{"2 - 3", ">= 2, <= 3 "},
		{"2 - 3, 2 - 3", ">= 2, <= 3 ,>= 2, <= 3 "},
		{"2 - 3, 4.0.0 - 5.1", ">= 2, <= 3 ,>= 4.0.0, <= 5.1 "},
		{"2 - 3 4.0.0 - 5.1", ">= 2, <= 3 >= 4.0.0, <= 5.1 "},
		{"1.0.0 - 2.0.0 <=2.0.0", ">= 1.0.0, <= 2.0.0 <=2.0.0"},
	}

	for _, tc := range tests {
		o := rewriteRange(tc.c)

		if o != tc.nc {
			t.Errorf("Range %s rewritten incorrectly as %q instead of expected %q", tc.c, o, tc.nc)
		}
	}
}

func TestIsX(t *testing.T) {
	tests := []struct {
		t string
		c bool
	}{
		{"A", false},
		{"%", false},
		{"X", true},
		{"x", true},
		{"*", true},
	}

	for _, tc := range tests {
		a := isX(tc.t)
		if a != tc.c {
			t.Errorf("Function isX error on %s", tc.t)
		}
	}
}

func TestConstraintsValidate(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		check      bool
	}{
		{"*", "1.2.3", true},
		{"~0.0.0", "1.2.3", true},
		{"= 2.0", "1.2.3", false},
		{"= 2.0", "2.0.0", true},
		{"4.1", "4.1.0", true},
		{"4.1.x", "4.1.3", true},
		{"1.x", "1.4", true},
		{"!=4.1", "4.1.0", false},
		{"!=4.1", "5.1.0", true},
		{"!=4.x", "5.1.0", true},
		{"!=4.x", "4.1.0", false},
		{"!=4.1.x", "4.2.0", true},
		{"!=4.2.x", "4.2.3", false},
		{">1.1", "4.1.0", true},
		{">1.1", "1.1.0", false},
		{"<1.1", "0.1.0", true},
		{"<1.1", "1.1.0", false},
		{"<1.1", "1.1.1", false},
		{"<1.x", "1.1.1", false},
		{"<2.x", "1.1.1", true},
		{"<1.x", "2.1.1", false},
		{"<1.1.x", "1.2.1", false},
		{"<1.1.x", "1.1.500", false},
		{"<1.2.x", "1.1.1", true},
		{">=1.1", "4.1.0", true},
		{">=1.1", "1.1.0", true},
		{">=1.1", "0.0.9", false},
		{"<=1.1", "0.1.0", true},
		{"<=1.1", "1.1.0", true},
		{"<=1.x", "1.1.0", true},
		{"<=2.x", "3.1.0", false},
		{"<=1.1", "1.1.1", true},
		{"<=1.1.x", "1.2.500", false},
		{">1.1, <2", "1.1.1", false},
		{">1.1, <2", "1.2.1", true},
		{">1.1, <3", "4.3.2", false},
		{">=1.1, <2, !=1.2.3", "1.2.3", false},
		{">=1.1, <2, !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1, <2, !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{"1.1 - 2", "1.1.1", true},
		{"1.1-3", "4.3.2", false},
		{"^1.1", "1.1.1", true},
		{"^1.1", "1.1.1-alpha", false},
		{"^1.1.1-alpha", "1.1.1-beta", true},
		{"^1.1.1-beta", "1.1.1-alpha", false},
		{"^1.1", "4.3.2", false},
		{"^1.x", "1.1.1", true},
		{"^2.x", "1.1.1", false},
		{"^1.x", "2.1.1", false},
		{"^0.0.1", "0.1.3", false},
		{"^0.0.1", "0.0.1", true},
		{"~*", "2.1.1", true},
		{"~1", "2.1.1", false},
		{"~1", "1.3.5", true},
		{"~1", "1.3.5-beta", false},
		{"~1.x", "2.1.1", false},
		{"~1.x", "1.3.5", true},
		{"~1.x", "1.3.5-beta", false},
		{"~1.3.6-alpha", "1.3.5-beta", false},
		{"~1.3.5-alpha", "1.3.5-beta", true},
		{"~1.3.5-beta", "1.3.5-alpha", false},
		{"~1.x", "1.4", true},
		{"~1.1", "1.1.1", true},
		{"~1.2.3", "1.2.5", true},
		{"~1.2.3", "1.2.2", false},
		{"~1.2.3", "1.3.2", false},
		{"~1.1", "1.2.3", false},
		{"~1.3", "2.4.5", false},
	}

	for _, tc := range tests {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		a, msgs := c.Validate(v)
		if a != tc.check {
			t.Errorf("Constraint '%s' failing with '%s'", tc.constraint, tc.version)
		} else if !a && len(msgs) == 0 {
			t.Errorf("%q failed with %q but no errors returned", tc.constraint, tc.version)
		}
	}

	v, err := StrictNewVersion("1.2.3")
	if err != nil {
		t.Errorf("err: %s", err)
	}

	c, err := NewConstraint("!= 1.2.5, ^2, <= 1.1.x")
	if err != nil {
		t.Errorf("err: %s", err)
	}

	_, msgs := c.Validate(v)
	if len(msgs) != 2 {
		t.Error("Invalid number of validations found")
	}
	e := msgs[0].Error()
	if e != "1.2.3 is less than 2" {
		t.Error("Did not get expected message: 1.2.3 is less than 2")
	}
	e = msgs[1].Error()
	if e != "1.2.3 is greater than 1.1.x" {
		t.Error("Did not get expected message: 1.2.3 is greater than 1.1.x")
	}

	tests2 := []struct {
		constraint, version, msg string
	}{
		{"2.x", "1.2.3", "1.2.3 is less than 2.x"},
		{"2", "1.2.3", "1.2.3 is less than 2"},
		{"= 2.0", "1.2.3", "1.2.3 is less than 2.0"},
		{"!=4.1", "4.1.0", "4.1.0 is equal to 4.1"},
		{"!=4.x", "4.1.0", "4.1.0 is equal to 4.x"},
		{"!=4.2.x", "4.2.3", "4.2.3 is equal to 4.2.x"},
		{">1.1", "1.1.0", "1.1.0 is less than or equal to 1.1"},
		{"<1.1", "1.1.0", "1.1.0 is greater than or equal to 1.1"},
		{"<1.1", "1.1.1", "1.1.1 is greater than or equal to 1.1"},
		{"<1.x", "2.1.1", "2.1.1 is greater than or equal to 1.x"},
		{"<1.1.x", "1.2.1", "1.2.1 is greater than or equal to 1.1.x"},
		{">=1.1", "0.0.9", "0.0.9 is less than 1.1"},
		{"<=2.x", "3.1.0", "3.1.0 is greater than 2.x"},
		{"<=1.1", "1.2.1", "1.2.1 is greater than 1.1"},
		{"<=1.1.x", "1.2.500", "1.2.500 is greater than 1.1.x"},
		{">1.1, <3", "4.3.2", "4.3.2 is greater than or equal to 3"},
		{">=1.1, <2, !=1.2.3", "1.2.3", "1.2.3 is equal to 1.2.3"},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", "3.0.0 is greater than or equal to 2"},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", "1.2.3 is equal to 1.2.3"},
		{"1.1 - 3", "4.3.2", "4.3.2 is greater than 3"},
		{"^1.1", "4.3.2", "4.3.2 does not have same major version as 1.1"},
		{"^1.12.7", "1.6.6", "1.6.6 is less than 1.12.7"},
		{"^2.x", "1.1.1", "1.1.1 is less than 2.x"},
		{"^1.x", "2.1.1", "2.1.1 does not have same major version as 1.x"},
		{"^0.2", "0.3.0", "0.3.0 does not have same minor version as 0.2. Expected minor versions to match when constraint major version is 0"},
		{"^0.2", "0.1.1", "0.1.1 is less than 0.2"},
		{"^0.0.3", "0.1.1", "0.1.1 does not have same minor version as 0.0.3"},
		{"^0.0.3", "0.0.4", "0.0.4 does not equal 0.0.3. Expect version and constraint to equal when major and minor versions are 0"},
		{"^0.0.3", "0.0.2", "0.0.2 is less than 0.0.3"},
		{"~1", "2.1.2", "2.1.2 does not have same major version as 1"},
		{"~1.x", "2.1.1", "2.1.1 does not have same major version as 1.x"},
		{"~1.2.3", "1.2.2", "1.2.2 is less than 1.2.3"},
		{"~1.2.3", "1.3.2", "1.3.2 does not have same major and minor version as 1.2.3"},
		{"~1.1", "1.2.3", "1.2.3 does not have same major and minor version as 1.1"},
		{"~1.3", "2.4.5", "2.4.5 does not have same major version as 1.3"},
		{"> 1.2.3", "1.2.3-beta.1", "1.2.3-beta.1 is a prerelease version and the constraint is only looking for release versions"},
	}

	for _, tc := range tests2 {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("constraint parsing err: %s", err)
			continue
		}

		v, err := StrictNewVersion(tc.version)
		if err != nil {
			t.Errorf("version parsing err: %s", err)
			continue
		}

		_, msgs := c.Validate(v)
		if len(msgs) == 0 {
			t.Errorf("Did not get error message on constraint %q", tc.constraint)
		} else {
			e := msgs[0].Error()
			if e != tc.msg {
				t.Errorf("Did not get expected message. Expected %q, got %q", tc.msg, e)
			}
		}
	}
}

func TestConstraintsValidateIncludePrerelease(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		check      bool
	}{
		// Tests that would fail if not including prereleases but
		// pass if prereleases are included.
		{"^1.1", "1.1.1-alpha", true},
		{"~1", "1.3.5-beta", true},
		{"~1.x", "1.3.5-beta", true},
		{">=1.1", "4.1.0-beta", true},
		{">1.1", "4.1.0-beta", true},
		{"<=1.1", "0.1.0-alpha", true},
		{"<1.1", "0.1.0-alpha", true},
		{"^1.x", "1.1.1-beta1", true},
		{"~1.1", "1.1.1-alpha", true},
		{"*", "1.2.3-alpha", true},
		{"= 2.0", "2.0.1-beta", true},

		// Tests that should continue to pass normally.
		{"*", "1.2.3", true},
		{"~0.0.0", "1.2.3", true},
		{"= 2.0", "1.2.3", false},
		{"= 2.0", "2.0.0", true},
		{"4.1", "4.1.0", true},
		{"4.1.x", "4.1.3", true},
		{"1.x", "1.4", true},
		{"!=4.1", "4.1.0", false},
		{"!=4.1", "5.1.0", true},
		{"!=4.x", "5.1.0", true},
		{"!=4.x", "4.1.0", false},
		{"!=4.1.x", "4.2.0", true},
		{"!=4.2.x", "4.2.3", false},
		{">1.1", "4.1.0", true},
		{">1.1", "1.1.0", false},
		{"<1.1", "0.1.0", true},
		{"<1.1", "1.1.0", false},
		{"<1.1", "1.1.1", false},
		{"<1.x", "1.1.1", false},
		{"<2.x", "1.1.1", true},
		{"<1.x", "2.1.1", false},
		{"<1.1.x", "1.2.1", false},
		{"<1.1.x", "1.1.500", false},
		{"<1.2.x", "1.1.1", true},
		{">=1.1", "4.1.0", true},
		{">=1.1", "1.1.0", true},
		{">=1.1", "0.0.9", false},
		{"<=1.1", "0.1.0", true},
		{"<=1.1", "1.1.0", true},
		{"<=1.x", "1.1.0", true},
		{"<=2.x", "3.1.0", false},
		{"<=1.1", "1.1.1", true},
		{"<=1.1.x", "1.2.500", false},
		{">1.1, <2", "1.1.1", false},
		{">1.1, <2", "1.2.1", true},
		{">1.1, <3", "4.3.2", false},
		{">=1.1, <2, !=1.2.3", "1.2.3", false},
		{">=1.1, <2, !=1.2.3 || > 3", "3.1.2", false},
		{">=1.1, <2, !=1.2.3 || > 3", "4.1.2", true},
		{">=1.1, <2, !=1.2.3 || >= 3", "3.0.0", true},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", false},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", false},
		{"1.1 - 2", "1.1.1", true},
		{"1.1-3", "4.3.2", false},
		{"^1.1", "1.1.1", true},
		{"^1.1.1-alpha", "1.1.1-beta", true},
		{"^1.1.1-beta", "1.1.1-alpha", false},
		{"^1.1", "4.3.2", false},
		{"^1.x", "1.1.1", true},
		{"^2.x", "1.1.1", false},
		{"^1.x", "2.1.1", false},
		{"^0.0.1", "0.1.3", false},
		{"^0.0.1", "0.0.1", true},
		{"~*", "2.1.1", true},
		{"~1", "2.1.1", false},
		{"~1", "1.3.5", true},
		{"~1.x", "2.1.1", false},
		{"~1.x", "1.3.5", true},
		{"~1.3.6-alpha", "1.3.5-beta", false},
		{"~1.3.5-alpha", "1.3.5-beta", true},
		{"~1.3.5-beta", "1.3.5-alpha", false},
		{"~1.x", "1.4", true},
		{"~1.1", "1.1.1", true},
		{"~1.2.3", "1.2.5", true},
		{"~1.2.3", "1.2.2", false},
		{"~1.2.3", "1.3.2", false},
		{"~1.1", "1.2.3", false},
		{"~1.3", "2.4.5", false},
	}

	for _, tc := range tests {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}
		c.IncludePrerelease = true

		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("err: %s", err)
			continue
		}

		a, msgs := c.Validate(v)
		if a != tc.check {
			t.Errorf("Constraint '%s' failing with '%s'", tc.constraint, tc.version)
		} else if !a && len(msgs) == 0 {
			t.Errorf("%q failed with %q but no errors returned", tc.constraint, tc.version)
		}
	}

	v, err := StrictNewVersion("1.2.3")
	if err != nil {
		t.Errorf("err: %s", err)
	}

	c, err := NewConstraint("!= 1.2.5, ^2, <= 1.1.x")
	if err != nil {
		t.Errorf("err: %s", err)
	}
	c.IncludePrerelease = true

	_, msgs := c.Validate(v)
	if len(msgs) != 2 {
		t.Error("Invalid number of validations found")
	}
	e := msgs[0].Error()
	if e != "1.2.3 is less than 2" {
		t.Error("Did not get expected message: 1.2.3 is less than 2")
	}
	e = msgs[1].Error()
	if e != "1.2.3 is greater than 1.1.x" {
		t.Error("Did not get expected message: 1.2.3 is greater than 1.1.x")
	}

	tests2 := []struct {
		constraint, version, msg string
	}{
		// Validations that would return a prerelease message normally but
		// because prereleases are included are being evaluated based on
		// the version.
		{"> 1.2.3", "1.2.3-beta.1", "1.2.3-beta.1 is less than or equal to 1.2.3"},
		{"< 1.2.3", "1.2.4-beta.1", "1.2.4-beta.1 is greater than or equal to 1.2.3"},
		{">= 1.2.3", "1.2.3-beta.1", "1.2.3-beta.1 is less than 1.2.3"},
		{"<= 1.2.3", "1.2.4-beta.1", "1.2.4-beta.1 is greater than 1.2.3"},

		// Test messages that are the same because there is no
		// prerelease issue.
		{"2.x", "1.2.3", "1.2.3 is less than 2.x"},
		{"2", "1.2.3", "1.2.3 is less than 2"},
		{"= 2.0", "1.2.3", "1.2.3 is less than 2.0"},
		{"!=4.1", "4.1.0", "4.1.0 is equal to 4.1"},
		{"!=4.x", "4.1.0", "4.1.0 is equal to 4.x"},
		{"!=4.2.x", "4.2.3", "4.2.3 is equal to 4.2.x"},
		{">1.1", "1.1.0", "1.1.0 is less than or equal to 1.1"},
		{"<1.1", "1.1.0", "1.1.0 is greater than or equal to 1.1"},
		{"<1.1", "1.1.1", "1.1.1 is greater than or equal to 1.1"},
		{"<1.x", "2.1.1", "2.1.1 is greater than or equal to 1.x"},
		{"<1.1.x", "1.2.1", "1.2.1 is greater than or equal to 1.1.x"},
		{">=1.1", "0.0.9", "0.0.9 is less than 1.1"},
		{"<=2.x", "3.1.0", "3.1.0 is greater than 2.x"},
		{"<=1.1", "1.2.1", "1.2.1 is greater than 1.1"},
		{"<=1.1.x", "1.2.500", "1.2.500 is greater than 1.1.x"},
		{">1.1, <3", "4.3.2", "4.3.2 is greater than or equal to 3"},
		{">=1.1, <2, !=1.2.3", "1.2.3", "1.2.3 is equal to 1.2.3"},
		{">=1.1, <2, !=1.2.3 || > 3", "3.0.0", "3.0.0 is greater than or equal to 2"},
		{">=1.1, <2, !=1.2.3 || > 3", "1.2.3", "1.2.3 is equal to 1.2.3"},
		{"1.1 - 3", "4.3.2", "4.3.2 is greater than 3"},
		{"^1.1", "4.3.2", "4.3.2 does not have same major version as 1.1"},
		{"^1.12.7", "1.6.6", "1.6.6 is less than 1.12.7"},
		{"^2.x", "1.1.1", "1.1.1 is less than 2.x"},
		{"^1.x", "2.1.1", "2.1.1 does not have same major version as 1.x"},
		{"^0.2", "0.3.0", "0.3.0 does not have same minor version as 0.2. Expected minor versions to match when constraint major version is 0"},
		{"^0.2", "0.1.1", "0.1.1 is less than 0.2"},
		{"^0.0.3", "0.1.1", "0.1.1 does not have same minor version as 0.0.3"},
		{"^0.0.3", "0.0.4", "0.0.4 does not equal 0.0.3. Expect version and constraint to equal when major and minor versions are 0"},
		{"^0.0.3", "0.0.2", "0.0.2 is less than 0.0.3"},
		{"~1", "2.1.2", "2.1.2 does not have same major version as 1"},
		{"~1.x", "2.1.1", "2.1.1 does not have same major version as 1.x"},
		{"~1.2.3", "1.2.2", "1.2.2 is less than 1.2.3"},
		{"~1.2.3", "1.3.2", "1.3.2 does not have same major and minor version as 1.2.3"},
		{"~1.1", "1.2.3", "1.2.3 does not have same major and minor version as 1.1"},
		{"~1.3", "2.4.5", "2.4.5 does not have same major version as 1.3"},
	}

	for _, tc := range tests2 {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("constraint parsing err: %s", err)
			continue
		}
		c.IncludePrerelease = true

		v, err := StrictNewVersion(tc.version)
		if err != nil {
			t.Errorf("version parsing err: %s", err)
			continue
		}

		_, msgs := c.Validate(v)
		if len(msgs) == 0 {
			t.Errorf("Did not get error message on constraint %q", tc.constraint)
		} else {
			e := msgs[0].Error()
			if e != tc.msg {
				t.Errorf("Did not get expected message. Expected %q, got %q", tc.msg, e)
			}
		}
	}
}

func TestConstraintString(t *testing.T) {
	tests := []struct {
		constraint string
		st         string
	}{
		{"*", "*"},
		{">=1.2.3", ">=1.2.3"},
		{">= 1.2.3", ">=1.2.3"},
		{"2.x,   >=1.2.3 || >4.5.6, < 5.7", "2.x >=1.2.3 || >4.5.6 <5.7"},
		{"2.x,   >=1.2.3 || >4.5.6, < 5.7 || >40.50.60, < 50.70", "2.x >=1.2.3 || >4.5.6 <5.7 || >40.50.60 <50.70"},
		{"1.2", "1.2"},
	}

	for _, tc := range tests {
		c, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("cannot create constraint for %q, err: %s", tc.constraint, err)
			continue
		}

		if c.String() != tc.st {
			t.Errorf("expected constraint from %q to be a string as %q but got %q", tc.constraint, tc.st, c.String())
		}

		if _, err = NewConstraint(c.String()); err != nil {
			t.Errorf("expected string from constrint %q to parse as valid but got err: %s", tc.constraint, err)
		}
	}
}

func TestTextMarshalConstraints(t *testing.T) {
	tests := []struct {
		constraint string
		want       string
	}{
		{"1.2.3", "1.2.3"},
		{">=1.2.3", ">=1.2.3"},
		{"<=1.2.3", "<=1.2.3"},
		{"1 <=1.2.3", "1 <=1.2.3"},
		{"1, <=1.2.3", "1 <=1.2.3"},
		{">1, <=1.2.3", ">1 <=1.2.3"},
		{"> 1 , <=1.2.3", ">1 <=1.2.3"},
	}

	for _, tc := range tests {
		cs, err := NewConstraint(tc.constraint)
		if err != nil {
			t.Errorf("Error creating constraints: %s", err)
		}

		out, err2 := cs.MarshalText()
		if err2 != nil {
			t.Errorf("Error constraint version: %s", err2)
		}

		got := string(out)
		if got != tc.want {
			t.Errorf("Error marshaling constraint, unexpected marshaled content: got=%q want=%q", got, tc.want)
		}

		// Test that this works for JSON as well as text. When JSON marshaling
		// functions are missing it falls through to TextMarshal.
		// NOTE: To not escape the < and > (which json.Marshal does) you need
		// a custom encoder where html escaping is disabled. This must be done
		// in the top level encoder being used to marshal the constraints.
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err = enc.Encode(cs)
		if err != nil {
			t.Errorf("Error unmarshaling constraint: %s", err)
		}
		got = buf.String()
		// The encoder used here adds a newline so we add that to what we want
		// so they align. The newline is an artifact of the testing.
		want := fmt.Sprintf("%q\n", tc.want)
		if got != want {
			t.Errorf("Error marshaling constraint, unexpected marshaled content: got=%q want=%q", got, want)
		}
	}
}

func TestTextUnmarshalConstraints(t *testing.T) {
	tests := []struct {
		constraint string
		want       string
	}{
		{"1.2.3", "1.2.3"},
		{">=1.2.3", ">=1.2.3"},
		{"<=1.2.3", "<=1.2.3"},
		{">1 <=1.2.3", ">1 <=1.2.3"},
		{"> 1 <=1.2.3", ">1 <=1.2.3"},
		{">1, <=1.2.3", ">1 <=1.2.3"},
	}

	for _, tc := range tests {
		cs := Constraints{}
		err := cs.UnmarshalText([]byte(tc.constraint))
		if err != nil {
			t.Errorf("Error unmarshaling constraints: %s", err)
		}
		got := cs.String()
		if got != tc.want {
			t.Errorf("Error unmarshaling constraint, unexpected object content: got=%q want=%q", got, tc.want)
		}

		// Test that this works for JSON as well as text. When JSON unmarshaling
		// functions are missing it falls through to TextUnmarshal.
		err = json.Unmarshal([]byte(fmt.Sprintf("%q", tc.constraint)), &cs)
		if err != nil {
			t.Errorf("Error unmarshaling constraints: %s", err)
		}
		got = cs.String()
		if got != tc.want {
			t.Errorf("Error unmarshaling constraint, unexpected object content: got=%q want=%q", got, tc.want)
		}
	}
}

func FuzzNewConstraint(f *testing.F) {
	testcases := []string{
		"v1.2.3",
		" ",
		"......",
		"1",
		"1.2.3-beta.1",
		"1.2.3+foo",
		"2.3.4-alpha.1+bar",
		"lorem ipsum",
		"*",
		"!=1.2.3",
		"^4.5",
		"1.0.0 - 2",
		"1.2.3.4.5.6",
		">= 1",
		"~9.8.7",
		"<= 12.13.14",
		"987654321.123456789.654123789",
		"1.x",
		"2.3.x",
		"9.2-beta.0",
	}

	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(_ *testing.T, a string) {
		_, _ = NewConstraint(a)
	})
}
