package nodes

import (
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	sqlp "github.com/rqlite/sql"
)

type ColumnType int

const (
	ColumnType_INT    = 1
	ColumnType_STRING = 2
	ColumnType_FLOAT  = 3
	ColumnType_DATE   = 4
	ColumnType_COMPLEX = 5
	ColumnType_ARRAY  = 6
)

type ColumnDef struct {
	Name      string
	Type      ColumnType
	Qualifier string
}

type Row struct {
	Value []any
	Group []Row
}

// NodeImpl represents the internal interface for execution nodes.
// Source nodes in the sources package implement this interface.
type NodeImpl interface {
	Types() []ColumnDef
	All() iter.Seq[Row]
}

// Node is the wrapper struct representing a relational algebra node.
type Node struct {
	impl NodeImpl
}

// NewNode creates a Node wrapping a NodeImpl. Used by the sources package.
func NewNode(impl NodeImpl) *Node {
	return &Node{impl: impl}
}

func (n *Node) Types() []ColumnDef {
	return n.impl.Types()
}

func (n *Node) All() iter.Seq[Row] {
	return n.impl.All()
}

func NodeFromStatement(stmtIn sqlp.Statement) *Node {
	stmt := stmtIn.(*sqlp.SelectStatement)
	sourceNode := nodeFromSource(stmt.Source)

	var currentNode *Node = sourceNode
	if stmt.WhereExpr != nil {
		currentNode = NewFilterNode(currentNode, stmt.WhereExpr)
	}

	if len(stmt.GroupByExprs) > 0 {
		currentNode = NewGroupByNode(currentNode, stmt.GroupByExprs)
	} else if hasAggregates(stmt.Columns) {
		currentNode = NewGroupByNode(currentNode, nil)
	}

	if len(stmt.OrderingTerms) > 0 {
		currentNode = NewOrderNode(currentNode, stmt.OrderingTerms, stmt.Columns)
	}

	return NewSelectNode(stmt.Columns, currentNode)
}

func hasAggregates(cols []*sqlp.ResultColumn) bool {
	for _, col := range cols {
		if col.Expr != nil {
			if isAggregateCall(col.Expr) {
				return true
			}
		}
	}
	return false
}

func isAggregateCall(expr sqlp.Expr) bool {
	if expr == nil {
		return false
	}
	if call, ok := expr.(*sqlp.Call); ok {
		name := strings.ToLower(call.Name.Name)
		if fn, exists := functions[name]; exists {
			_, isAgg := fn.(AggregateFunction)
			return isAgg
		}
	}
	return false
}

// SourceFactory is a function that creates a source node from a table function name and arguments.
// It returns the node, a default alias, and an error.
type SourceFactory func(name string, args []sqlp.Expr) (*Node, string, error)

// DefaultSourceFactory is a function that creates a default source node (when no table function matches).
type DefaultSourceFactory func() *Node

var sourceFactory SourceFactory
var defaultSourceFactory DefaultSourceFactory

// RegisterSourceFactory registers the factory used by nodeFromSource to create source nodes.
func RegisterSourceFactory(factory SourceFactory, defaultFactory DefaultSourceFactory) {
	sourceFactory = factory
	defaultSourceFactory = defaultFactory
}

