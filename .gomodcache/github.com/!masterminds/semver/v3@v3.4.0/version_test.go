package semver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
)

func TestStrictNewVersion(t *testing.T) {
	tests := []struct {
		version string
		err     bool
	}{
		{"1.2.3", false},
		{"1.2.3-alpha.01", true},
		{"1.2.3+test.01", false},
		{"1.2.3-alpha.-1", false},
		{"v1.2.3", true},
		{"1.0", true},
		{"v1.0", true},
		{"1", true},
		{"v1", true},
		{"1.2", true},
		{"1.2.beta", true},
		{"v1.2.beta", true},
		{"foo", true},
		{"1.2-5", true},
		{"v1.2-5", true},
		{"1.2-beta.5", true},
		{"v1.2-beta.5", true},
		{"\n1.2", true},
		{"\nv1.2", true},
		{"1.2.0-x.Y.0+metadata", false},
		{"v1.2.0-x.Y.0+metadata", true},
		{"1.2.0-x.Y.0+metadata-width-hypen", false},
		{"v1.2.0-x.Y.0+metadata-width-hypen", true},
		{"1.2.3-rc1-with-hypen", false},
		{"v1.2.3-rc1-with-hypen", true},
		{"1.2.3.4", true},
		{"v1.2.3.4", true},
		{"1.2.2147483648", false},
		{"1.2147483648.3", false},
		{"2147483648.3.0", false},

		// The SemVer spec in a pre-release expects to allow [0-9A-Za-z-]. But,
		// the lack of all 3 parts in this version should produce an error.
		{"20221209-update-renovatejson-v4", true},

		// Various cases that are invalid semver
		{"1.1.2+.123", true},                             // A leading . in build metadata. This would signify that the first segment is empty
		{"1.0.0-alpha_beta", true},                       // An underscore in the pre-release is an invalid character
		{"1.0.0-alpha..", true},                          // Multiple empty segments
		{"1.0.0-alpha..1", true},                         // Multiple empty segments but one with a value
		{"01.1.1", true},                                 // A leading 0 on a number segment
		{"1.01.1", true},                                 // A leading 0 on a number segment
		{"1.1.01", true},                                 // A leading 0 on a number segment
		{"9.8.7+meta+meta", true},                        // Multiple metadata parts
		{"1.2.31----RC-SNAPSHOT.12.09.1--.12+788", true}, // Leading 0 in a number part of a pre-release segment
		{"1.2.3-0123", true},
		{"1.2.3-0123.0123", true},
		{"+invalid", true},
		{"-invalid", true},
		{"-invalid.01", true},
		{"alpha+beta", true},
		{"1.2.3-alpha_beta+foo", true},
		{"1.0.0-alpha..1", true},
	}

	for _, tc := range tests {
		_, err := StrictNewVersion(tc.version)
		if tc.err && err == nil {
			t.Fatalf("expected error for version: %s", tc.version)
		} else if !tc.err && err != nil {
			t.Fatalf("error for version %s: %s", tc.version, err)
		}
	}
}

