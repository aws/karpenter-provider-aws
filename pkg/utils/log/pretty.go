package log

import (
	"encoding/json"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	LinePrefix = ""
	IndentSize = "    "
)

func PrettyInfo(args ...interface{}) {
	var prettyArgs []interface{}
	for _, arg := range args {
		prettyArgs = append(prettyArgs, Pretty(arg))
	}
	zap.S().Info(prettyArgs...)
}

func PrettyInfof(formatter string, args ...interface{}) {
	var prettyArgs []interface{}
	for _, arg := range args {
		prettyArgs = append(prettyArgs, Pretty(arg))
	}
	zap.S().Infof(formatter, prettyArgs...)
}

func Pretty(object interface{}) string {
	if data, err := json.MarshalIndent(object, LinePrefix, IndentSize); err != nil {
		return errors.Wrap(err, "failed to print pretty string for object").Error()
	} else {
		return string(data)
	}
}