func nodeFromSource(src sqlp.Source) *Node {
	switch s := src.(type) {
	case *sqlp.QualifiedTableFunctionName:
		var node *Node
		var defaultAlias string

		if sourceFactory != nil {
			var err error
			node, defaultAlias, err = sourceFactory(s.Name.Name, s.Args)
			if err != nil {
				panic(err)
			}
		}

		if node != nil {
			if s.Alias != nil {
				return NewAliasNode(node, s.Alias.Name)
			}
			if defaultAlias != "" {
				return NewAliasNode(node, defaultAlias)
			}
			return node
		}

	case *sqlp.QualifiedTableName:
		node := getDefaultNode()
		if s.Alias != nil {
			return NewAliasNode(node, s.Alias.Name)
		}
		return NewAliasNode(node, s.Name.Name)

	case *sqlp.JoinClause:
		leftNode := nodeFromSource(s.X)
		rightNode := nodeFromSource(s.Y)

		joinType := "JOIN"
		if s.Operator != nil {
			if s.Operator.Left.IsValid() {
				joinType = "LEFT"
			}
		}

		var constraint sqlp.Expr
		if s.Constraint != nil {
			if on, ok := s.Constraint.(*sqlp.OnConstraint); ok {
				constraint = on.X
			}
		}

		return NewJoinNode(leftNode, rightNode, joinType, constraint)

	case *sqlp.ParenSource:
		var node *Node
		if selectStmt, ok := s.X.(*sqlp.SelectStatement); ok {
			node = NodeFromStatement(selectStmt)
		} else {
			node = nodeFromSource(s.X)
		}
		if s.Alias != nil {
			node = NewAliasNode(node, s.Alias.Name)
		}
		return node
	}

	// Default
	return getDefaultNode()
}

func getDefaultNode() *Node {
	if defaultSourceFactory != nil {
		return defaultSourceFactory()
	}
	panic("no default source factory registered; import _ \"github.com/tbutter/qit/sources\"")
}



// NewFilterNode creates a filter node wrapping a source node and applying a WHERE clause.
func NewFilterNode(source *Node, where sqlp.Expr) *Node {
	return &Node{
		impl: &FilterNode{
			source: source,
			where:  where,
		},
	}
}

// FilterNode filters rows according to the WHERE expression.
type FilterNode struct {
	source *Node
	where  sqlp.Expr
}

func (f *FilterNode) Types() []ColumnDef {
	return f.source.Types()
}

func (f *FilterNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		for row := range f.source.All() {
			matched, err := evalFilter(f.where, row, f.source.Types())
			if err != nil {
				continue // skip row on evaluation error
			}
			if matched {
				if !yield(row) {
					return
				}
			}
		}
	}
}

// NewSelectNode creates a projection node wrapping a source node.
func NewSelectNode(columns []*sqlp.ResultColumn, source *Node) *Node {
	return &Node{
		impl: &SelectNode{
			source:  source,
			columns: columns,
		},
	}
}

// SelectNode projects specified columns.
type SelectNode struct {
	source  *Node
	columns []*sqlp.ResultColumn
}

func (s *SelectNode) Types() []ColumnDef {
	var cols []ColumnDef
	sourceCols := s.source.Types()
	for _, col := range s.columns {
		if col.Star.IsValid() {
			cols = append(cols, sourceCols...)
			continue
		}

		if qref, ok := col.Expr.(*sqlp.QualifiedRef); ok && qref.Star.IsValid() {
			tableName := qref.Table.Name
			for _, sourceCol := range sourceCols {
				if strings.EqualFold(sourceCol.Qualifier, tableName) {
					cols = append(cols, sourceCol)
				}
			}
			continue
		}

		var name string
		if col.Alias != nil {
			name = col.Alias.Name
		} else if col.Expr != nil {
			name = col.Expr.String()
			if ident, ok := col.Expr.(*sqlp.Ident); ok {
				name = ident.Name
			} else if qref, ok := col.Expr.(*sqlp.QualifiedRef); ok {
				name = qref.Column.Name
			}
		}

		colType := exprType(col.Expr, sourceCols)
		cols = append(cols, ColumnDef{Name: name, Type: colType})
	}
	return cols
}

func (s *SelectNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		sourceCols := s.source.Types()
		for row := range s.source.All() {
			var newValues []any
			for _, col := range s.columns {
				if col.Star.IsValid() {
					newValues = append(newValues, row.Value...)
					continue
				}

				if qref, ok := col.Expr.(*sqlp.QualifiedRef); ok && qref.Star.IsValid() {
					tableName := qref.Table.Name
					for i, sourceCol := range sourceCols {
						if strings.EqualFold(sourceCol.Qualifier, tableName) {
							newValues = append(newValues, row.Value[i])
						}
					}
					continue
				}

				val, err := evalExpr(col.Expr, row, sourceCols)
				if err != nil {
					val = nil
				}
				newValues = append(newValues, val)
			}
			if !yield(Row{Value: newValues, Group: row.Group}) {
				return
			}
		}
	}
}

