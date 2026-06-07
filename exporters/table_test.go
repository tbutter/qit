package exporters_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tbutter/qit/exporters"
	"github.com/tbutter/qit/nodes"
)

func TestExportTable(t *testing.T) {
	nodeTypes := []nodes.ColumnDef{
		{Name: "id", Type: nodes.ColumnType_INT},
		{Name: "name", Type: nodes.ColumnType_STRING},
		{Name: "price", Type: nodes.ColumnType_FLOAT},
		{Name: "date", Type: nodes.ColumnType_DATE},
	}

	dateVal := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	nodeRows := []nodes.Row{
		{
			Value: []any{
				1,
				"Widget",
				19.99,
				dateVal,
			},
		},
		{
			Value: []any{
				100,
				"Gizmo",
				5.5,
				nil,
			},
		},
	}

	memNode := &memoryNode{
		types: nodeTypes,
		rows:  nodeRows,
	}

	node := nodes.NewNode(memNode)

	var buf bytes.Buffer
	err := exporters.ExportTable(&buf, node)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	expectedTable := `+-----+--------+-------+------------+
|  id | name   | price | date       |
+-----+--------+-------+------------+
|   1 | Widget | 19.99 | 2026-06-01 |
| 100 | Gizmo  |   5.5 | NULL       |
+-----+--------+-------+------------+
`

	actualTable := buf.String()
	actualNormalized := strings.ReplaceAll(actualTable, "\r\n", "\n")
	expectedNormalized := strings.ReplaceAll(expectedTable, "\r\n", "\n")

	if actualNormalized != expectedNormalized {
		t.Errorf("unexpected table output:\nExpected:\n%s\nActual:\n%s", expectedNormalized, actualNormalized)
	}
}
