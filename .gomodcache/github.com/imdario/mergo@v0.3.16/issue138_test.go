package mergo_test

import (
	"encoding/json"
	"testing"

	"github.com/imdario/mergo"
)

const issue138configuration string = `
{
	"Port": 80
}
`

func TestIssue138(t *testing.T) {
	type config struct {
		Port uint16
	}
	type compatibleConfig struct {
		Port float64
	}

	foo := make(map[string]interface{})
	// encoding/json unmarshals numbers as float64
	// https://golang.org/pkg/encoding/json/#Unmarshal
	json.Unmarshal([]byte(issue138configuration), &foo)

	err := mergo.Map(&config{}, foo)
	if err == nil {
		t.Error("expected type mismatch error, got nil")
	} else {
		if err.Error() != "type mismatch on Port field: found float64, expected uint16" {
			t.Errorf("expected type mismatch error, got %q", err)
		}
	}

	c := compatibleConfig{}
	if err := mergo.Map(&c, foo); err != nil {
		t.Error(err)
	}
}
