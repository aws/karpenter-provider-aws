package middleware

import "testing"

func TestMetadataClone(t *testing.T) {
	original := map[interface{}]interface{}{
		"abc": 123,
		"efg": "hij",
	}

	var m Metadata
	for k, v := range original {
		m.Set(k, v)
	}

	o := m.Clone()
	o.Set("unique", "value")
	for k := range original {
		if !o.Has(k) {
			t.Errorf("expect %v to be in cloned metadata", k)
		}
	}

	if !o.Has("unique") {
		t.Errorf("expect cloned metadata to have new entry")
	}
	if m.Has("unique") {
		t.Errorf("expect cloned metadata to not leak in to original")
	}
}