func TestNewVersion(t *testing.T) {
	t.Run("With loose NewVersion", func(t *testing.T) {
		tests := []struct {
			version string
			err     bool
		}{
			{"1.2.3", false},
			{"1.2.3-alpha.01", true},
			{"1.2.3+test.01", false},
			{"1.2.3-alpha.-1", false},
			{"v1.2.3", false},
			{"1.0", false},
			{"v1.0", false},
			{"1", false},
			{"v1", false},
			{"1.2.beta", true},
			{"v1.2.beta", true},
			{"foo", true},
			{"1.2-5", false},
			{"v1.2-5", false},
			{"1.2-beta.5", false},
			{"v1.2-beta.5", false},
			{"\n1.2", true},
			{"\nv1.2", true},
			{"1.2.0-x.Y.0+metadata", false},
			{"v1.2.0-x.Y.0+metadata", false},
			{"1.2.0-x.Y.0+metadata-width-hypen", false},
			{"v1.2.0-x.Y.0+metadata-width-hypen", false},
			{"1.2.3-rc1-with-hypen", false},
			{"v1.2.3-rc1-with-hypen", false},
			{"1.2.3.4", true},
			{"v1.2.3.4", true},
			{"1.2.2147483648", false},
			{"1.2147483648.3", false},
			{"2147483648.3.0", false},

			// Due to having 4 parts these should produce an error. See
			// https://github.com/Masterminds/semver/issues/185 for the reason for
			// these tests.
			{"12.3.4.1234", true},
			{"12.23.4.1234", true},
			{"12.3.34.1234", true},

			// The SemVer spec in a pre-release expects to allow [0-9A-Za-z-].
			{"20221209-update-renovatejson-v4", false},

			// Various cases that are invalid semver
			{"1.1.2+.123", true},                             // A leading . in build metadata. This would signify that the first segment is empty
			{"1.0.0-alpha_beta", true},                       // An underscore in the pre-release is an invalid character
			{"1.0.0-alpha..", true},                          // Multiple empty segments
			{"1.0.0-alpha..1", true},                         // Multiple empty segments but one with a value
			{"9.8.7+meta+meta", true},                        // Multiple metadata parts
			{"1.2.31----RC-SNAPSHOT.12.09.1--.12+788", true}, // Leading 0 in a number part of a pre-release segment

			// Versions that are invalid but in loose mode are handled.
			// This enables a calver-ish style. This pattern has long
			// been supported by this package even though it technically
			// breaks from semver. Tools built on this allow it.
			{"01.1.1", false}, // A leading 0 on a number segment
			{"1.01.1", false}, // A leading 0 on a number segment
			{"1.1.01", false}, // A leading 0 on a number segment
		}

		for _, tc := range tests {
			_, err := NewVersion(tc.version)
			if tc.err && err == nil {
				t.Fatalf("expected error for version: %s", tc.version)
			} else if !tc.err && err != nil {
				t.Fatalf("error for version %s: %s", tc.version, err)
			}
		}
	})

	t.Run("Without loose NewVersion", func(t *testing.T) {
		CoerceNewVersion = false
		defer func() {
			CoerceNewVersion = true
		}()
		tests := []struct {
			version string
			err     bool
		}{
			{"1.2.3", false},
			{"1.2.3-alpha.01", true},
			{"1.2.3+test.01", false},
			{"1.2.3-alpha.-1", false},
			{"v1.2.3", false},
			{"1.0", false},
			{"v1.0", false},
			{"1", false},
			{"v1", false},
			{"1.2.beta", true},
			{"v1.2.beta", true},
			{"foo", true},
			{"1.2-5", false},
			{"v1.2-5", false},
			{"1.2-beta.5", false},
			{"v1.2-beta.5", false},
			{"\n1.2", true},
			{"\nv1.2", true},
			{"1.2.0-x.Y.0+metadata", false},
			{"v1.2.0-x.Y.0+metadata", false},
			{"1.2.0-x.Y.0+metadata-width-hypen", false},
			{"v1.2.0-x.Y.0+metadata-width-hypen", false},
			{"1.2.3-rc1-with-hypen", false},
			{"v1.2.3-rc1-with-hypen", false},
			{"1.2.3.4", true},
			{"v1.2.3.4", true},
			{"1.2.2147483648", false},
			{"1.2147483648.3", false},
			{"2147483648.3.0", false},

			// Due to having 4 parts these should produce an error. See
			// https://github.com/Masterminds/semver/issues/185 for the reason for
			// these tests.
			{"12.3.4.1234", true},
			{"12.23.4.1234", true},
			{"12.3.34.1234", true},

			// The SemVer spec in a pre-release expects to allow [0-9A-Za-z-].
			{"20221209-update-renovatejson-v4", false},

			// Various cases that are invalid semver
			{"1.1.2+.123", true},                             // A leading . in build metadata. This would signify that the first segment is empty
			{"1.0.0-alpha_beta", true},                       // An underscore in the pre-release is an invalid character
			{"1.0.0-alpha..", true},                          // Multiple empty segments
			{"1.0.0-alpha..1", true},                         // Multiple empty segments but one with a value
			{"01.1.1", true},                                 // A leading 0 on a number segment
			{"1.01.1", true},                                 // A leading 0 on a number segment
			{"1.1.01", true},                                 // A leading 0 on a number segment
			{"9.8.7+meta+meta", true},                        // Multiple metadata parts
			{"1.2.31----RC-SNAPSHOT.12.09.1--.12+788", true}, // Leading 0 in a number part of a pre-release segment
		}

		for _, tc := range tests {
			_, err := NewVersion(tc.version)
			if tc.err && err == nil {
				t.Fatalf("expected error for version: %s", tc.version)
			} else if !tc.err && err != nil {
				t.Fatalf("error for version %s: %s", tc.version, err)
			}
		}
	})
}

