package sources

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/tbutter/qit/nodes"
	sqlp "github.com/rqlite/sql"
)

func TestCSVNode(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `id,rate,joined_on,name
1,1.5,2026-06-01,Alice
2,2.7,2026-06-02,Bob
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV: %v", err)
	}

	// 1. Success case
	node, err := NewCSVNode(csvPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	types := node.Types()
	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}

	// Column type checks
	expectedTypes := map[string]nodes.ColumnType{
		"id":        nodes.ColumnType_INT,
		"rate":      nodes.ColumnType_FLOAT,
		"joined_on": nodes.ColumnType_DATE,
		"name":      nodes.ColumnType_STRING,
	}
	for _, col := range types {
		expType, found := expectedTypes[col.Name]
		if !found {
			t.Errorf("unexpected column name: %s", col.Name)
		} else if col.Type != expType {
			t.Errorf("expected column %s to be of type %v, got %v", col.Name, expType, col.Type)
		}
	}

	// Read rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0].(int) != 1 || rows[0][1].(float64) != 1.5 || rows[0][3].(string) != "Alice" {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}

	// 2. Error case: non-existent file
	_, err = NewCSVNode(filepath.Join(tempDir, "does_not_exist.csv"))
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestJSONLNode(t *testing.T) {
	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "test.jsonl")

	jsonlData := `{"id": 1, "is_valid": true, "rate": 1.5, "joined_on": "2026-06-01", "meta": {"k": "v"}, "name": "Alice", "tags": ["a", "b"]}
{"id": 2, "is_valid": false, "rate": 2.7, "joined_on": "2026-06-02", "meta": {"k": "v2"}, "name": "Bob", "tags": ["c"]}
`
	if err := os.WriteFile(jsonlPath, []byte(jsonlData), 0644); err != nil {
		t.Fatalf("failed to write temp JSONL: %v", err)
	}

	// 1. Success case
	node, err := NewJSONLNode(jsonlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	types := node.Types()
	if len(types) != 7 {
		t.Fatalf("expected 7 columns, got %d", len(types))
	}

	// Keys should be sorted alphabetically: id, is_valid, joined_on, meta, name, rate, tags
	expectedNames := []string{"id", "is_valid", "joined_on", "meta", "name", "rate", "tags"}
	for i, col := range types {
		if col.Name != expectedNames[i] {
			t.Errorf("expected column %d to be %s, got %s", i, expectedNames[i], col.Name)
		}
	}
	if types[6].Type != nodes.ColumnType_ARRAY {
		t.Errorf("expected column tags to be ColumnType_ARRAY, got %v", types[6].Type)
	}

	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Row 0: id (int), is_valid (int), joined_on (date), meta (complex), name (string), rate (float), tags (array)
	if rows[0][0].(int) != 1 || rows[0][1].(int) != 1 || rows[0][5].(float64) != 1.5 {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	tags, ok := rows[0][6].([]any)
	if !ok || len(tags) != 2 || tags[0].(string) != "a" || tags[1].(string) != "b" {
		t.Errorf("unexpected tags in row 0: %+v", rows[0][6])
	}

	// 2. Error case: empty file
	emptyPath := filepath.Join(tempDir, "empty.jsonl")
	if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}
	_, err = NewJSONLNode(emptyPath)
	if err == nil {
		t.Error("expected error for empty file, got nil")
	}

	// 3. Error case: invalid JSON
	invalidPath := filepath.Join(tempDir, "invalid.jsonl")
	if err := os.WriteFile(invalidPath, []byte("{invalidjson\n"), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}
	_, err = NewJSONLNode(invalidPath)
	if err == nil {
		t.Error("expected error for invalid JSON file, got nil")
	}
}

func TestSQLiteNode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		rate REAL,
		joined_on DATE,
		meta TEXT,
		name TEXT
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`INSERT INTO users (id, rate, joined_on, meta, name) VALUES
		(1, 1.5, '2026-06-01', '{"k":"v"}', 'Alice')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// 1. Success case
	node, err := NewSQLiteNode(dbPath, "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	types := node.Types()
	if len(types) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(types))
	}

	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][0].(int) != 1 || rows[0][1].(float64) != 1.5 || rows[0][4].(string) != "Alice" {
		t.Errorf("unexpected SQLite row: %+v", rows[0])
	}

	// 2. Error case: non-existent table
	_, err = NewSQLiteNode(dbPath, "non_existent_table")
	if err == nil {
		t.Error("expected error for non-existent table, got nil")
	}
}

func TestFakeNode(t *testing.T) {
	node := NewFakeNode()
	types := node.Types()
	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}

	var count int
	for row := range node.All() {
		count++
		if len(row.Value) != 4 {
			t.Errorf("expected row to have 4 values, got %d", len(row.Value))
		}
	}
	if count != 100 {
		t.Errorf("expected 100 rows from FakeNode, got %d", count)
	}
}

func TestCreateSource(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "source_test.csv")
	if err := os.WriteFile(csvPath, []byte("id,name\n1,Alice\n"), 0644); err != nil {
		t.Fatalf("failed to write CSV: %v", err)
	}

	jsonlPath := filepath.Join(tempDir, "source_test.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"id": 1, "name": "Alice"}`), 0644); err != nil {
		t.Fatalf("failed to write JSONL: %v", err)
	}

	// test csv creation
	node, name, err := createSource("csv", []sqlp.Expr{&sqlp.StringLit{Value: csvPath}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "source_test" {
		t.Errorf("expected name 'source_test', got %s", name)
	}
	if len(node.Types()) != 2 {
		t.Errorf("unexpected column count: %d", len(node.Types()))
	}

	// test jsonl creation
	node, name, err = createSource("jsonl", []sqlp.Expr{&sqlp.StringLit{Value: jsonlPath}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "source_test" {
		t.Errorf("expected name 'source_test', got %s", name)
	}

	// test fallback to fake node
	node, name, err = createSource("unknown_table_function", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "unknown_table_function" {
		t.Errorf("expected name 'unknown_table_function', got %s", name)
	}
	if len(node.Types()) != 4 { // FakeNode has 4 columns
		t.Errorf("expected FakeNode fallback, got column count: %d", len(node.Types()))
	}
}