func evalFilter(expr sqlp.Expr, row Row, cols []ColumnDef) (bool, error) {
	val, err := evalExpr(expr, row, cols)
	if err != nil {
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, nil
}

func evalExpr(expr sqlp.Expr, row Row, cols []ColumnDef) (any, error) {
	if expr == nil {
		return nil, nil
	}
	switch e := expr.(type) {
	case *sqlp.Ident:
		for i, col := range cols {
			if strings.EqualFold(col.Name, e.Name) {
				if i < len(row.Value) {
					return row.Value[i], nil
				}
				return nil, fmt.Errorf("column index out of bounds")
			}
		}
		return nil, fmt.Errorf("column %s not found", e.Name)

	case *sqlp.QualifiedRef:
		schemaName := ""
		if e.Schema != nil {
			schemaName = e.Schema.Name
		}
		tableName := ""
		if e.Table != nil {
			tableName = e.Table.Name
		}
		columnName := ""
		if e.Column != nil {
			columnName = e.Column.Name
		}

		// 1. Check if it's t.f.k (where f is COMPLEX and k is key)
		if schemaName != "" && tableName != "" {
			for i, col := range cols {
				if strings.EqualFold(col.Name, tableName) && col.Type == ColumnType_COMPLEX {
					if strings.EqualFold(col.Qualifier, schemaName) {
						if i < len(row.Value) {
							val := row.Value[i]
							return getComplexValue(val, columnName)
						}
						return nil, fmt.Errorf("column index out of bounds")
					}
				}
			}
		}

		// 2. Check if it's f.k (where f is COMPLEX and k is key)
		if schemaName == "" && tableName != "" {
			for i, col := range cols {
				if strings.EqualFold(col.Name, tableName) && col.Type == ColumnType_COMPLEX {
					if i < len(row.Value) {
						val := row.Value[i]
						return getComplexValue(val, columnName)
					}
					return nil, fmt.Errorf("column index out of bounds")
				}
			}
		}

		// 3. Otherwise, treat it as standard table/alias qualifier.
		for i, col := range cols {
			if strings.EqualFold(col.Name, columnName) {
				qual := tableName
				if schemaName != "" {
					qual = schemaName
				}
				if qual == "" || strings.EqualFold(col.Qualifier, qual) {
					if i < len(row.Value) {
						return row.Value[i], nil
					}
					return nil, fmt.Errorf("column index out of bounds")
				}
			}
		}
		return nil, fmt.Errorf("column %s.%s not found", tableName, columnName)

	case *sqlp.NumberLit:
		if val, err := strconv.Atoi(e.Value); err == nil {
			return val, nil
		}
		if val, err := strconv.ParseFloat(e.Value, 64); err == nil {
			return val, nil
		}
		return nil, fmt.Errorf("invalid number literal: %s", e.Value)

	case *sqlp.StringLit:
		return e.Value, nil

	case *sqlp.Call:
		name := strings.ToLower(e.Name.Name)
		fn, exists := functions[name]
		if !exists {
			return nil, fmt.Errorf("unsupported function: %s", name)
		}

		if aggFn, ok := fn.(AggregateFunction); ok {
			groupRows := row.Group
			if len(groupRows) == 0 {
				groupRows = []Row{row}
			}

			var groupArgs [][]any
			for _, r := range groupRows {
				var rowArgs []any
				for _, argExpr := range e.Args {
					val, err := evalExpr(argExpr, r, cols)
					if err != nil {
						val = nil
					}
					rowArgs = append(rowArgs, val)
				}
				groupArgs = append(groupArgs, rowArgs)
			}
			return aggFn.EvalGroup(groupArgs)
		}

		if scalarFn, ok := fn.(ScalarFunction); ok {
			var rowArgs []any
			for _, argExpr := range e.Args {
				val, err := evalExpr(argExpr, row, cols)
				if err != nil {
					return nil, err
				}
				rowArgs = append(rowArgs, val)
			}
			return scalarFn.Eval(rowArgs)
		}

		return nil, fmt.Errorf("unknown function type for: %s", name)

	case *sqlp.BinaryExpr:
		lhs, err := evalExpr(e.X, row, cols)
		if err != nil {
			return nil, err
		}
		rhs, err := evalExpr(e.Y, row, cols)
		if err != nil {
			return nil, err
		}
		if e.Op == sqlp.PLUS || e.Op == sqlp.MINUS {
			return arithmeticEval(e.Op, lhs, rhs)
		}
		return compareValues(e.Op, lhs, rhs)

	case *sqlp.CastExpr:
		val, err := evalExpr(e.X, row, cols)
		if err != nil {
			return nil, err
		}
		destTypeName := strings.ToUpper(e.Type.Name.Name)
		return performCast(val, destTypeName)
	}

	return nil, fmt.Errorf("unsupported expression type: %T", expr)
}

func compareValues(op sqlp.Token, lhs, rhs any) (bool, error) {
	switch lVal := lhs.(type) {
	case int:
		switch rVal := rhs.(type) {
		case int:
			return compareInts(op, lVal, rVal)
		case float64:
			return compareFloats(op, float64(lVal), rVal)
		}
	case float64:
		switch rVal := rhs.(type) {
		case int:
			return compareFloats(op, lVal, float64(rVal))
		case float64:
			return compareFloats(op, lVal, rVal)
		}
	case string:
		switch rVal := rhs.(type) {
		case string:
			return compareStrings(op, lVal, rVal)
		case time.Time:
			if lDate, err := parseDate(lVal); err == nil {
				return compareDates(op, lDate, rVal)
			}
		}
	case time.Time:
		switch rVal := rhs.(type) {
		case time.Time:
			return compareDates(op, lVal, rVal)
		case string:
			if rDate, err := parseDate(rVal); err == nil {
				return compareDates(op, lVal, rDate)
			}
		}
	}
	return false, fmt.Errorf("cannot compare type %T and %T", lhs, rhs)
}

func compareInts(op sqlp.Token, a, b int) (bool, error) {
	switch op {
	case sqlp.EQ:
		return a == b, nil
	case sqlp.NE:
		return a != b, nil
	case sqlp.LT:
		return a < b, nil
	case sqlp.LE:
		return a <= b, nil
	case sqlp.GT:
		return a > b, nil
	case sqlp.GE:
		return a >= b, nil
	}
	return false, fmt.Errorf("unsupported operator: %s", op.String())
}

func compareFloats(op sqlp.Token, a, b float64) (bool, error) {
	switch op {
	case sqlp.EQ:
		return a == b, nil
	case sqlp.NE:
		return a != b, nil
	case sqlp.LT:
		return a < b, nil
	case sqlp.LE:
		return a <= b, nil
	case sqlp.GT:
		return a > b, nil
	case sqlp.GE:
		return a >= b, nil
	}
	return false, fmt.Errorf("unsupported operator: %s", op.String())
}

func compareStrings(op sqlp.Token, a, b string) (bool, error) {
	switch op {
	case sqlp.EQ:
		return a == b, nil
	case sqlp.NE:
		return a != b, nil
	case sqlp.LT:
		return a < b, nil
	case sqlp.LE:
		return a <= b, nil
	case sqlp.GT:
		return a > b, nil
	case sqlp.GE:
		return a >= b, nil
	}
	return false, fmt.Errorf("unsupported operator: %s", op.String())
}

func exprType(expr sqlp.Expr, sourceCols []ColumnDef) ColumnType {
	if expr == nil {
		return ColumnType_STRING
	}
	switch e := expr.(type) {
	case *sqlp.Ident:
		for _, col := range sourceCols {
			if strings.EqualFold(col.Name, e.Name) {
				return col.Type
			}
		}
	case *sqlp.QualifiedRef:
		schemaName := ""
		if e.Schema != nil {
			schemaName = e.Schema.Name
		}
		tableName := ""
		if e.Table != nil {
			tableName = e.Table.Name
		}
		columnName := ""
		if e.Column != nil {
			columnName = e.Column.Name
		}

		for _, col := range sourceCols {
			if schemaName != "" && strings.EqualFold(col.Name, tableName) && col.Type == ColumnType_COMPLEX {
				if strings.EqualFold(col.Qualifier, schemaName) {
					return ColumnType_STRING
				}
			}
			if schemaName == "" && strings.EqualFold(col.Name, tableName) && col.Type == ColumnType_COMPLEX {
				return ColumnType_STRING
			}
		}

		for _, col := range sourceCols {
			if strings.EqualFold(col.Name, columnName) {
				qual := tableName
				if schemaName != "" {
					qual = schemaName
				}
				if qual == "" || strings.EqualFold(col.Qualifier, qual) {
					return col.Type
				}
			}
		}
	case *sqlp.NumberLit:
		if strings.Contains(e.Value, ".") {
			return ColumnType_FLOAT
		}
		return ColumnType_INT
	case *sqlp.StringLit:
		return ColumnType_STRING
	case *sqlp.Call:
		name := strings.ToLower(e.Name.Name)
		if fn, exists := functions[name]; exists {
			var argTypes []ColumnType
			for _, argExpr := range e.Args {
				argTypes = append(argTypes, exprType(argExpr, sourceCols))
			}
			return fn.Type(argTypes)
		}
	case *sqlp.BinaryExpr:
		if e.Op == sqlp.PLUS || e.Op == sqlp.MINUS {
			tX := exprType(e.X, sourceCols)
			tY := exprType(e.Y, sourceCols)
			if tX == ColumnType_FLOAT || tY == ColumnType_FLOAT {
				return ColumnType_FLOAT
			}
			if tX == ColumnType_INT && tY == ColumnType_INT {
				return ColumnType_INT
			}
			return ColumnType_STRING
		}
		return ColumnType_INT // boolean results as INT

	case *sqlp.CastExpr:
		destType := strings.ToUpper(e.Type.Name.Name)
		switch destType {
		case "INT", "INTEGER":
			return ColumnType_INT
		case "FLOAT", "REAL", "DOUBLE":
			return ColumnType_FLOAT
		case "STRING", "VARCHAR", "TEXT":
			return ColumnType_STRING
		case "DATE", "DATETIME", "TIMESTAMP":
			return ColumnType_DATE
		case "COMPLEX":
			return ColumnType_COMPLEX
		case "ARRAY":
			return ColumnType_ARRAY
		}
		return ColumnType_STRING
	}
	return ColumnType_STRING
}

// NewGroupByNode creates a new Node that groups rows from a source node by key expressions.
func NewGroupByNode(source *Node, keys []sqlp.Expr) *Node {
	return &Node{
		impl: &GroupByNode{
			source: source,
			keys:   keys,
		},
	}
}

// GroupByNode aggregates rows from a source node based on grouping keys.
type GroupByNode struct {
	source *Node
	keys   []sqlp.Expr
}

func (g *GroupByNode) Types() []ColumnDef {
	return g.source.Types()
}

func (g *GroupByNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		sourceCols := g.source.Types()
		groups := make(map[string]Row)
		var groupOrder []string

		for row := range g.source.All() {
			var keyVals []any
			for _, keyExpr := range g.keys {
				val, err := evalExpr(keyExpr, row, sourceCols)
				if err != nil {
					val = nil
				}
				keyVals = append(keyVals, val)
			}

			k := serializeKey(keyVals)
			if groupRow, exists := groups[k]; !exists {
				groups[k] = Row{
					Value: row.Value,
					Group: []Row{row},
				}
				groupOrder = append(groupOrder, k)
			} else {
				groupRow.Group = append(groupRow.Group, row)
				groups[k] = groupRow
			}
		}

		for _, k := range groupOrder {
			if !yield(groups[k]) {
				return
			}
		}
	}
}

