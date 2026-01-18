package tests

//easyjson:json
type EscStringStruct struct {
	A string `json:"a"`
}

//easyjson:json
type EscIntStruct struct {
	A int `json:"a,string"`
}
