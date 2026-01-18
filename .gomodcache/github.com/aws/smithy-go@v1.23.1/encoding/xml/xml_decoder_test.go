package xml

import (
	"bytes"
	"encoding/xml"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestXMLNodeDecoder_Token(t *testing.T) {
	cases := map[string]struct {
		responseBody         io.Reader
		expectedStartElement xml.StartElement
		expectedDone         bool
		expectedError        string
	}{
		"simple success case": {
			responseBody: bytes.NewReader([]byte(`<Response>abc</Response>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "",
				},
			},
			expectedDone: true,
		},
		"no value": {
			responseBody: bytes.NewReader([]byte(`<Response></Response>`)),
			expectedDone: true,
		},
		"empty body": {
			responseBody:  bytes.NewReader([]byte(``)),
			expectedError: "EOF",
		},
		"with indentation": {
			responseBody: bytes.NewReader([]byte(`	<Response><Struct>abc</Struct></Response>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "Struct",
				},
				Attr: []xml.Attr{},
			},
		},
		"with comment and indentation": {
			responseBody: bytes.NewReader([]byte(`<!--comment-->	<Response><Struct>abc</Struct></Response>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "Struct",
				},
				Attr: []xml.Attr{},
			},
		},
		"attr with namespace": {
			responseBody: bytes.NewReader([]byte(`<Response><Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="CanonicalUser"></Grantee></Response>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "Grantee",
				},
				Attr: []xml.Attr{
					{
						Name: xml.Name{
							Space: "xmlns",
							Local: "xsi",
						},
						Value: "http://www.w3.org/2001/XMLSchema-instance",
					},
					{
						Name: xml.Name{
							Space: "xsi",
							Local: "type",
						},
						Value: "CanonicalUser",
					},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			xmlDecoder := xml.NewDecoder(c.responseBody)
			st, err := FetchRootElement(xmlDecoder)
			if err != nil {
				if len(c.expectedError) == 0 {
					t.Fatalf("Expected no error, got %v", err)
				}

				if e, a := c.expectedError, err; !strings.Contains(err.Error(), c.expectedError) {
					t.Fatalf("expected error to contain %v, found %v", e, a.Error())
				}
			}
			nodeDecoder := WrapNodeDecoder(xmlDecoder, st)
			token, done, err := nodeDecoder.Token()
			if err != nil {
				if len(c.expectedError) == 0 {
					t.Fatalf("Expected no error, got %v", err)
				}

				if e, a := c.expectedError, err; !strings.Contains(err.Error(), c.expectedError) {
					t.Fatalf("expected error to contain %v, found %v", e, a.Error())
				}
			}

			if e, a := c.expectedDone, done; e != a {
				t.Fatalf("expected a valid end element token for the xml document, got none")
			}

			if !reflect.DeepEqual(c.expectedStartElement, token) {
				t.Fatalf("Found diff : %v != %v", c.expectedStartElement, token)
			}
		})
	}
}

func TestXMLNodeDecoder_TokenExample(t *testing.T) {
	responseBody := bytes.NewReader([]byte(`<Struct><Response>abc</Response></Struct>`))

	xmlDecoder := xml.NewDecoder(responseBody)
	// Fetches <Struct> tag as start element.
	st, err := FetchRootElement(xmlDecoder)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// nodeDecoder will track <Struct> tag as root node of the document
	nodeDecoder := WrapNodeDecoder(xmlDecoder, st)

	// Retrieves <Response> tag
	token, done, err := nodeDecoder.Token()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)

	}

	expect := xml.StartElement{Name: xml.Name{Local: "Response"}, Attr: []xml.Attr{}}
	if !reflect.DeepEqual(expect, token) {
		t.Fatalf("Found diff : %v != %v", expect, token)
	}
	if done {
		t.Fatalf("expected decoding to not be done yet")
	}

	// Skips the value and gets </Response> that is the end token of previously retrieved <Response> tag.
	// The way node decoder works it only keeps track of the root start tag using which it was initialized.
	// Here <Struct> is used to initialize, while</Response> is end element corresponding to already read
	// <Response> tag. We won't be done until we receive </Struct>
	token, done, err = nodeDecoder.Token()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)

	}

	expect = xml.StartElement{Name: xml.Name{Local: ""}, Attr: nil}
	if !reflect.DeepEqual(expect, token) {
		t.Fatalf("Found diff : %v != %v", expect, token)
	}
	if done {
		t.Fatalf("expected decoding to not be done yet")
	}

	// Retrieves </Struct> end element tag corresponding to <Struct> tag.
	// Since we got the end element that corresponds to the start element being track, we are done decoding.
	token, done, err = nodeDecoder.Token()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)

	}

	if !reflect.DeepEqual(expect, token) {
		t.Fatalf("%v != %v", expect, token)
	}
	if !done {
		t.Fatalf("expected decoding to be done as we fetched the end element </Struct>")
	}
}

