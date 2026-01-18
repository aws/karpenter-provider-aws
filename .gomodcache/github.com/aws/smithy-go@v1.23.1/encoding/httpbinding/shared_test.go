package httpbinding

import (
	"fmt"
	"reflect"
)

func reflectCall(funcValue reflect.Value, args []interface{}) error {
	argValues := make([]reflect.Value, len(args))

	for i, v := range args {
		value := reflect.ValueOf(v)
		argValues[i] = value
	}

	retValues := funcValue.Call(argValues)
	if len(retValues) > 0 {
		errValue := retValues[0]

		if typeName := errValue.Type().Name(); typeName != "error" {
			panic(fmt.Sprintf("expected first return argument to be error but got %v", typeName))
		}

		if errValue.IsNil() {
			return nil
		}

		if err, ok := errValue.Interface().(error); ok {
			return err
		}

		panic(fmt.Sprintf("expected %v to return error type, but got %v", funcValue.Type().String(), retValues[0].Type().String()))
	}

	return nil
}
