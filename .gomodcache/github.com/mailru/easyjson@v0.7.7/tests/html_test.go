package tests

import (
	"testing"

	"github.com/mailru/easyjson/jwriter"
)

func TestHTML(t *testing.T) {
	s := Struct{
		Test: "<b>test</b>",
	}

	j := jwriter.Writer{
		NoEscapeHTML: false,
	}
	s.MarshalEasyJSON(&j)

	data, _ := j.BuildBytes()

	if string(data) != `{"Test":"\u003cb\u003etest\u003c/b\u003e"}` {
		t.Fatal("EscapeHTML error:", string(data))
	}

	j.NoEscapeHTML = true
	s.MarshalEasyJSON(&j)

	data, _ = j.BuildBytes()

	if string(data) != `{"Test":"<b>test</b>"}` {
		t.Fatal("NoEscapeHTML error:", string(data))
	}
}