func serializeKey(vals []any) string {
	var sb strings.Builder
	for i, val := range vals {
		if i > 0 {
			sb.WriteByte(0)
		}
		sb.WriteString(fmt.Sprintf("%v", val))
	}
	return sb.String()
}

func arithmeticEval(op sqlp.Token, lhs, rhs any) (any, error) {
	switch lVal := lhs.(type) {
	case int:
		switch rVal := rhs.(type) {
		case int:
			if op == sqlp.PLUS {
				return lVal + rVal, nil
			}
			if op == sqlp.MINUS {
				return lVal - rVal, nil
			}
		case float64:
			if op == sqlp.PLUS {
				return float64(lVal) + rVal, nil
			}
			if op == sqlp.MINUS {
				return float64(lVal) - rVal, nil
			}
		}
	case float64:
		switch rVal := rhs.(type) {
		case int:
			if op == sqlp.PLUS {
				return lVal + float64(rVal), nil
			}
			if op == sqlp.MINUS {
				return lVal - float64(rVal), nil
			}
		case float64:
			if op == sqlp.PLUS {
				return lVal + rVal, nil
			}
			if op == sqlp.MINUS {
				return lVal - rVal, nil
			}
		}
	}
	return nil, fmt.Errorf("unsupported operands for arithmetic operation: %T and %T", lhs, rhs)
}