// TestNewVersionCheckError checks the returned error for compatibility
func TestNewVersionCheckError(t *testing.T) {
	t.Run("With detailed errors", func(t *testing.T) {
		CoerceNewVersion = false
		defer func() {
			CoerceNewVersion = true
		}()
		tests := []struct {
			version string
			wantErr error
		}{
			{
				"1.2.3",
				nil,
			},
			{
				"1.2.3-dev.1",
				nil,
			},
			{
				"1.2.3+meta",
				nil,
			},
			{
				"1.2.3-dev.1+meta",
				nil,
			},
			{
				"1.2.3-dev.1+meta.01",
				nil,
			},
			{
				"1.2.3-dev..1",
				ErrInvalidSemVer,
			},
			{
				"1.2.3-dev.01",
				ErrSegmentStartsZero,
			},
			{
				"1.2.3-de-v.1",
				nil,
			},
			{
				"1.02.3",
				ErrSegmentStartsZero,
			},
			{
				"01.2.3",
				ErrSegmentStartsZero,
			},
			{
				"1.2.03",
				ErrSegmentStartsZero,
			},
			{
				"1.2.3-023658",
				ErrSegmentStartsZero,
			},
		}

		for _, tc := range tests {
			_, err := NewVersion(tc.version)
			if err != tc.wantErr {
				t.Errorf("expected error %q but got %q for version %s", tc.wantErr, err, tc.version)
			}
		}
	})

	t.Run("Without detailed errors", func(t *testing.T) {
		DetailedNewVersionErrors = false
		CoerceNewVersion = false
		defer func() {
			DetailedNewVersionErrors = true
			CoerceNewVersion = true
		}()

		tests := []struct {
			version string
			wantErr error
		}{
			{
				"1.2.3",
				nil,
			},
			{
				"1.2.3-dev.1",
				nil,
			},
			{
				"1.2.3+meta",
				nil,
			},
			{
				"1.2.3-dev.1+meta",
				nil,
			},
			{
				"1.2.3-dev.1+meta.01",
				nil,
			},
			{
				"1.2.3-dev..1",
				ErrInvalidSemVer,
			},
			{
				"1.2.3-dev.01",
				ErrInvalidSemVer,
			},
			{
				"1.2.3-de-v.1",
				nil,
			},
			{
				"1.02.3",
				ErrInvalidSemVer,
			},
			{
				"01.2.3",
				ErrInvalidSemVer,
			},
			{
				"1.2.03",
				ErrInvalidSemVer,
			},
			{
				"1.2.3-023658",
				ErrInvalidSemVer,
			},
		}

		for _, tc := range tests {
			_, err := NewVersion(tc.version)
			if err != tc.wantErr {
				t.Errorf("expected error %q but got %q for version %s", tc.wantErr, err, tc.version)
			}
		}
	})
}

func TestNew(t *testing.T) {
	// v0.1.2
	v := New(0, 1, 2, "", "")

	if v.String() != "0.1.2" {
		t.Errorf("expected version 0.1.2 but got %q", v.String())
	}

	// v1.2.3-alpha.1+foo.bar
	v = New(1, 2, 3, "alpha.1", "foo.bar")

	if v.String() != "1.2.3-alpha.1+foo.bar" {
		t.Errorf("expected version 1.2.3-alpha.1+foo.bar but got %q", v.String())
	}
}

