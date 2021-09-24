package pretty

import (
	"encoding/json"
)

func Concise(o interface{}) string {
	bytes, err := json.Marshal(o)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func Verbose(o interface{}) string {
	bytes, err := json.MarshalIndent(o, "", "\t")
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}