var dateLayouts = []string{
	"2006-01-02",
	"2006-01-02 15:04:05",
	time.RFC3339,
}

func parseDate(val string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, val); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", val)
}

func compareDates(op sqlp.Token, a, b time.Time) (bool, error) {
	switch op {
	case sqlp.EQ:
		return a.Equal(b), nil
	case sqlp.NE:
		return !a.Equal(b), nil
	case sqlp.LT:
		return a.Before(b), nil
	case sqlp.LE:
		return a.Before(b) || a.Equal(b), nil
	case sqlp.GT:
		return a.After(b), nil
	case sqlp.GE:
		return a.After(b) || a.Equal(b), nil
	}
	return false, fmt.Errorf("unsupported operator for date: %s", op.String())
}

// --- Dynamic Function Registration System ---

type Function interface {
	Type(argTypes []ColumnType) ColumnType
}

type ScalarFunction interface {
	Function
	Eval(args []any) (any, error)
}

type AggregateFunction interface {
	Function
	EvalGroup(args [][]any) (any, error)
}

var functions = make(map[string]Function)

func RegisterFunction(name string, fn Function) {
	functions[strings.ToLower(name)] = fn
}

func performCast(val any, targetType string) (any, error) {
	if val == nil {
		return nil, nil
	}

	switch targetType {
	case "INT", "INTEGER":
		switch v := val.(type) {
		case int:
			return v, nil
		case float64:
			return int(v), nil
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i, nil
			}
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return int(f), nil
			}
			return nil, fmt.Errorf("cannot cast string %q to INT", v)
		case time.Time:
			return int(v.Unix()), nil
		}

	case "FLOAT", "REAL", "DOUBLE":
		switch v := val.(type) {
		case int:
			return float64(v), nil
		case float64:
			return v, nil
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f, nil
			}
			return nil, fmt.Errorf("cannot cast string %q to FLOAT", v)
		case time.Time:
			return float64(v.Unix()), nil
		}

	case "STRING", "VARCHAR", "TEXT":
		switch v := val.(type) {
		case int:
			return strconv.Itoa(v), nil
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		case string:
			return v, nil
		case time.Time:
			return v.Format("2006-01-02 15:04:05"), nil
		}

	case "DATE", "DATETIME", "TIMESTAMP":
		switch v := val.(type) {
		case int:
			return time.Unix(int64(v), 0).UTC(), nil
		case float64:
			return time.Unix(int64(v), 0).UTC(), nil
		case string:
			if t, err := parseDate(v); err == nil {
				return t, nil
			}
			return nil, fmt.Errorf("cannot cast string %q to DATE", v)
		case time.Time:
			return v, nil
		}

	case "COMPLEX":
		if m, ok := val.(map[string]any); ok {
			return m, nil
		}
		if s, ok := val.(string); ok {
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err == nil {
				return m, nil
			}
		}
		return nil, fmt.Errorf("cannot cast type %T to COMPLEX", val)

	case "ARRAY":
		if arr, ok := val.([]any); ok {
			return arr, nil
		}
		if s, ok := val.(string); ok {
			var arr []any
			if err := json.Unmarshal([]byte(s), &arr); err == nil {
				return arr, nil
			}
		}
		// Convert any other slice/array types to []any using reflection
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			arr := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				arr[i] = rv.Index(i).Interface()
			}
			return arr, nil
		}
		return nil, fmt.Errorf("cannot cast type %T to ARRAY", val)
	}

	return nil, fmt.Errorf("unsupported cast to type %q", targetType)
}

