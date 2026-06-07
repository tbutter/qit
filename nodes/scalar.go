package nodes

import (
	"fmt"
	"strings"
)

type concatFunc struct{}

func (c concatFunc) Type(argTypes []ColumnType) ColumnType {
	return ColumnType_STRING
}

func (c concatFunc) Eval(args []any) (any, error) {
	var sb strings.Builder
	for _, arg := range args {
		if arg != nil {
			sb.WriteString(fmt.Sprintf("%v", arg))
		}
	}
	return sb.String(), nil
}

func init() {
	RegisterFunction("concat", concatFunc{})
}
