package govalidator

import (
	"testing"
)

func TestIsRequestURI(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"http://foo.bar/#com", true},
		{"http://foobar.com", true},
		{"https://foobar.com", true},
		{"foobar.com", false},
		{"http://foobar.coffee/", true},
		{"http://foobar.中文网/", true},
		{"http://foobar.org/", true},
		{"http://foobar.org:8080/", true},
		{"ftp://foobar.ru/", true},
		{"http://user:pass@www.foobar.com/", true},
		{"http://127.0.0.1/", true},
		{"http://duckduckgo.com/?q=%2F", true},
		{"http://localhost:3000/", true},
		{"http://foobar.com/?foo=bar#baz=qux", true},
		{"http://foobar.com?foo=bar", true},
		{"http://www.xn--froschgrn-x9a.net/", true},
		{"xyz://foobar.com", true},
		{"invalid.", false},
		{".com", false},
		{"rtmp://foobar.com", true},
		{"http://www.foo_bar.com/", true},
		{"http://localhost:3000/", true},
		{"http://foobar.com/#baz=qux", true},
		{"http://foobar.com/t$-_.+!*\\'(),", true},
		{"http://www.foobar.com/~foobar", true},
		{"http://www.-foobar.com/", true},
		{"http://www.foo---bar.com/", true},
		{"mailto:someone@example.com", true},
		{"irc://irc.server.org/channel", true},
		{"/abs/test/dir", true},
		{"./rel/test/dir", false},
	}
	for _, test := range tests {
		actual := IsRequestURI(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsRequestURI(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsHexcolor(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"#ff", false},
		{"fff0", false},
		{"#ff12FG", false},
		{"CCccCC", true},
		{"fff", true},
		{"#f00", true},
	}
	for _, test := range tests {
		actual := IsHexcolor(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsHexcolor(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsRGBcolor(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"rgb(0,31,255)", true},
		{"rgb(1,349,275)", false},
		{"rgb(01,31,255)", false},
		{"rgb(0.6,31,255)", false},
		{"rgba(0,31,255)", false},
		{"rgb(0,  31, 255)", true},
	}
	for _, test := range tests {
		actual := IsRGBcolor(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsRGBcolor(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsCreditCard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{"empty", "", false},
		{"not numbers", "credit card", false},
		{"invalid luhn algorithm", "4220855426213389", false},

		{"visa", "4220855426222389", true},
		{"visa spaces", "4220 8554 2622 2389", true},
		{"visa dashes", "4220-8554-2622-2389", true},
		{"mastercard", "5139288802098206", true},
		{"american express", "374953669708156", true},
		{"discover", "6011464355444102", true},
		{"jcb", "3548209662790989", true},

		// below should be valid, do they respect international standards?
		// is our validator logic not correct?
		{"diners club international", "30190239451016", false},
		{"rupay", "6521674451993089", false},
		{"mir", "2204151414444676", false},
		{"china unionPay", "624356436327468104", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCreditCard(tt.number); got != tt.want {
				t.Errorf("IsCreditCard(%v) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}

func TestIsISBN(t *testing.T) {
	t.Parallel()

	// Without version
	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"foo", false},
		{"3836221195", true},
		{"1-61729-085-8", true},
		{"3 423 21412 0", true},
		{"3 401 01319 X", true},
		{"9784873113685", true},
		{"978-4-87311-368-5", true},
		{"978 3401013190", true},
		{"978-3-8362-2119-1", true},
	}
	for _, test := range tests {
		actual := IsISBN(test.param, -1)
		if actual != test.expected {
			t.Errorf("Expected IsISBN(%q, -1) to be %v, got %v", test.param, test.expected, actual)
		}
	}

	// ISBN 10
	tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"foo", false},
		{"3423214121", false},
		{"978-3836221191", false},
		{"3-423-21412-1", false},
		{"3 423 21412 1", false},
		{"3836221195", true},
		{"1-61729-085-8", true},
		{"3 423 21412 0", true},
		{"3 401 01319 X", true},
	}
	for _, test := range tests {
		actual := IsISBN10(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsISBN10(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}

	// ISBN 13
	tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"foo", false},
		{"3-8362-2119-5", false},
		{"01234567890ab", false},
		{"978 3 8362 2119 0", false},
		{"9784873113685", true},
		{"978-4-87311-368-5", true},
		{"978 3401013190", true},
		{"978-3-8362-2119-1", true},
	}
	for _, test := range tests {
		actual := IsISBN13(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsISBN13(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsBase64(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"TG9yZW0gaXBzdW0gZG9sb3Igc2l0IGFtZXQsIGNvbnNlY3RldHVyIGFkaXBpc2NpbmcgZWxpdC4=", true},
		{"Vml2YW11cyBmZXJtZW50dW0gc2VtcGVyIHBvcnRhLg==", true},
		{"U3VzcGVuZGlzc2UgbGVjdHVzIGxlbw==", true},
		{"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuMPNS1Ufof9EW/M98FNw" +
			"UAKrwflsqVxaxQjBQnHQmiI7Vac40t8x7pIb8gLGV6wL7sBTJiPovJ0V7y7oc0Ye" +
			"rhKh0Rm4skP2z/jHwwZICgGzBvA0rH8xlhUiTvcwDCJ0kc+fh35hNt8srZQM4619" +
			"FTgB66Xmp4EtVyhpQV+t02g6NzK72oZI0vnAvqhpkxLeLiMCyrI416wHm5Tkukhx" +
			"QmcL2a6hNOyu0ixX/x2kSFXApEnVrJ+/IxGyfyw8kf4N2IZpW5nEP847lpfj0SZZ" +
			"Fwrd1mnfnDbYohX2zRptLy2ZUn06Qo9pkG5ntvFEPo9bfZeULtjYzIl6K8gJ2uGZ" + "HQIDAQAB", true},
		{"12345", false},
		{"", false},
		{"Vml2YW11cyBmZXJtZtesting123", false},
	}
	for _, test := range tests {
		actual := IsBase64(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsBase64(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIIsIPv6(t *testing.T) {
	t.Parallel()

	// IPv6
	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"127.0.0.1", false},
		{"0.0.0.0", false},
		{"255.255.255.255", false},
		{"1.2.3.4", false},
		{"::1", true},
		{"2001:db8:0000:1:1:1:1:1", true},
		{"300.0.0.0", false},
	}
	for _, test := range tests {
		actual := IsIPv6(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsIPv6(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsMAC(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"3D:F2:C9:A6:B3:4F", true},
		{"3D-F2-C9-A6-B3:4F", false},
		{"123", false},
		{"", false},
		{"abacaba", false},
	}
	for _, test := range tests {
		actual := IsMAC(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsMAC(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsSSN(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"", false},
		{"00-90-8787", false},
		{"66690-76", false},
		{"191 60 2869", true},
		{"191-60-2869", true},
	}
	for _, test := range tests {
		actual := IsSSN(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsSSN(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsInt(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"-2147483648", true},          //Signed 32 Bit Min Int
		{"2147483647", true},           //Signed 32 Bit Max Int
		{"-2147483649", true},          //Signed 32 Bit Min Int - 1
		{"2147483648", true},           //Signed 32 Bit Max Int + 1
		{"4294967295", true},           //Unsigned 32 Bit Max Int
		{"4294967296", true},           //Unsigned 32 Bit Max Int + 1
		{"-9223372036854775808", true}, //Signed 64 Bit Min Int
		{"9223372036854775807", true},  //Signed 64 Bit Max Int
		{"-9223372036854775809", true}, //Signed 64 Bit Min Int - 1
		{"9223372036854775808", true},  //Signed 64 Bit Max Int + 1
		{"18446744073709551615", true}, //Unsigned 64 Bit Max Int
		{"18446744073709551616", true}, //Unsigned 64 Bit Max Int + 1
		{"", true},
		{"123", true},
		{"0", true},
		{"-0", true},
		{"+0", true},
		{"01", false},
		{"123.123", false},
		{" ", false},
		{"000", false},
	}
	for _, test := range tests {
		actual := IsInt(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsInt(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}

func TestIsNull(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		param    string
		expected bool
	}{
		{"abacaba", false},
		{"", true},
	}
	for _, test := range tests {
		actual := IsNull(test.param)
		if actual != test.expected {
			t.Errorf("Expected IsNull(%q) to be %v, got %v", test.param, test.expected, actual)
		}
	}
}
