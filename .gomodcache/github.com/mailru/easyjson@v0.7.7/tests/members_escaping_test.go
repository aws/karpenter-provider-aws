package tests

import (
	"reflect"
	"testing"

	"github.com/mailru/easyjson"
)

func TestMembersEscaping(t *testing.T) {
	cases := []struct {
		data  string
		esc   MembersEscaped
		unesc MembersUnescaped
	}{
		{
			data:  `{"漢語": "中国"}`,
			esc:   MembersEscaped{A: "中国"},
			unesc: MembersUnescaped{A: "中国"},
		},
		{
			data:  `{"漢語": "\u4e2D\u56fD"}`,
			esc:   MembersEscaped{A: "中国"},
			unesc: MembersUnescaped{A: "中国"},
		},
		{
			data:  `{"\u6f22\u8a9E": "中国"}`,
			esc:   MembersEscaped{A: "中国"},
			unesc: MembersUnescaped{A: ""},
		},
		{
			data:  `{"\u6f22\u8a9E": "\u4e2D\u56fD"}`,
			esc:   MembersEscaped{A: "中国"},
			unesc: MembersUnescaped{A: ""},
		},
	}

	for i, c := range cases {
		var esc MembersEscaped
		err := easyjson.Unmarshal([]byte(c.data), &esc)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(esc, c.esc) {
			t.Errorf("[%d] TestMembersEscaping(): got=%+v, exp=%+v", i, esc, c.esc)
		}

		var unesc MembersUnescaped
		err = easyjson.Unmarshal([]byte(c.data), &unesc)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(unesc, c.unesc) {
			t.Errorf("[%d] TestMembersEscaping(): no-unescaping case: got=%+v, exp=%+v", i, esc, c.esc)
		}
	}
}
