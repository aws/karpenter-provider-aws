package smithy

import "testing"

func TestProperties(t *testing.T) {
	original := map[interface{}]interface{}{
		"abc": 123,
		"efg": "hij",
	}

	var m Properties
	for k, v := range original {
		m.Set(k, v)
	}
	for k, v := range original {
		if m.Get(k) != v {
			t.Errorf("expect key / value properties to be equivalent: %v / %v", k, v)
		}
	}

	var n Properties
	n.SetAll(&m)
	for k, v := range original {
		if n.Get(k) != v {
			t.Errorf("expect key / value properties to be equivalent: %v / %v", k, v)
		}
	}

}