func TestOriginal(t *testing.T) {
	tests := []string{
		"1.2.3",
		"v1.2.3",
		"1.0",
		"v1.0",
		"1",
		"v1",
		"1.2-5",
		"v1.2-5",
		"1.2-beta.5",
		"v1.2-beta.5",
		"1.2.0-x.Y.0+metadata",
		"v1.2.0-x.Y.0+metadata",
		"1.2.0-x.Y.0+metadata-width-hypen",
		"v1.2.0-x.Y.0+metadata-width-hypen",
		"1.2.3-rc1-with-hypen",
		"v1.2.3-rc1-with-hypen",
	}

	for _, tc := range tests {
		v, err := NewVersion(tc)
		if err != nil {
			t.Errorf("Error parsing version %s", tc)
		}

		o := v.Original()
		if o != tc {
			t.Errorf("Error retrieving original. Expected '%s' but got '%v'", tc, v)
		}
	}
}

func TestParts(t *testing.T) {
	v, err := NewVersion("1.2.3-beta.1+build.123")
	if err != nil {
		t.Error("Error parsing version 1.2.3-beta.1+build.123")
	}

	if v.Major() != 1 {
		t.Error("Major() returning wrong value")
	}
	if v.Minor() != 2 {
		t.Error("Minor() returning wrong value")
	}
	if v.Patch() != 3 {
		t.Error("Patch() returning wrong value")
	}
	if v.Prerelease() != "beta.1" {
		t.Error("Prerelease() returning wrong value")
	}
	if v.Metadata() != "build.123" {
		t.Error("Metadata() returning wrong value")
	}
}

