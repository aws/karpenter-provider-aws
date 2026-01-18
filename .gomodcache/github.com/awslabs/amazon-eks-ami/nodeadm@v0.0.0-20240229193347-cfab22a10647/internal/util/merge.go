package util

import (
	"encoding/json"

	"dario.cat/mergo"
)

// DocumentMerge merges two arguments using their marshalled json
// representations and returns the resulting data in a `map[string]interface{}`
func DocumentMerge(a, b any, opts ...func(*mergo.Config)) (map[string]interface{}, error) {
	var aMap, bMap map[string]interface{}
	aBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(aBytes, &aMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bBytes, &bMap); err != nil {
		return nil, err
	}
	if err := mergo.Merge(&aMap, &bMap, opts...); err != nil {
		return nil, err
	}
	return aMap, nil
}
