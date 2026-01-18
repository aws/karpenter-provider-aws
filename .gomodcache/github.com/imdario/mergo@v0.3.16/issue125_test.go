package mergo_test

import (
	"encoding/json"
	"testing"

	"github.com/imdario/mergo"
)

type settings struct {
	FirstSlice  []string `json:"FirstSlice"`
	SecondSlice []string `json:"SecondSlice"`
}

func TestIssue125MergeWithOverwrite(t *testing.T) {
	var (
		defaultSettings = settings{
			FirstSlice:  []string{},
			SecondSlice: []string{},
		}
		something settings
		data      = `{"FirstSlice":[], "SecondSlice": null}`
	)

	if err := json.Unmarshal([]byte(data), &something); err != nil {
		t.Errorf("Error while Unmarshalling maprequest: %s", err)
	}

	if err := mergo.Merge(&something, defaultSettings, mergo.WithOverrideEmptySlice); err != nil {
		t.Errorf("Error while merging: %s", err)
	}

	if something.FirstSlice == nil {
		t.Error("Invalid merging first slice")
	}

	if something.SecondSlice == nil {
		t.Error("Invalid merging second slice")
	}
}
