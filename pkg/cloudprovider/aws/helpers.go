package aws

import (
	"bytes"
	"encoding/json"
)

func deepCopy[T any](v *T) (*T, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&buf)
	var cp T
	if err := dec.Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
}
