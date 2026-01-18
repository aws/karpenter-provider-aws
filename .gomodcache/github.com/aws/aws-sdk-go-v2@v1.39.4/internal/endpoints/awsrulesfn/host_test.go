package awsrulesfn

import (
	"testing"
)

func TestIsVirtualHostableS3Bucket(t *testing.T) {
	cases := map[string]struct {
		input           string
		allowSubDomains bool
		expect          bool
	}{
		"single label no split": {
			input:  "abc123-",
			expect: true,
		},
		"single label no split too short": {
			input:  "a",
			expect: false,
		},
		"single label with split": {
			input:           "abc123-",
			allowSubDomains: true,
			expect:          true,
		},
		"multiple labels no split": {
			input:  "abc.123-",
			expect: false,
		},
		"multiple labels with split": {
			input:           "abc.123-",
			allowSubDomains: true,
			expect:          true,
		},
		"multiple labels with split invalid label": {
			input:           "abc.123-...",
			allowSubDomains: true,
			expect:          false,
		},
		"max length host label": {
			input:  "012345678901234567890123456789012345678901234567890123456789123",
			expect: true,
		},
		"too large host label": {
			input:  "0123456789012345678901234567890123456789012345678901234567891234",
			expect: false,
		},
		"too small host label": {
			input:  "",
			expect: false,
		},
		"lower case only": {
			input:  "AbC",
			expect: false,
		},
		"like IP address": {
			input:  "127.111.222.123",
			expect: false,
		},
		"multiple labels like IP address": {
			input:           "127.111.222.123",
			allowSubDomains: true,
			expect:          false,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actual := IsVirtualHostableS3Bucket(c.input, c.allowSubDomains)
			if e, a := c.expect, actual; e != a {
				t.Fatalf("expect %v hostable bucket, got %v", e, a)
			}
		})
	}
}