func getComplexValue(val any, key string) (any, error) {
	if val == nil {
		return nil, nil
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("value is not of type COMPLEX (map)")
	}
	return m[key], nil
}

// ParseDate is exported for use by the sources package.
func ParseDate(val string) (time.Time, error) {
	return parseDate(val)
}

// NewOrderNode creates a new Node that orders rows from a source node by ordering terms.
func NewOrderNode(source *Node, terms []*sqlp.OrderingTerm, selectCols []*sqlp.ResultColumn) *Node {
	return &Node{
		impl: &OrderNode{
			source:     source,
			terms:      terms,
			selectCols: selectCols,
		},
	}
}

type OrderNode struct {
	source     *Node
	terms      []*sqlp.OrderingTerm
	selectCols []*sqlp.ResultColumn
}

func (o *OrderNode) Types() []ColumnDef {
	return o.source.Types()
}

type sortRow struct {
	row      Row
	sortKeys []any
}

func (o *OrderNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		var rows []Row
		for row := range o.source.All() {
			rows = append(rows, row)
		}

		sourceCols := o.source.Types()
		resolvedExprs := make([]sqlp.Expr, len(o.terms))
		for i, term := range o.terms {
			resolvedExprs[i] = resolveOrderingExpr(term.X, o.selectCols, sourceCols)
		}

		sortRows := make([]sortRow, len(rows))
		for i, r := range rows {
			keys := make([]any, len(o.terms))
			for j, expr := range resolvedExprs {
				val, err := evalExpr(expr, r, sourceCols)
				if err != nil {
					val = nil
				}
				keys[j] = val
			}
			sortRows[i] = sortRow{row: r, sortKeys: keys}
		}

		slices.SortStableFunc(sortRows, func(a, b sortRow) int {
			for i, term := range o.terms {
				valA := a.sortKeys[i]
				valB := b.sortKeys[i]
				desc := term.Desc.IsValid()

				if valA == nil && valB == nil {
					continue
				}
				if valA == nil || valB == nil {
					nullsFirst := !desc
					if term.NullsFirst.IsValid() {
						nullsFirst = true
					} else if term.NullsLast.IsValid() {
						nullsFirst = false
					}
					if valA == nil {
						if nullsFirst {
							return -1
						}
						return 1
					} else {
						if nullsFirst {
							return 1
						}
						return -1
					}
				}

				cmp := compareOrderValues(valA, valB)
				if cmp != 0 {
					if desc {
						return -cmp
					}
					return cmp
				}
			}
			return 0
		})

		for _, sr := range sortRows {
			if !yield(sr.row) {
				return
			}
		}
	}
}

