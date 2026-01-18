// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package swag

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

func TestJSONToYAML(t *testing.T) {
	sd := `{"1":"the int key value","name":"a string value","y":"some value"}`
	var data JSONMapSlice
	require.NoError(t, json.Unmarshal([]byte(sd), &data))

	y, err := data.MarshalYAML()
	require.NoError(t, err)
	const expected = `"1": the int key value
name: a string value
y: some value
`
	assert.Equal(t, expected, string(y.([]byte)))

	nstd := `{"1":"the int key value","name":"a string value","y":"some value","tag":{"name":"tag name"}}`
	const nestpected = `"1": the int key value
name: a string value
y: some value
tag:
    name: tag name
`
	var ndata JSONMapSlice
	require.NoError(t, json.Unmarshal([]byte(nstd), &ndata))
	ny, err := ndata.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, nestpected, string(ny.([]byte)))

	ydoc, err := BytesToYAMLDoc([]byte(fixtures2224))
	require.NoError(t, err)
	b, err := YAMLToJSON(ydoc)
	require.NoError(t, err)

	var bdata JSONMapSlice
	require.NoError(t, json.Unmarshal(b, &bdata))

}

func TestJSONToYAMLWithNull(t *testing.T) {
	const (
		jazon    = `{"1":"the int key value","name":null,"y":"some value"}`
		expected = `"1": the int key value
name: null
y: some value
`
	)
	var data JSONMapSlice
	require.NoError(t, json.Unmarshal([]byte(jazon), &data))
	ny, err := data.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, expected, string(ny.([]byte)))
}

func TestMarshalYAML(t *testing.T) {
	t.Run("marshalYAML should be deterministic", func(t *testing.T) {
		const (
			jazon    = `{"1":"x","2":null,"3":{"a":1,"b":2,"c":3}}`
			expected = `"1": x
"2": null
"3":
    a: !!float 1
    b: !!float 2
    c: !!float 3
`
		)
		const iterations = 10
		for n := 0; n < iterations; n++ {
			var data JSONMapSlice
			require.NoError(t, json.Unmarshal([]byte(jazon), &data))
			ny, err := data.MarshalYAML()
			require.NoError(t, err)
			assert.Equal(t, expected, string(ny.([]byte)))
		}
	})
}

