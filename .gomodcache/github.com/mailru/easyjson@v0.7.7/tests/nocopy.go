package tests

//easyjson:json
type NocopyStruct struct {
	A string `json:"a"`
	B string `json:"b,nocopy"`
}