func TestCoerceString(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.0", "1.0.0"},
		{"v1.0", "1.0.0"},
		{"1", "1.0.0"},
		{"v1", "1.0.0"},
		{"1.2-5", "1.2.0-5"},
		{"v1.2-5", "1.2.0-5"},
		{"1.2-beta.5", "1.2.0-beta.5"},
		{"v1.2-beta.5", "1.2.0-beta.5"},
		{"1.2.0-x.Y.0+metadata", "1.2.0-x.Y.0+metadata"},
		{"v1.2.0-x.Y.0+metadata", "1.2.0-x.Y.0+metadata"},
		{"1.2.0-x.Y.0+metadata-width-hypen", "1.2.0-x.Y.0+metadata-width-hypen"},
		{"v1.2.0-x.Y.0+metadata-width-hypen", "1.2.0-x.Y.0+metadata-width-hypen"},
		{"1.2.3-rc1-with-hypen", "1.2.3-rc1-with-hypen"},
		{"v1.2.3-rc1-with-hypen", "1.2.3-rc1-with-hypen"},
	}

	for _, tc := range tests {
		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("Error parsing version %s", tc)
		}

		s := v.String()
		if s != tc.expected {
			t.Errorf("Error generating string. Expected '%s' but got '%s'", tc.expected, s)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.2.3", "1.5.1", -1},
		{"2.2.3", "1.5.1", 1},
		{"2.2.3", "2.2.2", 1},
		{"3.2-beta", "3.2-beta", 0},
		{"1.3", "1.1.4", 1},
		{"4.2", "4.2-beta", 1},
		{"4.2-beta", "4.2", -1},
		{"4.2-alpha", "4.2-beta", -1},
		{"4.2-alpha", "4.2-alpha", 0},
		{"4.2-beta.2", "4.2-beta.1", 1},
		{"4.2-beta2", "4.2-beta1", 1},
		{"4.2-beta", "4.2-beta.2", -1},
		{"4.2-beta", "4.2-beta.foo", -1},
		{"4.2-beta.2", "4.2-beta", 1},
		{"4.2-beta.foo", "4.2-beta", 1},
		{"1.2+bar", "1.2+baz", 0},
		{"1.0.0-beta.4", "1.0.0-beta.-2", -1},
		{"1.0.0-beta.-2", "1.0.0-beta.-3", -1},
		{"1.0.0-beta.-3", "1.0.0-beta.5", 1},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := NewVersion(tc.v2)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		a := v1.Compare(v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%d', got '%d'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestLessThan(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"1.2.3", "1.5.1", true},
		{"2.2.3", "1.5.1", false},
		{"3.2-beta", "3.2-beta", false},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := NewVersion(tc.v2)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		a := v1.LessThan(v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%t', got '%t'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestLessThanEqual(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"1.2.3", "1.5.1", true},
		{"2.2.3", "1.5.1", false},
		{"1.5.1", "1.5.1", true},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := NewVersion(tc.v2)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		a := v1.LessThanEqual(v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%t', got '%t'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"1.2.3", "1.5.1", false},
		{"2.2.3", "1.5.1", true},
		{"3.2-beta", "3.2-beta", false},
		{"3.2.0-beta.1", "3.2.0-beta.5", false},
		{"3.2-beta.4", "3.2-beta.2", true},
		{"7.43.0-SNAPSHOT.99", "7.43.0-SNAPSHOT.103", false},
		{"7.43.0-SNAPSHOT.FOO", "7.43.0-SNAPSHOT.103", true},
		{"7.43.0-SNAPSHOT.99", "7.43.0-SNAPSHOT.BAR", false},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := NewVersion(tc.v2)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		a := v1.GreaterThan(v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%t', got '%t'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestGreaterThanEqual(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
	}{
		{"1.2.3", "1.5.1", false},
		{"2.2.3", "1.5.1", true},
		{"1.5.1", "1.5.1", true},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := NewVersion(tc.v2)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		a := v1.GreaterThanEqual(v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%t', got '%t'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		v1       *Version
		v2       *Version
		expected bool
	}{
		{MustParse("1.2.3"), MustParse("1.5.1"), false},
		{MustParse("2.2.3"), MustParse("1.5.1"), false},
		{MustParse("3.2-beta"), MustParse("3.2-beta"), true},
		{MustParse("3.2-beta+foo"), MustParse("3.2-beta+bar"), true},
		{nil, nil, true},
		{nil, MustParse("1.2.3"), false},
		{MustParse("1.2.3"), nil, false},
	}

	for _, tc := range tests {
		a := tc.v1.Equal(tc.v2)
		e := tc.expected
		if a != e {
			t.Errorf(
				"Comparison of '%s' and '%s' failed. Expected '%t', got '%t'",
				tc.v1, tc.v2, e, a,
			)
		}
	}
}

func TestInc(t *testing.T) {
	tests := []struct {
		v1               string
		expected         string
		how              string
		expectedOriginal string
	}{
		{"1.2.3", "1.2.4", "patch", "1.2.4"},
		{"v1.2.4", "1.2.5", "patch", "v1.2.5"},
		{"1.2.3", "1.3.0", "minor", "1.3.0"},
		{"v1.2.4", "1.3.0", "minor", "v1.3.0"},
		{"1.2.3", "2.0.0", "major", "2.0.0"},
		{"v1.2.4", "2.0.0", "major", "v2.0.0"},
		{"1.2.3+meta", "1.2.4", "patch", "1.2.4"},
		{"1.2.3-beta+meta", "1.2.3", "patch", "1.2.3"},
		{"v1.2.4-beta+meta", "1.2.4", "patch", "v1.2.4"},
		{"1.2.3-beta+meta", "1.3.0", "minor", "1.3.0"},
		{"v1.2.4-beta+meta", "1.3.0", "minor", "v1.3.0"},
		{"1.2.3-beta+meta", "2.0.0", "major", "2.0.0"},
		{"v1.2.4-beta+meta", "2.0.0", "major", "v2.0.0"},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}
		var v2 Version
		switch tc.how {
		case "patch":
			v2 = v1.IncPatch()
		case "minor":
			v2 = v1.IncMinor()
		case "major":
			v2 = v1.IncMajor()
		}

		a := v2.String()
		e := tc.expected
		if a != e {
			t.Errorf(
				"Inc %q failed. Expected %q got %q",
				tc.how, e, a,
			)
		}

		a = v2.Original()
		e = tc.expectedOriginal
		if a != e {
			t.Errorf(
				"Inc %q failed. Expected original %q got %q",
				tc.how, e, a,
			)
		}
	}
}

func TestSetPrerelease(t *testing.T) {
	tests := []struct {
		v1                 string
		prerelease         string
		expectedVersion    string
		expectedPrerelease string
		expectedOriginal   string
		expectedErr        error
	}{
		{"1.2.3", "**", "1.2.3", "", "1.2.3", ErrInvalidPrerelease},
		{"1.2.3", "beta", "1.2.3-beta", "beta", "1.2.3-beta", nil},
		{"v1.2.4", "beta", "1.2.4-beta", "beta", "v1.2.4-beta", nil},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := v1.SetPrerelease(tc.prerelease)
		if err != tc.expectedErr {
			t.Errorf("Expected to get err=%s, but got err=%s", tc.expectedErr, err)
		}

		a := v2.Prerelease()
		e := tc.expectedPrerelease
		if a != e {
			t.Errorf("Expected prerelease value=%q, but got %q", e, a)
		}

		a = v2.String()
		e = tc.expectedVersion
		if a != e {
			t.Errorf("Expected version string=%q, but got %q", e, a)
		}

		a = v2.Original()
		e = tc.expectedOriginal
		if a != e {
			t.Errorf("Expected version original=%q, but got %q", e, a)
		}
	}
}

func TestSetMetadata(t *testing.T) {
	tests := []struct {
		v1               string
		metadata         string
		expectedVersion  string
		expectedMetadata string
		expectedOriginal string
		expectedErr      error
	}{
		{"1.2.3", "**", "1.2.3", "", "1.2.3", ErrInvalidMetadata},
		{"1.2.3", "meta", "1.2.3+meta", "meta", "1.2.3+meta", nil},
		{"v1.2.4", "meta", "1.2.4+meta", "meta", "v1.2.4+meta", nil},
	}

	for _, tc := range tests {
		v1, err := NewVersion(tc.v1)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		v2, err := v1.SetMetadata(tc.metadata)
		if err != tc.expectedErr {
			t.Errorf("Expected to get err=%s, but got err=%s", tc.expectedErr, err)
		}

		a := v2.Metadata()
		e := tc.expectedMetadata
		if a != e {
			t.Errorf("Expected metadata value=%q, but got %q", e, a)
		}

		a = v2.String()
		e = tc.expectedVersion
		if e != a {
			t.Errorf("Expected version string=%q, but got %q", e, a)
		}

		a = v2.Original()
		e = tc.expectedOriginal
		if a != e {
			t.Errorf("Expected version original=%q, but got %q", e, a)
		}
	}
}

func TestOriginalVPrefix(t *testing.T) {
	tests := []struct {
		version string
		vprefix string
	}{
		{"1.2.3", ""},
		{"v1.2.4", "v"},
	}

	for _, tc := range tests {
		v1, _ := NewVersion(tc.version)
		a := v1.originalVPrefix()
		e := tc.vprefix
		if a != e {
			t.Errorf("Expected vprefix=%q, but got %q", e, a)
		}
	}
}

func TestJsonMarshal(t *testing.T) {
	sVer := "1.1.1"
	x, err := StrictNewVersion(sVer)
	if err != nil {
		t.Errorf("Error creating version: %s", err)
	}
	out, err2 := json.Marshal(x)
	if err2 != nil {
		t.Errorf("Error marshaling version: %s", err2)
	}
	got := string(out)
	want := fmt.Sprintf("%q", sVer)
	if got != want {
		t.Errorf("Error marshaling unexpected marshaled content: got=%q want=%q", got, want)
	}
}

func TestJsonUnmarshal(t *testing.T) {
	sVer := "1.1.1"
	ver := &Version{}
	err := json.Unmarshal([]byte(fmt.Sprintf("%q", sVer)), ver)
	if err != nil {
		t.Errorf("Error unmarshaling version: %s", err)
	}
	got := ver.String()
	want := sVer
	if got != want {
		t.Errorf("Error unmarshaling unexpected object content: got=%q want=%q", got, want)
	}
}

func TestTextMarshal(t *testing.T) {
	sVer := "1.1.1"

	x, err := StrictNewVersion(sVer)
	if err != nil {
		t.Errorf("Error creating version: %s", err)
	}

	out, err2 := x.MarshalText()
	if err2 != nil {
		t.Errorf("Error marshaling version: %s", err2)
	}

	got := string(out)
	want := sVer
	if got != want {
		t.Errorf("Error marshaling unexpected marshaled content: got=%q want=%q", got, want)
	}
}

func TestTextUnmarshal(t *testing.T) {
	sVer := "1.1.1"
	ver := &Version{}

	err := ver.UnmarshalText([]byte(sVer))
	if err != nil {
		t.Errorf("Error unmarshaling version: %s", err)
	}

	got := ver.String()
	want := sVer
	if got != want {
		t.Errorf("Error unmarshaling unexpected object content: got=%q want=%q", got, want)
	}
}

func TestSQLScanner(t *testing.T) {
	sVer := "1.1.1"
	x, err := StrictNewVersion(sVer)
	if err != nil {
		t.Errorf("Error creating version: %s", err)
	}
	var s sql.Scanner = x
	var out *Version
	var ok bool
	if out, ok = s.(*Version); !ok {
		t.Errorf("Error expected Version type, got=%T want=%T", s, Version{})
	}
	got := out.String()
	want := sVer
	if got != want {
		t.Errorf("Error sql scanner unexpected scan content: got=%q want=%q", got, want)
	}
}

func TestDriverValuer(t *testing.T) {
	sVer := "1.1.1"
	x, err := StrictNewVersion(sVer)
	if err != nil {
		t.Errorf("Error creating version: %s", err)
	}
	got, err := x.Value()
	if err != nil {
		t.Fatalf("Error getting value, got %v", err)
	}
	want := sVer
	if got != want {
		t.Errorf("Error driver valuer unexpected value content: got=%q want=%q", got, want)
	}
}

func TestValidatePrerelease(t *testing.T) {
	tests := []struct {
		pre      string
		expected error
	}{
		{"foo", nil},
		{"alpha.1", nil},
		{"alpha.01", ErrSegmentStartsZero},
		{"foo☃︎", ErrInvalidPrerelease},
		{"alpha.0-1", nil},
	}

	for _, tc := range tests {
		if err := validatePrerelease(tc.pre); err != tc.expected {
			t.Errorf("Unexpected error %q for prerelease %q", err, tc.pre)
		}
	}
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		meta     string
		expected error
	}{
		{"foo", nil},
		{"alpha.1", nil},
		{"alpha.01", nil},
		{"foo☃︎", ErrInvalidMetadata},
		{"alpha.0-1", nil},
		{"al-pha.1Phe70CgWe050H9K1mJwRUqTNQXZRERwLOEg37wpXUb4JgzgaD5YkL52ABnoyiE", nil},
	}

	for _, tc := range tests {
		if err := validateMetadata(tc.meta); err != tc.expected {
			t.Errorf("Unexpected error %q for metadata %q", err, tc.meta)
		}
	}
}

func FuzzNewVersion(f *testing.F) {
	testcases := []string{"v1.2.3", " ", "......", "1", "1.2.3-beta.1", "1.2.3+foo", "2.3.4-alpha.1+bar", "lorem ipsum"}

	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(_ *testing.T, a string) {
		_, _ = NewVersion(a)
	})
}

func FuzzStrictNewVersion(f *testing.F) {
	testcases := []string{"v1.2.3", " ", "......", "1", "1.2.3-beta.1", "1.2.3+foo", "2.3.4-alpha.1+bar", "lorem ipsum"}

	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(_ *testing.T, a string) {
		_, _ = StrictNewVersion(a)
	})
}
