package tests

//easyjson:skip
type TypeSkipped struct {
	Value string
}

type TypeNotSkipped struct {
	Value string
}

var (
	myTypeNotSkippedValue  = TypeDeclared{Value: "TypeNotSkipped"}
	myTypeNotSkippedString = `{"Value":"TypeNotSkipped"}`
)
