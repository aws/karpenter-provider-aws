package mergo_test

import (
	"reflect"
	"testing"

	"github.com/imdario/mergo"
)

func TestIssue202(t *testing.T) {
	tests := []struct {
		name           string
		dst, src, want map[string]interface{}
	}{
		{
			name: "slice override string",
			dst: map[string]interface{}{
				"x": 456,
				"y": "foo",
			},
			src: map[string]interface{}{
				"x": "123",
				"y": []int{1, 2, 3},
			},
			want: map[string]interface{}{
				"x": "123",
				"y": []int{1, 2, 3},
			},
		},
		{
			name: "string override slice",
			dst: map[string]interface{}{
				"x": 456,
				"y": []int{1, 2, 3},
			},
			src: map[string]interface{}{
				"x": "123",
				"y": "foo",
			},
			want: map[string]interface{}{
				"x": "123",
				"y": "foo",
			},
		},
		{
			name: "map override string",
			dst: map[string]interface{}{
				"x": 456,
				"y": "foo",
			},
			src: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
				},
			},
			want: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
				},
			},
		},
		{
			name: "string override map",
			dst: map[string]interface{}{
				"x": 456,
				"y": map[string]interface{}{
					"a": true,
				},
			},
			src: map[string]interface{}{
				"x": "123",
				"y": "foo",
			},
			want: map[string]interface{}{
				"x": "123",
				"y": "foo",
			},
		},
		{
			name: "map override map",
			dst: map[string]interface{}{
				"x": 456,
				"y": map[string]interface{}{
					"a": 10,
				},
			},
			src: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
				},
			},
			want: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
				},
			},
		},
		{
			name: "map override map with merge",
			dst: map[string]interface{}{
				"x": 456,
				"y": map[string]interface{}{
					"a": 10,
					"b": 100,
				},
			},
			src: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
				},
			},
			want: map[string]interface{}{
				"x": "123",
				"y": map[string]interface{}{
					"a": true,
					"b": 100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := mergo.Merge(&tt.dst, tt.src, mergo.WithOverride); err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(tt.dst, tt.want) {
				t.Errorf("maps not equal.\nwant:\n%v\ngot:\n%v\n", tt.want, tt.dst)
			}
		})
	}
}
