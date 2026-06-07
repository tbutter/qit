package nodes

import (
	"iter"
	"testing"

	sqlp "github.com/rqlite/sql"
)

type simpleMockNode struct {
	types []ColumnDef
	rows  []Row
}

func (s *simpleMockNode) Types() []ColumnDef {
	return s.types
}

func (s *simpleMockNode) All() iter.Seq[Row] {
	return func(yield func(Row) bool) {
		for _, r := range s.rows {
			if !yield(r) {
				return
			}
		}
	}
}

func TestAliasNode(t *testing.T) {
	mock := &simpleMockNode{
		types: []ColumnDef{
			{Name: "id", Type: ColumnType_INT, Qualifier: "orig"},
			{Name: "val", Type: ColumnType_STRING, Qualifier: "orig"},
		},
		rows: []Row{
			{Value: []any{1, "apple"}},
			{Value: []any{2, "banana"}},
		},
	}

	node := NewAliasNode(NewNode(mock), "my_alias")

	// Test Types()
	types := node.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types))
	}
	if types[0].Qualifier != "my_alias" || types[1].Qualifier != "my_alias" {
		t.Errorf("expected Qualifier to be overridden to 'my_alias', got: %+v", types)
	}

	// Test All()
	var rows []Row
	for row := range node.All() {
		rows = append(rows, row)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Value[0].(int) != 1 || rows[0].Value[1].(string) != "apple" {
		t.Errorf("row values got corrupted in AliasNode: %+v", rows[0])
	}
}

func TestJoinNode(t *testing.T) {
	leftMock := &simpleMockNode{
		types: []ColumnDef{
			{Name: "id", Type: ColumnType_INT, Qualifier: "L"},
			{Name: "x", Type: ColumnType_STRING, Qualifier: "L"},
		},
		rows: []Row{
			{Value: []any{1, "L1"}},
			{Value: []any{2, "L2"}},
			{Value: []any{3, "L3"}},
		},
	}

	rightMock := &simpleMockNode{
		types: []ColumnDef{
			{Name: "id", Type: ColumnType_INT, Qualifier: "R"},
			{Name: "y", Type: ColumnType_STRING, Qualifier: "R"},
		},
		rows: []Row{
			{Value: []any{1, "R1"}},
			{Value: []any{3, "R3"}},
			{Value: []any{4, "R4"}},
		},
	}

	leftNode := NewNode(leftMock)
	rightNode := NewNode(rightMock)

	// 1. Types() check
	joinNode := NewJoinNode(leftNode, rightNode, "JOIN", nil)
	types := joinNode.Types()
	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}
	if types[0].Name != "id" || types[0].Qualifier != "L" || types[2].Name != "id" || types[2].Qualifier != "R" {
		t.Errorf("unexpected combined columns: %+v", types)
	}

	// 2. Cartesian product (constraint = nil)
	var rowsCartesian []Row
	for r := range joinNode.All() {
		rowsCartesian = append(rowsCartesian, r)
	}
	if len(rowsCartesian) != 9 {
		t.Errorf("expected 9 rows for Cartesian product, got %d", len(rowsCartesian))
	}

	// 3. INNER JOIN with constraint (L.id = R.id)
	constraint := &sqlp.BinaryExpr{
		Op: sqlp.EQ,
		X: &sqlp.QualifiedRef{
			Table:  &sqlp.Ident{Name: "L"},
			Column: &sqlp.Ident{Name: "id"},
		},
		Y: &sqlp.QualifiedRef{
			Table:  &sqlp.Ident{Name: "R"},
			Column: &sqlp.Ident{Name: "id"},
		},
	}

	innerJoinNode := NewJoinNode(leftNode, rightNode, "JOIN", constraint)
	var rowsInner []Row
	for r := range innerJoinNode.All() {
		rowsInner = append(rowsInner, r)
	}
	// Expected matches: (1, "L1", 1, "R1") and (3, "L3", 3, "R3") -> 2 rows
	if len(rowsInner) != 2 {
		t.Fatalf("expected 2 rows for Inner Join, got %d", len(rowsInner))
	}
	if rowsInner[0].Value[0].(int) != 1 || rowsInner[0].Value[2].(int) != 1 {
		t.Errorf("unexpected inner join row 0: %+v", rowsInner[0])
	}
	if rowsInner[1].Value[0].(int) != 3 || rowsInner[1].Value[2].(int) != 3 {
		t.Errorf("unexpected inner join row 1: %+v", rowsInner[1])
	}

	// 4. LEFT JOIN with same constraint
	leftOuterJoinNode := NewJoinNode(leftNode, rightNode, "LEFT", constraint)
	var rowsLeft []Row
	for r := range leftOuterJoinNode.All() {
		rowsLeft = append(rowsLeft, r)
	}
	// Expected matches:
	// - (1, "L1", 1, "R1")
	// - (2, "L2", nil, nil)
	// - (3, "L3", 3, "R3")
	if len(rowsLeft) != 3 {
		t.Fatalf("expected 3 rows for Left Join, got %d", len(rowsLeft))
	}
	// Verify row 1 (the mismatch row): right values must be nil
	if rowsLeft[1].Value[0].(int) != 2 || rowsLeft[1].Value[2] != nil || rowsLeft[1].Value[3] != nil {
		t.Errorf("unexpected left join row 1: %+v", rowsLeft[1])
	}
}