func TestYAMLToJSON(t *testing.T) {
	sd := `---
1: the int key value
name: a string value
'y': some value
`
	var data yaml.Node
	_ = yaml.Unmarshal([]byte(sd), &data)

	d, err := YAMLToJSON(data)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, `{"1":"the int key value","name":"a string value","y":"some value"}`, string(d))

	ns := []*yaml.Node{
		{
			Kind:  yaml.ScalarNode,
			Value: "true",
			Tag:   "!!bool",
		},
		{
			Kind:  yaml.ScalarNode,
			Value: "the bool value",
			Tag:   "!!str",
		},
	}
	data.Content[0].Content = append(data.Content[0].Content, ns...)
	d, err = YAMLToJSON(data)
	require.Error(t, err)
	require.Nil(t, d)

	data.Content[0].Content = data.Content[0].Content[:len(data.Content[0].Content)-2]

	tag := []*yaml.Node{
		{
			Kind:  yaml.ScalarNode,
			Value: "tag",
			Tag:   "!!str",
		},
		{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{
					Kind:  yaml.ScalarNode,
					Value: "name",
					Tag:   "!!str",
				},
				{
					Kind:  yaml.ScalarNode,
					Value: "tag name",
					Tag:   "!!str",
				},
			},
		},
	}
	data.Content[0].Content = append(data.Content[0].Content, tag...)

	d, err = YAMLToJSON(data)
	require.NoError(t, err)
	assert.Equal(t, `{"1":"the int key value","name":"a string value","y":"some value","tag":{"name":"tag name"}}`, string(d))

	tag[1].Content = []*yaml.Node{
		{
			Kind:  yaml.ScalarNode,
			Value: "true",
			Tag:   "!!bool",
		},
		{
			Kind:  yaml.ScalarNode,
			Value: "the bool tag name",
			Tag:   "!!str",
		},
	}

	d, err = YAMLToJSON(data)
	require.Error(t, err)
	require.Nil(t, d)

	var lst []interface{}
	lst = append(lst, "hello")

	d, err = YAMLToJSON(lst)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, []byte(`["hello"]`), []byte(d))

	lst = append(lst, data)

	d, err = YAMLToJSON(lst)
	require.Error(t, err)
	require.Nil(t, d)

	_, err = BytesToYAMLDoc([]byte("- name: hello\n"))
	require.Error(t, err)

	dd, err := BytesToYAMLDoc([]byte("description: 'object created'\n"))
	require.NoError(t, err)

	d, err = YAMLToJSON(dd)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{"description":"object created"}`), d)
}

var yamlPestoreServer = func(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(yamlPetStore))
}

func TestWithYKey(t *testing.T) {
	doc, err := BytesToYAMLDoc([]byte(withYKey))
	require.NoError(t, err)

	_, err = YAMLToJSON(doc)
	require.NoError(t, err)

	doc, err = BytesToYAMLDoc([]byte(withQuotedYKey))
	require.NoError(t, err)
	jsond, err := YAMLToJSON(doc)
	require.NoError(t, err)

	var yt struct {
		Definitions struct {
			Viewbox struct {
				Properties struct {
					Y struct {
						Type string `json:"type"`
					} `json:"y"`
				} `json:"properties"`
			} `json:"viewbox"`
		} `json:"definitions"`
	}
	require.NoError(t, json.Unmarshal(jsond, &yt))
	assert.Equal(t, "integer", yt.Definitions.Viewbox.Properties.Y.Type)
}

func TestMapKeyTypes(t *testing.T) {
	dm := map[interface{}]interface{}{
		12345:               "int",
		int8(1):             "int8",
		int16(12345):        "int16",
		int32(12345678):     "int32",
		int64(12345678910):  "int64",
		uint(12345):         "uint",
		uint8(1):            "uint8",
		uint16(12345):       "uint16",
		uint32(12345678):    "uint32",
		uint64(12345678910): "uint64",
	}
	_, err := YAMLToJSON(dm)
	require.NoError(t, err)
}

const fixtures2224 = `definitions:
  Time:
    type: string
    format: date-time
    x-go-type:
      import:
        package: time
      embedded: true
      type: Time
    x-nullable: true

  TimeAsObject:  # <- time.Time is actually a struct
    type: string
    format: date-time
    x-go-type:
      import:
        package: time
        hints:
          kind: object
      embedded: true
      type: Time
    x-nullable: true

  Raw:
    x-go-type:
      import:
        package: encoding/json
      hints:
        kind: primitive
      embedded: true
      type: RawMessage

  Request:
    x-go-type:
      import:
        package: net/http
      hints:
        kind: object
      embedded: true
      type: Request

  RequestPointer:
    x-go-type:
      import:
        package: net/http
      hints:
        kind: object
        nullable: true
      embedded: true
      type: Request

  OldStyleImport:
    type: object
    x-go-type:
      import:
        package: net/http
      type: Request
      hints:
        noValidation: true

  OldStyleRenamed:
    type: object
    x-go-type:
      import:
        package: net/http
      type: Request
      hints:
        noValidation: true
    x-go-name: OldRenamed

  ObjectWithEmbedded:
    type: object
    properties:
      a:
        $ref: '#/definitions/Time'
      b:
        $ref: '#/definitions/Request'
      c:
        $ref: '#/definitions/TimeAsObject'
      d:
        $ref: '#/definitions/Raw'
      e:
        $ref: '#/definitions/JSONObject'
      f:
        $ref: '#/definitions/JSONMessage'
      g:
        $ref: '#/definitions/JSONObjectWithAlias'

  ObjectWithExternals:
    type: object
    properties:
      a:
        $ref: '#/definitions/OldStyleImport'
      b:
        $ref: '#/definitions/OldStyleRenamed'

  Base:
    properties: &base
      id:
        type: integer
        format: uint64
        x-go-custom-tag: 'gorm:"primary_key"'
      FBID:
        type: integer
        format: uint64
        x-go-custom-tag: 'gorm:"index"'
      created_at:
        $ref: "#/definitions/Time"
      updated_at:
        $ref: "#/definitions/Time"
      version:
        type: integer
        format: uint64

  HotspotType:
    type: string
    enum:
      - A
      - B
      - C

  Hotspot:
    type: object
    allOf:
      - properties: *base
      - properties:
          access_points:
            type: array
            items:
              $ref: '#/definitions/AccessPoint'
          type:
            $ref: '#/definitions/HotspotType'
        required:
          - type

  AccessPoint:
    type: object
    allOf:
      - properties: *base
      - properties:
          mac_address:
            type: string
            x-go-custom-tag: 'gorm:"index;not null;unique"'
          hotspot_id:
            type: integer
            format: uint64
          hotspot:
            $ref: '#/definitions/Hotspot'

  JSONObject:
    type: object
    additionalProperties:
      type: array
      items:
        $ref: '#/definitions/Raw'

  JSONObjectWithAlias:
    type: object
    additionalProperties:
      type: object
      properties:
        message:
          $ref: '#/definitions/JSONMessage'

  JSONMessage:
    $ref: '#/definitions/Raw'

  Incorrect:
    x-go-type:
      import:
        package: net
        hints:
          kind: array
      embedded: true
      type: Buffers
    x-nullable: true