func TestXMLNodeDecoder_Value(t *testing.T) {
	cases := map[string]struct {
		responseBody  io.Reader
		expectedValue []byte
		expectedDone  bool
		expectedError string
	}{
		"simple success case": {
			responseBody:  bytes.NewReader([]byte(`<Response>abc</Response>`)),
			expectedValue: []byte(`abc`),
		},
		"no value": {
			responseBody:  bytes.NewReader([]byte(`<Response></Response>`)),
			expectedValue: []byte{},
		},
		"self-closing": {
			responseBody:  bytes.NewReader([]byte(`<Response />`)),
			expectedValue: []byte{},
		},
		"empty body": {
			responseBody:  bytes.NewReader([]byte(``)),
			expectedError: "EOF",
		},
		"start element retrieved": {
			responseBody:  bytes.NewReader([]byte(`<Response><Struct>abc</Struct></Response>`)),
			expectedError: "expected value for Response element, got xml.StartElement type",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			xmlDecoder := xml.NewDecoder(c.responseBody)
			st, err := FetchRootElement(xmlDecoder)
			if err != nil {
				if len(c.expectedError) == 0 {
					t.Fatalf("Expected no error, got %v", err)
				}

				if e, a := c.expectedError, err; !strings.Contains(err.Error(), c.expectedError) {
					t.Fatalf("expected error to contain %v, found %v", e, a.Error())
				}
			}
			nodeDecoder := WrapNodeDecoder(xmlDecoder, st)
			token, err := nodeDecoder.Value()
			if err != nil {
				if len(c.expectedError) == 0 {
					t.Fatalf("Expected no error, got %v", err)
				}

				if e, a := c.expectedError, err; !strings.Contains(err.Error(), c.expectedError) {
					t.Fatalf("expected error to contain %v, found %v", e, a.Error())
				}
			}

			if !reflect.DeepEqual(c.expectedValue, token) {
				t.Fatalf("%v != %v", c.expectedValue, token)
			}
		})
	}
}

func Test_FetchXMLRootElement(t *testing.T) {
	cases := map[string]struct {
		responseBody         io.Reader
		expectedStartElement xml.StartElement
		expectedError        string
	}{
		"simple success case": {
			responseBody: bytes.NewReader([]byte(`<Response><Struct>abc</Struct></Response>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "Response",
				},
				Attr: []xml.Attr{},
			},
		},
		"empty body": {
			responseBody:  bytes.NewReader([]byte(``)),
			expectedError: "EOF",
		},
		"with indentation": {
			responseBody: bytes.NewReader([]byte(`	<ErrorResponse>
    <Error>
        <Type>Sender</Type>
        <Code>InvalidGreeting</Code>
        <Message>Hi</Message>
        <AnotherSetting>setting</AnotherSetting>
    </Error>
    <RequestId>foo-id</RequestId>
</ErrorResponse>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "ErrorResponse",
				},
				Attr: []xml.Attr{},
			},
		},
		"with preamble": {
			responseBody: bytes.NewReader([]byte(`<?xml version = "1.0" encoding = "UTF-8" standalone = "no" ?>
<ErrorResponse>
    <Error>
        <Type>Sender</Type>
        <Code>InvalidGreeting</Code>
        <Message>Hi</Message>
        <AnotherSetting>setting</AnotherSetting>
    </Error>
    <RequestId>foo-id</RequestId>
</ErrorResponse>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "ErrorResponse",
				},
				Attr: []xml.Attr{},
			},
		},
		"with comments": {
			responseBody: bytes.NewReader([]byte(`<!--Sample comment for testing-->
<?xml version = "1.0" encoding = "UTF-8" standalone = "no" ?>
<ErrorResponse>
    <Error>
        <Type>Sender</Type>
        <Code>InvalidGreeting</Code>
        <Message>Hi</Message>
        <AnotherSetting>setting</AnotherSetting>
    </Error>
    <RequestId>foo-id</RequestId>
</ErrorResponse>`)),
			expectedStartElement: xml.StartElement{
				Name: xml.Name{
					Local: "ErrorResponse",
				},
				Attr: []xml.Attr{},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			decoder := xml.NewDecoder(c.responseBody)
			st, err := FetchRootElement(decoder)
			if err != nil {
				if len(c.expectedError) == 0 {
					t.Fatalf("Expected no error, got %v", err)
				}

				if e, a := c.expectedError, err; !strings.Contains(err.Error(), c.expectedError) {
					t.Fatalf("expected error to contain %v, found %v", e, a.Error())
				}
			}

			if !reflect.DeepEqual(c.expectedStartElement, st) {
				t.Fatalf("Found diff : %v != %v", c.expectedStartElement, st)
			}
		})
	}
}
