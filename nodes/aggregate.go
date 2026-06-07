package nodes

import (
	sqlp "github.com/rqlite/sql"
)

type minFunc struct{}

func (m minFunc) Type(argTypes []ColumnType) ColumnType {
	if len(argTypes) > 0 {
		return argTypes[0]
	}
	return ColumnType_STRING
}

func (m minFunc) EvalGroup(args [][]any) (any, error) {
	var extreme any
	for _, rowArgs := range args {
		if len(rowArgs) == 0 || rowArgs[0] == nil {
			continue
		}
		val := rowArgs[0]
		if extreme == nil {
			extreme = val
			continue
		}
		isLess, err := compareValues(sqlp.LT, val, extreme)
		if err == nil && isLess {
			extreme = val
		}
	}
	return extreme, nil
}

type maxFunc struct{}

func (m maxFunc) Type(argTypes []ColumnType) ColumnType {
	if len(argTypes) > 0 {
		return argTypes[0]
	}
	return ColumnType_STRING
}

func (m maxFunc) EvalGroup(args [][]any) (any, error) {
	var extreme any
	for _, rowArgs := range args {
		if len(rowArgs) == 0 || rowArgs[0] == nil {
			continue
		}
		val := rowArgs[0]
		if extreme == nil {
			extreme = val
			continue
		}
		isGreater, err := compareValues(sqlp.GT, val, extreme)
		if err == nil && isGreater {
			extreme = val
		}
	}
	return extreme, nil
}

type countFunc struct{}

func (c countFunc) Type(argTypes []ColumnType) ColumnType {
	return ColumnType_INT
}

func (c countFunc) EvalGroup(args [][]any) (any, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args[0]) == 0 {
		return len(args), nil
	}
	cnt := 0
	for _, rowArgs := range args {
		if len(rowArgs) > 0 && rowArgs[0] != nil {
			cnt++
		}
	}
	return cnt, nil
}

type sumFunc struct{}

func (s sumFunc) Type(argTypes []ColumnType) ColumnType {
	if len(argTypes) > 0 {
		return argTypes[0]
	}
	return ColumnType_INT
}

func (s sumFunc) EvalGroup(args [][]any) (any, error) {
	var sumInt int
	var sumFloat float64
	var hasFloat bool
	var hasValue bool
	for _, rowArgs := range args {
		if len(rowArgs) == 0 || rowArgs[0] == nil {
			continue
		}
		hasValue = true
		switch v := rowArgs[0].(type) {
		case int:
			if hasFloat {
				sumFloat += float64(v)
			} else {
				sumInt += v
			}
		case float64:
			if !hasFloat {
				sumFloat = float64(sumInt)
				hasFloat = true
			}
			sumFloat += v
		}
	}
	if !hasValue {
		return nil, nil
	}
	if hasFloat {
		return sumFloat, nil
	}
	return sumInt, nil
}

func init() {
	RegisterFunction("min", minFunc{})
	RegisterFunction("max", maxFunc{})
	RegisterFunction("count", countFunc{})
	RegisterFunction("sum", sumFunc{})
}