`

const withQuotedYKey = `consumes:
- application/json
definitions:
  viewBox:
    type: object
    properties:
      x:
        type: integer
        format: int16
      # y -> types don't match: expect map key string or int get: bool
      "y":
        type: integer
        format: int16
      width:
        type: integer
        format: int16
      height:
        type: integer
        format: int16
info:
  description: Test RESTful APIs
  title: Test Server
  version: 1.0.0
basePath: /api
paths:
  /test:
    get:
      operationId: findAll
      parameters:
        - name: since
          in: query
          type: integer
          format: int64
        - name: limit
          in: query
          type: integer
          format: int32
          default: 20
      responses:
        200:
          description: Array[Trigger]
          schema:
            type: array
            items:
              $ref: "#/definitions/viewBox"
produces:
- application/json
schemes:
- https
swagger: "2.0"
`

const withYKey = `consumes:
- application/json
definitions:
  viewBox:
    type: object
    properties:
      x:
        type: integer
        format: int16
      # y -> types don't match: expect map key string or int get: bool
      y:
        type: integer
        format: int16
      width:
        type: integer
        format: int16
      height:
        type: integer
        format: int16
info:
  description: Test RESTful APIs
  title: Test Server
  version: 1.0.0
basePath: /api
paths:
  /test:
    get:
      operationId: findAll
      parameters:
        - name: since
          in: query
          type: integer
          format: int64
        - name: limit
          in: query
          type: integer
          format: int32
          default: 20
      responses:
        200:
          description: Array[Trigger]
          schema:
            type: array
            items:
              $ref: "#/definitions/viewBox"
produces:
- application/json
schemes:
- https
swagger: "2.0"
`

const yamlPetStore = `swagger: '2.0'
info:
  version: '1.0.0'
  title: Swagger Petstore
  description: A sample API that uses a petstore as an example to demonstrate features in the swagger-2.0 specification
  termsOfService: http://helloreverb.com/terms/
  contact:
    name: Swagger API team
    email: foo@example.com
    url: http://swagger.io
  license:
    name: MIT
    url: http://opensource.org/licenses/MIT
host: petstore.swagger.wordnik.com
basePath: /api
schemes:
  - http
consumes:
  - application/json
produces:
  - application/json
paths:
  /pets:
    get:
      description: Returns all pets from the system that the user has access to
      operationId: findPets
      produces:
        - application/json
        - application/xml
        - text/xml
        - text/html
      parameters:
        - name: tags
          in: query
          description: tags to filter by
          required: false
          type: array
          items:
            type: string
          collectionFormat: csv
        - name: limit
          in: query
          description: maximum number of results to return
          required: false
          type: integer
          format: int32
      responses:
        '200':
          description: pet response
          schema:
            type: array
            items:
              $ref: '#/definitions/pet'
        default:
          description: unexpected error
          schema:
            $ref: '#/definitions/errorModel'
    post:
      description: Creates a new pet in the store.  Duplicates are allowed
      operationId: addPet
      produces:
        - application/json
      parameters:
        - name: pet
          in: body
          description: Pet to add to the store
          required: true
          schema:
            $ref: '#/definitions/newPet'
      responses:
        '200':
          description: pet response
          schema:
            $ref: '#/definitions/pet'
        default:
          description: unexpected error
          schema:
            $ref: '#/definitions/errorModel'
  /pets/{id}:
    get:
      description: Returns a user based on a single ID, if the user does not have access to the pet
      operationId: findPetById
      produces:
        - application/json
        - application/xml
        - text/xml
        - text/html
      parameters:
        - name: id
          in: path
          description: ID of pet to fetch
          required: true
          type: integer
          format: int64
      responses:
        '200':
          description: pet response
          schema:
            $ref: '#/definitions/pet'
        default:
          description: unexpected error
          schema:
            $ref: '#/definitions/errorModel'
    delete:
      description: deletes a single pet based on the ID supplied
      operationId: deletePet
      parameters:
        - name: id
          in: path
          description: ID of pet to delete
          required: true
          type: integer
          format: int64
      responses:
        '204':
          description: pet deleted
        default:
          description: unexpected error
          schema:
            $ref: '#/definitions/errorModel'
definitions:
  pet:
    required:
      - id
      - name
    properties:
      id:
        type: integer
        format: int64
      name:
        type: string
      tag:
        type: string
  newPet:
    allOf:
      - $ref: '#/definitions/pet'
      - required:
          - name
        properties:
          id:
            type: integer
            format: int64
          name:
            type: string
  errorModel:
    required:
      - code
      - message
    properties:
      code:
        type: integer
        format: int32
      message:
        type: string
`
