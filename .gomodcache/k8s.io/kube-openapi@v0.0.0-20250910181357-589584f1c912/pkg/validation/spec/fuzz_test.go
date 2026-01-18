/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spec

import (
	"github.com/go-openapi/jsonreference"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/randfill"
)

var SwaggerFuzzFuncs []interface{} = []interface{}{
	func(v *Responses, c randfill.Continue) {
		c.FillNoCustom(v)
		if v.Default != nil {
			// Check if we hit maxDepth and left an incomplete value
			if v.Default.Description == "" {
				v.Default = nil
				v.StatusCodeResponses = nil
			}
		}

		// conversion has no way to discern empty statusCodeResponses from
		// nil, since "default" is always included in the map.
		// So avoid empty responses list
		if len(v.StatusCodeResponses) == 0 {
			v.StatusCodeResponses = nil
		}
	},
	func(v *Operation, c randfill.Continue) {
		c.FillNoCustom(v)

		if v != nil {
			// force non-nil
			v.Responses = &Responses{}
			c.Fill(v.Responses)

			v.Schemes = nil
			if c.Bool() {
				v.Schemes = append(v.Schemes, "http")
			}

			if c.Bool() {
				v.Schemes = append(v.Schemes, "https")
			}

			if c.Bool() {
				v.Schemes = append(v.Schemes, "ws")
			}

			if c.Bool() {
				v.Schemes = append(v.Schemes, "wss")
			}

			// Gnostic unconditionally makes security values non-null
			// So do not fuzz null values into the array.
			for i, val := range v.Security {
				if val == nil {
					v.Security[i] = make(map[string][]string)
				}

				for k, v := range val {
					if v == nil {
						val[k] = make([]string, 0)
					}
				}
			}
		}
	},
	func(v map[int]Response, c randfill.Continue) {
		n := 0
		c.Fill(&n)
		if n == 0 {
			// Test that fuzzer is not at maxDepth so we do not
			// end up with empty elements
			return
		}

		// Prevent negative numbers
		num := c.Intn(4)
		for i := 0; i < num+2; i++ {
			val := Response{}
			c.Fill(&val)

			val.Description = c.String(0) + "x"
			v[100*(i+1)+c.Intn(100)] = val
		}
	},
	func(v map[string]PathItem, c randfill.Continue) {
		n := 0
		c.Fill(&n)
		if n == 0 {
			// Test that fuzzer is not at maxDepth so we do not
			// end up with empty elements
			return
		}

		num := c.Intn(5)
		for i := 0; i < num+2; i++ {
			val := PathItem{}
			c.Fill(&val)

			// Ref params are only allowed in certain locations, so
			// possibly add a few to PathItems
			numRefsToAdd := c.Intn(5)
			for i := 0; i < numRefsToAdd; i++ {
				theRef := Parameter{}
				c.Fill(&theRef.Refable)

				val.Parameters = append(val.Parameters, theRef)
			}

			v["/"+c.String(0)] = val
		}
	},
	func(v *SchemaOrArray, c randfill.Continue) {
		*v = SchemaOrArray{}
		// gnostic parser just doesn't support more
		// than one Schema here
		v.Schema = &Schema{}
		c.Fill(&v.Schema)

	},
	func(v *SchemaOrBool, c randfill.Continue) {
		*v = SchemaOrBool{}

		if c.Bool() {
			v.Allows = c.Bool()
		} else {
			v.Schema = &Schema{}
			v.Allows = true
			c.Fill(&v.Schema)
		}
	},
	func(v map[string]Response, c randfill.Continue) {
		n := 0
		c.Fill(&n)
		if n == 0 {
			// Test that fuzzer is not at maxDepth so we do not
			// end up with empty elements
			return
		}

		// Response definitions are not allowed to
		// be refs
		for i := 0; i < c.Intn(5)+1; i++ {
			resp := &Response{}

			c.Fill(resp)
			resp.Ref = Ref{}
			resp.Description = c.String(0) + "x"

			// Response refs are not vendor extensible by gnostic
			resp.VendorExtensible.Extensions = nil
			v[c.String(0)+"x"] = *resp
		}
	},
	func(v *Header, c randfill.Continue) {
		if v != nil {
			c.FillNoCustom(v)

			// descendant Items of Header may not be refs
			cur := v.Items
			for cur != nil {
				cur.Ref = Ref{}
				cur = cur.Items
			}
		}
	},
	func(v *Ref, c randfill.Continue) {
		*v = Ref{}
		v.Ref, _ = jsonreference.New("http://asd.com/" + c.String(0))
	},
	func(v *Response, c randfill.Continue) {
		*v = Response{}
		if c.Bool() {
			v.Ref = Ref{}
			v.Ref.Ref, _ = jsonreference.New("http://asd.com/" + c.String(0))
		} else {
			c.Fill(&v.VendorExtensible)
			c.Fill(&v.Schema)
			c.Fill(&v.ResponseProps)

			v.Headers = nil
			v.Ref = Ref{}

			n := 0
			c.Fill(&n)
			if n != 0 {
				// Test that fuzzer is not at maxDepth so we do not
				// end up with empty elements
				num := c.Intn(4)
				for i := 0; i < num; i++ {
					if v.Headers == nil {
						v.Headers = make(map[string]Header)
					}
					hdr := Header{}
					c.Fill(&hdr)
					if hdr.Type == "" {
						// hit maxDepth, just abort trying to make haders
						v.Headers = nil
						break
					}
					v.Headers[c.String(0)+"x"] = hdr
				}
			} else {
				v.Headers = nil
			}
		}

		v.Description = c.String(0) + "x"

		// Gnostic parses empty as nil, so to keep avoid putting empty
		if len(v.Headers) == 0 {
			v.Headers = nil
		}
	},
	func(v **Info, c randfill.Continue) {
		// Info is never nil
		*v = &Info{}
		c.FillNoCustom(*v)

		(*v).Title = c.String(0) + "x"
	},
	func(v *Extensions, c randfill.Continue) {
		// gnostic parser only picks up x- vendor extensions
		numChildren := c.Intn(5)
		for i := 0; i < numChildren; i++ {
			if *v == nil {
				*v = Extensions{}
			}
			(*v)["x-"+c.String(0)] = c.String(0)
		}
	},
	func(v *Swagger, c randfill.Continue) {
		c.FillNoCustom(v)

		if v.Paths == nil {
			// Force paths non-nil since it does not have omitempty in json tag.
			// This means a perfect roundtrip (via json) is impossible,
			// since we can't tell the difference between empty/unspecified paths
			v.Paths = &Paths{}
			c.Fill(v.Paths)
		}

		v.Swagger = "2.0"

		// Gnostic support serializing ID at all
		// unavoidable data loss
		v.ID = ""

		v.Schemes = nil
		if c.Uint64()%2 == 1 {
			v.Schemes = append(v.Schemes, "http")
		}

		if c.Uint64()%2 == 1 {
			v.Schemes = append(v.Schemes, "https")
		}

		if c.Uint64()%2 == 1 {
			v.Schemes = append(v.Schemes, "ws")
		}

		if c.Uint64()%2 == 1 {
			v.Schemes = append(v.Schemes, "wss")
		}

		// Gnostic unconditionally makes security values non-null
		// So do not fuzz null values into the array.
		for i, val := range v.Security {
			if val == nil {
				v.Security[i] = make(map[string][]string)
			}

			for k, v := range val {
				if v == nil {
					val[k] = make([]string, 0)
				}
			}
		}
	},
	func(v *SecurityScheme, c randfill.Continue) {
		v.Description = c.String(0) + "x"
		c.Fill(&v.VendorExtensible)

		switch c.Intn(3) {
		case 0:
			v.Type = "basic"
		case 1:
			v.Type = "apiKey"
			switch c.Intn(2) {
			case 0:
				v.In = "header"
			case 1:
				v.In = "query"
			default:
				panic("unreachable")
			}
			v.Name = "x" + c.String(0)
		case 2:
			v.Type = "oauth2"

			switch c.Intn(4) {
			case 0:
				v.Flow = "accessCode"
				v.TokenURL = "https://" + c.String(0)
				v.AuthorizationURL = "https://" + c.String(0)
			case 1:
				v.Flow = "application"
				v.TokenURL = "https://" + c.String(0)
			case 2:
				v.Flow = "implicit"
				v.AuthorizationURL = "https://" + c.String(0)
			case 3:
				v.Flow = "password"
				v.TokenURL = "https://" + c.String(0)
			default:
				panic("unreachable")
			}
			c.Fill(&v.Scopes)
		default:
			panic("unreachable")
		}
	},
	func(v *interface{}, c randfill.Continue) {
		*v = c.String(0) + "x"
	},
	func(v *string, c randfill.Continue) {
		*v = c.String(0) + "x"
	},
	func(v *ExternalDocumentation, c randfill.Continue) {
		v.Description = c.String(0) + "x"
		v.URL = c.String(0) + "x"
	},
	func(v *SimpleSchema, c randfill.Continue) {
		c.FillNoCustom(v)

		switch c.Intn(5) {
		case 0:
			v.Type = "string"
		case 1:
			v.Type = "number"
		case 2:
			v.Type = "boolean"
		case 3:
			v.Type = "integer"
		case 4:
			v.Type = "array"
		default:
			panic("unreachable")
		}

		switch c.Intn(5) {
		case 0:
			v.CollectionFormat = "csv"
		case 1:
			v.CollectionFormat = "ssv"
		case 2:
			v.CollectionFormat = "tsv"
		case 3:
			v.CollectionFormat = "pipes"
		case 4:
			v.CollectionFormat = ""
		default:
			panic("unreachable")
		}

		// None of the types which include SimpleSchema in our definitions
		// actually support "example" in the official spec
		v.Example = nil

		// unsupported by openapi
		v.Nullable = false
	},
	func(v *int64, c randfill.Continue) {
		c.Fill(v)

		// Gnostic does not differentiate between 0 and non-specified
		// so avoid using 0 for fuzzer
		if *v == 0 {
			*v = 1
		}
	},
	func(v *float64, c randfill.Continue) {
		c.Fill(v)

		// Gnostic does not differentiate between 0 and non-specified
		// so avoid using 0 for fuzzer
		if *v == 0.0 {
			*v = 1.0
		}
	},
	func(v *Parameter, c randfill.Continue) {
		if v == nil {
			return
		}
		c.Fill(&v.VendorExtensible)
		if c.Bool() {
			// body param
			v.Description = c.String(0) + "x"
			v.Name = c.String(0) + "x"
			v.In = "body"
			c.Fill(&v.Description)
			c.Fill(&v.Required)

			v.Schema = &Schema{}
			c.Fill(&v.Schema)

		} else {
			c.Fill(&v.SimpleSchema)
			c.Fill(&v.CommonValidations)
			v.AllowEmptyValue = false
			v.Description = c.String(0) + "x"
			v.Name = c.String(0) + "x"

			switch c.Intn(4) {
			case 0:
				// Header param
				v.In = "header"
			case 1:
				// Form data param
				v.In = "formData"
				v.AllowEmptyValue = c.Bool()
			case 2:
				// Query param
				v.In = "query"
				v.AllowEmptyValue = c.Bool()
			case 3:
				// Path param
				v.In = "path"
				v.Required = true
			default:
				panic("unreachable")
			}

			// descendant Items of Parameter may not be refs
			cur := v.Items
			for cur != nil {
				cur.Ref = Ref{}
				cur = cur.Items
			}
		}
	},
	func(v *Schema, c randfill.Continue) {
		if c.Bool() {
			// file schema
			c.Fill(&v.Default)
			c.Fill(&v.Description)
			c.Fill(&v.Example)
			c.Fill(&v.ExternalDocs)

			c.Fill(&v.Format)
			c.Fill(&v.ReadOnly)
			c.Fill(&v.Required)
			c.Fill(&v.Title)
			v.Type = StringOrArray{"file"}

		} else {
			// normal schema
			c.Fill(&v.SchemaProps)
			c.Fill(&v.SwaggerSchemaProps)
			c.Fill(&v.VendorExtensible)
			// c.Fill(&v.ExtraProps)
			// ExtraProps will not roundtrip - gnostic throws out
			// unrecognized keys
		}

		// Not supported by official openapi v2 spec
		// and stripped by k8s apiserver
		v.ID = ""
		v.AnyOf = nil
		v.OneOf = nil
		v.Not = nil
		v.Nullable = false
		v.AdditionalItems = nil
		v.Schema = ""
		v.PatternProperties = nil
		v.Definitions = nil
		v.Dependencies = nil
	},
}

var SwaggerDiffOptions = []cmp.Option{
	// cmp.Diff panics on Ref since jsonreference.Ref uses unexported fields
	cmp.Comparer(func(a Ref, b Ref) bool {
		return a.String() == b.String()
	}),
}