func resolveOrderingExpr(expr sqlp.Expr, selectCols []*sqlp.ResultColumn, sourceCols []ColumnDef) sqlp.Expr {
	if expr == nil {
		return nil
	}

	if numLit, ok := expr.(*sqlp.NumberLit); ok {
		if val, err := strconv.Atoi(numLit.Value); err == nil {
			colIdx := 0
			for _, col := range selectCols {
				if col.Star.IsValid() {
					for range sourceCols {
						colIdx++
						if colIdx == val {
							srcCol := sourceCols[colIdx-1]
							if srcCol.Qualifier != "" {
								return &sqlp.QualifiedRef{
									Table:  &sqlp.Ident{Name: srcCol.Qualifier},
									Column: &sqlp.Ident{Name: srcCol.Name},
								}
							}
							return &sqlp.Ident{Name: srcCol.Name}
						}
					}
				} else if qref, ok := col.Expr.(*sqlp.QualifiedRef); ok && qref.Star.IsValid() {
					tableName := qref.Table.Name
					for _, sourceCol := range sourceCols {
						if strings.EqualFold(sourceCol.Qualifier, tableName) {
							colIdx++
							if colIdx == val {
								return &sqlp.QualifiedRef{
									Table:  &sqlp.Ident{Name: sourceCol.Qualifier},
									Column: &sqlp.Ident{Name: sourceCol.Name},
								}
							}
						}
					}
				} else {
					colIdx++
					if colIdx == val {
						return col.Expr
					}
				}
			}
		}
	}

	return resolveAliases(expr, selectCols)
}

