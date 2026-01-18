package json

import (
	"bytes"
	"encoding/json"
	"testing"

	smithytesting "github.com/aws/smithy-go/testing"
)

func TestDiscardUnknownField(t *testing.T) {
	cases := map[string][]byte{
		"empty object":  []byte(`{}`),
		"simple object": []byte(`{"foo": "bar"}`),
		"nested object": []byte(`{"foo": {"bar": "baz"}}`),
		"empty list":    []byte(`[]`),
		"simple list":   []byte(`["foo", "bar", "baz"]`),
		"nested list":   []byte(`["foo", ["bar", ["baz"]]]`),
		"number":        []byte(`1`),
		"boolean":       []byte(`true`),
		"null":          []byte(`null`),
		"string":        []byte(`"foo"`),
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			buff := bytes.NewBuffer(c)
			decoder := json.NewDecoder(buff)
			err := DiscardUnknownField(decoder)
			if err != nil {
				t.Fatalf("failed to discard, %v", err)
			}
			if decoder.More() {
				t.Fatalf("failed to discard entire contents")
			}
		})
	}
}

func TestCollectUnknownField(t *testing.T) {
	cases := map[string][]byte{
		"empty object":  []byte(`{}`),
		"simple object": []byte(`{"foo": "bar"}`),
		"nested object": []byte(`{"foo": {"bar": "baz"}}`),
		"empty list":    []byte(`[]`),
		"simple list":   []byte(`["foo", "bar", "baz"]`),
		"nested list":   []byte(`["foo", ["bar", ["baz"]]]`),
		"number":        []byte(`1`),
		"boolean":       []byte(`true`),
		"null":          []byte(`null`),
		"string":        []byte(`"foo"`),
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			buff := bytes.NewBuffer(c)
			decoder := json.NewDecoder(buff)
			actual, err := CollectUnknownField(decoder)
			if err != nil {
				t.Fatalf("failed to collect, %v", err)
			}
			smithytesting.AssertJSONEqual(t, c, actual)
		})
	}
}
