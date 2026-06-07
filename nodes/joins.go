package nodes

import (
	"iter"

	sqlp "github.com/rqlite/sql"
)

// NewAliasNode creates an alias node wrapping a source node and applying a table qualifier.
func NewAliasNode(source *Node, alias string) *Node {
	return &Node{
		impl: &AliasNode{
			source: source,
			alias:  alias,
		},
	}
}

// AliasNode overrides the Qualifier of source columns.
type AliasNode struct {
	source *Node
	alias  string
}

func (a *AliasNode) Types() []ColumnDef {
	cols := a.source.Types()
	res := make([]ColumnDef, len(cols))
	for i, col := range cols {
		res[i] = col
		res[i].Qualifier = a.alias
	}
	return res
}

func (a *AliasNode) All() iter.Seq[Row] {
	return a.source.All()
}

// NewJoinNode creates a join node wrapping two sources.
func NewJoinNode(left, right *Node, joinType string, constraint sqlp.Expr) *Node {
	return &Node{
		impl: &JoinNode{
			left:       left,
			right:      right,
			joinType:   joinType,
			constraint: constraint,
		},
	}
}

// JoinNode joins left and right nodes.
type JoinNode struct {
	left       *Node
	right      *Node
	joinType   string
	constraint sqlp.Expr
}

func (j *JoinNode) Types() []ColumnDef {
	return append(j.left.Types(), j.right.Types()...)
}

func (j *JoinNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		leftCols := j.left.Types()
		rightCols := j.right.Types()
		combinedCols := append(leftCols, rightCols...)

		for leftRow := range j.left.All() {
			matchedAny := false
			for rightRow := range j.right.All() {
				// Combined values
				combinedVals := make([]any, 0, len(leftRow.Value)+len(rightRow.Value))
				combinedVals = append(combinedVals, leftRow.Value...)
				combinedVals = append(combinedVals, rightRow.Value...)
				combinedRow := Row{
					Value: combinedVals,
				}

				matched := true
				if j.constraint != nil {
					var err error
					matched, err = evalFilter(j.constraint, combinedRow, combinedCols)
					if err != nil {
						matched = false
					}
				}

				if matched {
					matchedAny = true
					if !yield(combinedRow) {
						return
					}
				}
			}

			// Left Join support
			if !matchedAny && (j.joinType == "LEFT" || j.joinType == "LEFT OUTER") {
				nullRight := make([]any, len(rightCols))
				combinedVals := make([]any, 0, len(leftRow.Value)+len(nullRight))
				combinedVals = append(combinedVals, leftRow.Value...)
				combinedVals = append(combinedVals, nullRight...)
				combinedRow := Row{
					Value: combinedVals,
				}
				if !yield(combinedRow) {
					return
				}
			}
		}
	}
}
