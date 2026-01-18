package tests

//easyjson:json
type NoIntern struct {
	Field string `json:"field"`
}

//easyjson:json
type Intern struct {
	Field string `json:"field,intern"`
}

var intern = Intern{Field: "interned"}
var internString = `{"field":"interned"}`