func resolveAliases(expr sqlp.Expr, selectCols []*sqlp.ResultColumn) sqlp.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *sqlp.Ident:
		for _, col := range selectCols {
			if col.Alias != nil && strings.EqualFold(col.Alias.Name, e.Name) {
				return col.Expr
			}
		}
		return e
	case *sqlp.BinaryExpr:
		return &sqlp.BinaryExpr{
			Op: e.Op,
			X:  resolveAliases(e.X, selectCols),
			Y:  resolveAliases(e.Y, selectCols),
		}
	case *sqlp.Call:
		newArgs := make([]sqlp.Expr, len(e.Args))
		for i, arg := range e.Args {
			newArgs[i] = resolveAliases(arg, selectCols)
		}
		return &sqlp.Call{
			Name: e.Name,
			Args: newArgs,
		}
	case *sqlp.CastExpr:
		return &sqlp.CastExpr{
			X:    resolveAliases(e.X, selectCols),
			Type: e.Type,
		}
	default:
		return e
	}
}

func compareOrderValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch aVal := a.(type) {
	case int:
		switch bVal := b.(type) {
		case int:
			if aVal < bVal {
				return -1
			}
			if aVal > bVal {
				return 1
			}
			return 0
		case float64:
			return compareFloats3Way(float64(aVal), bVal)
		}
	case float64:
		switch bVal := b.(type) {
		case int:
			return compareFloats3Way(aVal, float64(bVal))
		case float64:
			return compareFloats3Way(aVal, bVal)
		}
	case string:
		switch bVal := b.(type) {
		case string:
			if aVal < bVal {
				return -1
			}
			if aVal > bVal {
				return 1
			}
			return 0
		case time.Time:
			if aDate, err := parseDate(aVal); err == nil {
				return compareDates3Way(aDate, bVal)
			}
		}
	case time.Time:
		switch bVal := b.(type) {
		case time.Time:
			return compareDates3Way(aVal, bVal)
		case string:
			if bDate, err := parseDate(bVal); err == nil {
				return compareDates3Way(aVal, bDate)
			}
		}
	case bool:
		if bVal, ok := b.(bool); ok {
			if !aVal && bVal {
				return -1
			}
			if aVal && !bVal {
				return 1
			}
			return 0
		}
	}

	strA := fmt.Sprintf("%v", a)
	strB := fmt.Sprintf("%v", b)
	if strA < strB {
		return -1
	}
	if strA > strB {
		return 1
	}
	return 0
}

func compareFloats3Way(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareDates3Way(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}
