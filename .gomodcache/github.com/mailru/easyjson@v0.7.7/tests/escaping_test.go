package tests

import (
	"reflect"
	"testing"

	"github.com/mailru/easyjson"
)

func TestStrFieldsUnescaping(t *testing.T) {
	cases := []struct {
		data string
		exp  EscStringStruct
	}{
		{
			data: `{}`,
			exp:  EscStringStruct{},
		},
		{
			data: `{"a": "\""}`,
			exp:  EscStringStruct{A: `"`},
		},
		{
			data: `{"a": "\\"}`,
			exp:  EscStringStruct{A: `\`},
		},
		{
			data: `{"a": "\\\""}`,
			exp:  EscStringStruct{A: `\"`},
		},
		{
			data: `{"a": "\\\\'"}`,
			exp:  EscStringStruct{A: `\\'`},
		},
		{
			data: `{"a": "\t\\\nx\\\""}`,
			exp:  EscStringStruct{A: "\t\\\nx\\\""},
		},
		{
			data: `{"a": "\r\n"}`,
			exp:  EscStringStruct{A: "\r\n"},
		},
		{
			data: `{"a": "\r\n\u4e2D\u56fD\\\""}`,
			exp:  EscStringStruct{A: "\r\n中国\\\""},
		},
	}

	for i, c := range cases {
		var val EscStringStruct
		err := easyjson.Unmarshal([]byte(c.data), &val)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(val, c.exp) {
			t.Errorf("[%d] TestStrFieldsUnescaping(): got=%q, exp=%q", i, val, c.exp)
		}
	}
}

func TestIntFieldsUnescaping(t *testing.T) {
	cases := []struct {
		data string
		exp  EscIntStruct
	}{
		{
			data: `{}`,
			exp:  EscIntStruct{A: 0},
		},
		{
			data: `{"a": "1"}`,
			exp:  EscIntStruct{A: 1},
		},
		{
			data: `{"a": "\u0032"}`,
			exp:  EscIntStruct{A: 2},
		},
	}

	for i, c := range cases {
		var val EscIntStruct
		err := easyjson.Unmarshal([]byte(c.data), &val)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(val, c.exp) {
			t.Errorf("[%d] TestIntFieldsUnescaping(): got=%v, exp=%v", i, val, c.exp)
		}
	}
}
