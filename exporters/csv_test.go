package exporters_test

import (
	"bytes"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/tbutter/qit/exporters"
	"github.com/tbutter/qit/nodes"
)

type memoryNode struct {
	types []nodes.ColumnDef
	rows  []nodes.Row
}

func (m *memoryNode) Types() []nodes.ColumnDef {
	return m.types
}

func (m *memoryNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		for _, row := range m.rows {
			if !yield(row) {
				return
			}
		}
	}
}

func TestExportCSV(t *testing.T) {
	nodeTypes := []nodes.ColumnDef{
		{Name: "id", Type: nodes.ColumnType_INT},
		{Name: "name", Type: nodes.ColumnType_STRING},
		{Name: "price", Type: nodes.ColumnType_FLOAT},
		{Name: "date_only", Type: nodes.ColumnType_DATE},
		{Name: "date_time", Type: nodes.ColumnType_DATE},
		{Name: "meta", Type: nodes.ColumnType_COMPLEX},
		{Name: "empty", Type: nodes.ColumnType_STRING},
	}

	date1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 6, 2, 15, 30, 45, 0, time.UTC)

	complexVal := map[string]any{"tags": []any{"a", "b"}, "valid": true}

	nodeRows := []nodes.Row{
		{
			Value: []any{
				1,
				"Widget",
				19.99,
				date1,
				date2,
				complexVal,
				nil,
			},
		},
		{
			Value: []any{
				2,
				"Gizmo",
				5.5,
				date1,
				date1, // Even if it's the date_time column, if time portion is zero, it formats as date-only
				nil,
				"not empty",
			},
		},
	}

	memNode := &memoryNode{
		types: nodeTypes,
		rows:  nodeRows,
	}

	node := nodes.NewNode(memNode)

	var buf bytes.Buffer
	err := exporters.ExportCSV(&buf, node)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	expectedCSV := `id,name,price,date_only,date_time,meta,empty
1,Widget,19.99,2026-06-01,2026-06-02 15:30:45,"{""tags"":[""a"",""b""],""valid"":true}",
2,Gizmo,5.5,2026-06-01,2026-06-01,,not empty
`

	actualCSV := buf.String()
	// Normalize line endings to avoid platform conflicts in tests
	actualCSVNormalized := strings.ReplaceAll(actualCSV, "\r\n", "\n")
	expectedCSVNormalized := strings.ReplaceAll(expectedCSV, "\r\n", "\n")

	if actualCSVNormalized != expectedCSVNormalized {
		t.Errorf("unexpected CSV output:\nExpected:\n%s\nActual:\n%s", expectedCSVNormalized, actualCSVNormalized)
	}
}
