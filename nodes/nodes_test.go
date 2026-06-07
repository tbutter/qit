package nodes_test

import (
	dbsql "database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tbutter/qit/nodes"
	"github.com/tbutter/qit/sources"
	"github.com/tbutter/qit/sql"
	_ "modernc.org/sqlite"
)

func TestFakeNode(t *testing.T) {
	node := sources.NewFakeNode()
	types := node.Types()

	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}
	if types[0].Name != "a" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column at 0: %+v", types[0])
	}
	if types[1].Name != "b" || types[1].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column at 1: %+v", types[1])
	}
	if types[2].Name != "c" || types[2].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column at 2: %+v", types[2])
	}
	if types[3].Name != "d" || types[3].Type != nodes.ColumnType_COMPLEX {
		t.Errorf("unexpected column at 3: %+v", types[3])
	}

	count := 0
	for row := range node.All() {
		count++
		vals := row.Value
		if len(vals) != 4 {
			t.Fatalf("expected row to have 4 fields, got %d", len(vals))
		}
		if a, ok := vals[0].(int); !ok || a < 1 || a > 10 {
			t.Errorf("unexpected value for column a: %v", vals[0])
		}
		if b, ok := vals[1].(int); !ok || b < 100 || b > 999 {
			t.Errorf("unexpected value for column b: %v", vals[1])
		}
		if _, ok := vals[2].(string); !ok {
			t.Errorf("unexpected value for column c: %v", vals[2])
		}
		if m, ok := vals[3].(map[string]any); !ok || m["k1"] != "nested_value" {
			t.Errorf("unexpected value for column d: %v", vals[3])
		}
	}

	if count != 100 {
		t.Errorf("expected 100 rows, got %d", count)
	}
}

func TestQueryExecution(t *testing.T) {
	stmt := sql.Parse("SELECT a, c FROM x WHERE a = 5")
	node := nodes.NodeFromStatement(stmt)
	types := node.Types()

	if len(types) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types))
	}
	if types[0].Name != "a" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected projected column 0: %+v", types[0])
	}
	if types[1].Name != "c" || types[1].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected projected column 1: %+v", types[1])
	}

	count := 0
	for row := range node.All() {
		count++
		vals := row.Value
		if len(vals) != 2 {
			t.Fatalf("expected projected row to have 2 fields, got %d", len(vals))
		}
		if vals[0].(int) != 5 {
			t.Errorf("expected filtered column a to be 5, got %v", vals[0])
		}
		if _, ok := vals[1].(string); !ok {
			t.Errorf("expected projected column c to be string, got %T", vals[1])
		}
	}

	t.Logf("found %d matching rows for query", count)
}

func TestCSVNode(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
3,20,banana
5,30,cherry
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query selecting from the CSV file
	stmt := sql.Parse("SELECT a, c FROM csv(\"" + csvPath + "\") WHERE a = 5")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types))
	}
	if types[0].Name != "a" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column 0: %+v", types[0])
	}
	if types[1].Name != "c" || types[1].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 1: %+v", types[1])
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Row 1
	if rows[0][0].(int) != 5 || rows[0][1].(string) != "apple" {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	// Row 2
	if rows[1][0].(int) != 5 || rows[1][1].(string) != "cherry" {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
}

func TestJSONLNode(t *testing.T) {
	// Create a temporary JSONL file
	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "test.jsonl")

	jsonlData := `{"a": 5, "b": 10.5, "c": "apple", "d": "2026-06-05", "e": {"nested": "value"}}
{"a": 3, "b": 20.0, "c": "banana", "d": "2026-06-06", "e": {"nested": "other"}}
{"a": 5, "b": 30.5, "c": "cherry", "d": "2026-06-07"}
`
	if err := os.WriteFile(jsonlPath, []byte(jsonlData), 0644); err != nil {
		t.Fatalf("failed to write temp JSONL file: %v", err)
	}

	// Parse query selecting from the JSONL file
	stmt := sql.Parse("SELECT a, b, c, d, e FROM jsonl(\"" + jsonlPath + "\") WHERE a = 5")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(types))
	}
	if types[0].Name != "a" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column 0: %+v", types[0])
	}
	if types[1].Name != "b" || types[1].Type != nodes.ColumnType_FLOAT {
		t.Errorf("unexpected column 1: %+v", types[1])
	}
	if types[2].Name != "c" || types[2].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 2: %+v", types[2])
	}
	if types[3].Name != "d" || types[3].Type != nodes.ColumnType_DATE {
		t.Errorf("unexpected column 3: %+v", types[3])
	}
	if types[4].Name != "e" || types[4].Type != nodes.ColumnType_COMPLEX {
		t.Errorf("unexpected column 4: %+v", types[4])
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Row 1
	if rows[0][0].(int) != 5 || rows[0][1].(float64) != 10.5 || rows[0][2].(string) != "apple" {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	t1, _ := nodes.ParseDate("2026-06-05")
	if !rows[0][3].(time.Time).Equal(t1) {
		t.Errorf("unexpected date in row 0: %+v", rows[0][3])
	}
	m1 := rows[0][4].(map[string]any)
	if m1["nested"] != "value" {
		t.Errorf("unexpected complex value in row 0: %+v", rows[0][4])
	}

	// Row 2 (the third row in JSONL, where 'e' is missing)
	if rows[1][0].(int) != 5 || rows[1][1].(float64) != 30.5 || rows[1][2].(string) != "cherry" {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
	t2, _ := nodes.ParseDate("2026-06-07")
	if !rows[1][3].(time.Time).Equal(t2) {
		t.Errorf("unexpected date in row 1: %+v", rows[1][3])
	}
	if rows[1][4] != nil {
		t.Errorf("expected missing key 'e' to be nil, got: %+v", rows[1][4])
	}
}

func TestGroupByNode(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
5,20,banana
3,30,cherry
3,40,date
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query with GROUP BY
	stmt := sql.Parse("SELECT a, b, c FROM csv(\"" + csvPath + "\") GROUP BY a")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(types))
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 aggregated rows, got %d", len(rows))
	}

	// Row 1 (key = 5) should use the first b value (10) and first c value ("apple")
	if rows[0][0].(int) != 5 || rows[0][1].(int) != 10 || rows[0][2].(string) != "apple" {
		t.Errorf("unexpected aggregated row 0: %+v", rows[0])
	}
	// Row 2 (key = 3) should use the first b value (30) and first c value ("cherry")
	if rows[1][0].(int) != 3 || rows[1][1].(int) != 30 || rows[1][2].(string) != "cherry" {
		t.Errorf("unexpected aggregated row 1: %+v", rows[1])
	}
}

func TestAggregatesWithGroupBy(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
5,20,banana
3,30,cherry
3,40,date
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query with GROUP BY and aggregates
	stmt := sql.Parse("SELECT a, min(b), max(b), count(b), sum(b), count(*) FROM csv(\"" + csvPath + "\") GROUP BY a")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(types))
	}
	if types[1].Type != nodes.ColumnType_INT || types[2].Type != nodes.ColumnType_INT || types[3].Type != nodes.ColumnType_INT || types[4].Type != nodes.ColumnType_INT || types[5].Type != nodes.ColumnType_INT {
		t.Errorf("expected all aggregate types to be INT, got: %+v", types)
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 aggregated rows, got %d", len(rows))
	}

	// Row 1 (key = 5): a=5, min(b)=10, max(b)=20, count(b)=2, sum(b)=30, count(*)=2
	if rows[0][0].(int) != 5 || rows[0][1].(int) != 10 || rows[0][2].(int) != 20 || rows[0][3].(int) != 2 || rows[0][4].(int) != 30 || rows[0][5].(int) != 2 {
		t.Errorf("unexpected aggregated row 0: %+v", rows[0])
	}
	// Row 2 (key = 3): a=3, min(b)=30, max(b)=40, count(b)=2, sum(b)=70, count(*)=2
	if rows[1][0].(int) != 3 || rows[1][1].(int) != 30 || rows[1][2].(int) != 40 || rows[1][3].(int) != 2 || rows[1][4].(int) != 70 || rows[1][5].(int) != 2 {
		t.Errorf("unexpected aggregated row 1: %+v", rows[1])
	}
}

func TestAggregatesWithoutGroupBy(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
5,20,banana
3,30,cherry
3,40,date
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query with aggregates but no GROUP BY
	stmt := sql.Parse("SELECT min(b), max(b), count(b), sum(b), count(*) FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(types))
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 aggregated row, got %d", len(rows))
	}

	// expected: min(b)=10, max(b)=40, count(b)=4, sum(b)=100, count(*)=4
	if rows[0][0].(int) != 10 || rows[0][1].(int) != 40 || rows[0][2].(int) != 4 || rows[0][3].(int) != 100 || rows[0][4].(int) != 4 {
		t.Errorf("unexpected aggregated row: %+v", rows[0])
	}
}

func TestArithmeticSelect(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
3,20,banana
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query with arithmetic operations: SELECT a-b, a+b, a-b+10
	stmt := sql.Parse("SELECT a-b, a+b, a-b+10 FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(types))
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Row 1: a=5, b=10. a-b = -5, a+b = 15, a-b+10 = 5
	if rows[0][0].(int) != -5 || rows[0][1].(int) != 15 || rows[0][2].(int) != 5 {
		t.Errorf("unexpected arithmetic row 0: %+v", rows[0])
	}
	// Row 2: a=3, b=20. a-b = -17, a+b = 23, a-b+10 = -7
	if rows[1][0].(int) != -17 || rows[1][1].(int) != 23 || rows[1][2].(int) != -7 {
		t.Errorf("unexpected arithmetic row 1: %+v", rows[1])
	}
}

func TestConcatFunction(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
3,20,banana
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query using concat: SELECT concat(c, '-', a)
	stmt := sql.Parse("SELECT concat(c, '-', a) FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0][0].(string) != "apple-5" {
		t.Errorf("expected apple-5, got %v", rows[0][0])
	}
	if rows[1][0].(string) != "banana-3" {
		t.Errorf("expected banana-3, got %v", rows[1][0])
	}
}

// Custom function to reverse a string
type reverseFunc struct{}

func (r reverseFunc) Type(argTypes []nodes.ColumnType) nodes.ColumnType {
	return nodes.ColumnType_STRING
}

func (r reverseFunc) Eval(args []any) (any, error) {
	if len(args) == 0 || args[0] == nil {
		return "", nil
	}
	s := fmt.Sprintf("%v", args[0])
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes), nil
}

func TestCustomRegisteredFunction(t *testing.T) {
	// Register custom reverse function
	nodes.RegisterFunction("reverse", reverseFunc{})

	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10,apple
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query using reverse: SELECT reverse(c)
	stmt := sql.Parse("SELECT reverse(c) FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if rows[0][0].(string) != "elppa" {
		t.Errorf("expected elppa, got %v", rows[0][0])
	}
}

func TestFloatSupport(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5.5,10,apple
3.2,20,banana
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query: SELECT a, a+b, a-1.5 FROM csv(...)
	stmt := sql.Parse("SELECT a, a+b, a-1.5 FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(types))
	}
	if types[0].Type != nodes.ColumnType_FLOAT || types[1].Type != nodes.ColumnType_FLOAT || types[2].Type != nodes.ColumnType_FLOAT {
		t.Errorf("expected columns to be FLOAT, got: %+v", types)
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Row 1: a=5.5, b=10. a+b = 15.5, a-1.5 = 4.0
	if rows[0][0].(float64) != 5.5 || rows[0][1].(float64) != 15.5 || rows[0][2].(float64) != 4.0 {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	// Row 2: a=3.2, b=20. a+b = 23.2, a-1.5 = 1.7
	if rows[1][0].(float64) != 3.2 || rows[1][1].(float64) != 23.2 || math.Abs(rows[1][2].(float64)-1.7) > 1e-9 {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
}

func TestDateSupport(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b
2026-06-01,apple
2026-06-06,banana
2026-06-10,cherry
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query: SELECT a, b FROM csv(...) WHERE a > '2026-06-05'
	stmt := sql.Parse("SELECT a, b FROM csv(\"" + csvPath + "\") WHERE a > '2026-06-05'")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types))
	}
	if types[0].Type != nodes.ColumnType_DATE || types[1].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column types: %+v", types)
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	// Should match: 2026-06-06 and 2026-06-10 (2 rows)
	if len(rows) != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", len(rows))
	}

	d1, _ := time.Parse("2006-01-02", "2026-06-06")
	d2, _ := time.Parse("2006-01-02", "2026-06-10")

	if !rows[0][0].(time.Time).Equal(d1) || rows[0][1].(string) != "banana" {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	if !rows[1][0].(time.Time).Equal(d2) || rows[1][1].(string) != "cherry" {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
}

func TestCastSupport(t *testing.T) {
	// Create a temporary CSV file
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	csvData := `a,b,c
5,10.5,2026-06-06
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write temp CSV file: %v", err)
	}

	// Parse query: SELECT CAST(a AS FLOAT), CAST(b AS INT), CAST(c AS DATE) FROM csv(...)
	stmt := sql.Parse("SELECT CAST(a AS FLOAT), CAST(b AS INT), CAST(c AS DATE) FROM csv(\"" + csvPath + "\")")
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(types))
	}
	if types[0].Type != nodes.ColumnType_FLOAT || types[1].Type != nodes.ColumnType_INT || types[2].Type != nodes.ColumnType_DATE {
		t.Errorf("unexpected column types: %+v", types)
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	// Expected: float64(5.0), int(10), time.Time(2026-06-06)
	d, _ := time.Parse("2006-01-02", "2026-06-06")

	if rows[0][0].(float64) != 5.0 || rows[0][1].(int) != 10 || !rows[0][2].(time.Time).Equal(d) {
		t.Errorf("unexpected casted row: %+v", rows[0])
	}
}

func TestSQLiteSupport(t *testing.T) {
	// Create a temporary directory for the sqlite DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	// Open the sqlite DB
	db, err := dbsql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	// Create a table with different column types
	createTableSQL := `
	CREATE TABLE test_table (
		a INTEGER,
		b REAL,
		c TEXT,
		d DATE
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test data
	insertSQL := `
	INSERT INTO test_table (a, b, c, d) VALUES
	(1, 1.5, 'apple', '2026-06-01'),
	(2, 2.5, 'banana', '2026-06-06'),
	(3, 3.5, 'cherry', '2026-06-10');`
	if _, err := db.Exec(insertSQL); err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Parse query: SELECT a, b, c, d FROM sqlite("dbPath", "test_table") WHERE a > 1
	queryStr := fmt.Sprintf("SELECT a, b, c, d FROM sqlite(%q, %q) WHERE a > 1", dbPath, "test_table")
	stmt := sql.Parse(queryStr)
	node := nodes.NodeFromStatement(stmt)

	// Verify Types
	types := node.Types()
	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}
	if types[0].Name != "a" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column 0: %+v", types[0])
	}
	if types[1].Name != "b" || types[1].Type != nodes.ColumnType_FLOAT {
		t.Errorf("unexpected column 1: %+v", types[1])
	}
	if types[2].Name != "c" || types[2].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 2: %+v", types[2])
	}
	if types[3].Name != "d" || types[3].Type != nodes.ColumnType_DATE {
		t.Errorf("unexpected column 3: %+v", types[3])
	}

	// Verify Rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", len(rows))
	}

	d1, _ := time.Parse("2006-01-02", "2026-06-06")
	d2, _ := time.Parse("2006-01-02", "2026-06-10")

	// Row 1: 2, 2.5, 'banana', '2026-06-06'
	if rows[0][0].(int) != 2 || rows[0][1].(float64) != 2.5 || rows[0][2].(string) != "banana" || !rows[0][3].(time.Time).Equal(d1) {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	// Row 2: 3, 3.5, 'cherry', '2026-06-10'
	if rows[1][0].(int) != 3 || rows[1][1].(float64) != 3.5 || rows[1][2].(string) != "cherry" || !rows[1][3].(time.Time).Equal(d2) {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}
}

func TestJoinSupport(t *testing.T) {
	tempDir := t.TempDir()
	aPath := filepath.Join(tempDir, "a.csv")
	bPath := filepath.Join(tempDir, "b.csv")

	aData := `id,val
1,10
2,20
`
	bData := `id,name
1,apple
3,cherry
`
	if err := os.WriteFile(aPath, []byte(aData), 0644); err != nil {
		t.Fatalf("failed to write a.csv: %v", err)
	}
	if err := os.WriteFile(bPath, []byte(bData), 0644); err != nil {
		t.Fatalf("failed to write b.csv: %v", err)
	}

	// 1. Test INNER JOIN
	query1 := fmt.Sprintf("SELECT a.id, a.val, b.name FROM csv(%q) AS a JOIN csv(%q) AS b ON a.id = b.id", aPath, bPath)
	stmt1 := sql.Parse(query1)
	node1 := nodes.NodeFromStatement(stmt1)

	// Verify types
	types1 := node1.Types()
	if len(types1) != 3 {
		t.Fatalf("expected 3 columns for inner join, got %d", len(types1))
	}
	if types1[0].Name != "id" || types1[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column 0: %+v", types1[0])
	}
	if types1[1].Name != "val" || types1[1].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column 1: %+v", types1[1])
	}
	if types1[2].Name != "name" || types1[2].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 2: %+v", types1[2])
	}

	// Verify rows
	var rows1 [][]any
	for row := range node1.All() {
		rows1 = append(rows1, row.Value)
	}
	if len(rows1) != 1 {
		t.Fatalf("expected 1 row for inner join, got %d", len(rows1))
	}
	if rows1[0][0].(int) != 1 || rows1[0][1].(int) != 10 || rows1[0][2].(string) != "apple" {
		t.Errorf("unexpected inner join row: %+v", rows1[0])
	}

	// 2. Test LEFT JOIN
	query2 := fmt.Sprintf("SELECT a.id, a.val, b.name FROM csv(%q) AS a LEFT JOIN csv(%q) AS b ON a.id = b.id", aPath, bPath)
	stmt2 := sql.Parse(query2)
	node2 := nodes.NodeFromStatement(stmt2)

	// Verify rows
	var rows2 [][]any
	for row := range node2.All() {
		rows2 = append(rows2, row.Value)
	}
	if len(rows2) != 2 {
		t.Fatalf("expected 2 rows for left join, got %d", len(rows2))
	}
	// Row 1: [1, 10, "apple"]
	if rows2[0][0].(int) != 1 || rows2[0][1].(int) != 10 || rows2[0][2].(string) != "apple" {
		t.Errorf("unexpected left join row 0: %+v", rows2[0])
	}
	// Row 2: [2, 20, nil]
	if rows2[1][0].(int) != 2 || rows2[1][1].(int) != 20 || rows2[1][2] != nil {
		t.Errorf("unexpected left join row 1: %+v", rows2[1])
	}

	// 3. Test Qualified Star (a.*)
	query3 := fmt.Sprintf("SELECT a.*, b.name FROM csv(%q) AS a JOIN csv(%q) AS b ON a.id = b.id", aPath, bPath)
	stmt3 := sql.Parse(query3)
	node3 := nodes.NodeFromStatement(stmt3)

	types3 := node3.Types()
	if len(types3) != 3 {
		t.Fatalf("expected 3 columns for star query, got %d", len(types3))
	}
	if types3[0].Name != "id" || types3[1].Name != "val" || types3[2].Name != "name" {
		t.Errorf("unexpected star query schema: %+v", types3)
	}

	var rows3 [][]any
	for row := range node3.All() {
		rows3 = append(rows3, row.Value)
	}
	if len(rows3) != 1 {
		t.Fatalf("expected 1 row for star query, got %d", len(rows3))
	}
	if rows3[0][0].(int) != 1 || rows3[0][1].(int) != 10 || rows3[0][2].(string) != "apple" {
		t.Errorf("unexpected star query row: %+v", rows3[0])
	}
}

func TestSubselectSupport(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "data.csv")

	csvData := `a,n
1,10
1,15
2,20
2,25
`
	if err := os.WriteFile(csvPath, []byte(csvData), 0644); err != nil {
		t.Fatalf("failed to write CSV: %v", err)
	}

	// Parse query: SELECT m FROM (SELECT a, max(n) AS m FROM csv("csvPath") GROUP BY a) t
	query := fmt.Sprintf("SELECT m FROM (SELECT a, max(n) AS m FROM csv(%q) GROUP BY a) t", csvPath)
	stmt := sql.Parse(query)
	node := nodes.NodeFromStatement(stmt)

	// Verify types
	types := node.Types()
	if len(types) != 1 {
		t.Fatalf("expected 1 column, got %d", len(types))
	}
	if types[0].Name != "m" || types[0].Type != nodes.ColumnType_INT {
		t.Errorf("unexpected column: %+v", types[0])
	}

	// Verify rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0][0].(int) != 15 {
		t.Errorf("expected 15, got %v", rows[0][0])
	}
	if rows[1][0].(int) != 25 {
		t.Errorf("expected 25, got %v", rows[1][0])
	}
}

func TestComplexTypeSupport(t *testing.T) {
	// 1. Query unqualified dot access: d.k1, d.k2
	stmt1 := sql.Parse("SELECT d.k1, d.k2 FROM x")
	node1 := nodes.NodeFromStatement(stmt1)

	types1 := node1.Types()
	if len(types1) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types1))
	}
	if types1[0].Name != "k1" || types1[0].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 0: %+v", types1[0])
	}
	if types1[1].Name != "k2" || types1[1].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 1: %+v", types1[1])
	}

	count1 := 0
	for row := range node1.All() {
		count1++
		if row.Value[0].(string) != "nested_value" {
			t.Errorf("expected 'nested_value', got %v", row.Value[0])
		}
		if _, ok := row.Value[1].(int); !ok {
			t.Errorf("expected int, got %T (%v)", row.Value[1], row.Value[1])
		}
	}
	if count1 != 100 {
		t.Errorf("expected 100 rows, got %d", count1)
	}

	// 2. Query qualified dot access: t.d.k1, t.d.k2
	stmt2 := sql.Parse("SELECT t.d.k1, t.d.k2 FROM x AS t")
	node2 := nodes.NodeFromStatement(stmt2)

	types2 := node2.Types()
	if len(types2) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(types2))
	}
	if types2[0].Name != "k1" || types2[0].Type != nodes.ColumnType_STRING {
		t.Errorf("unexpected column 0: %+v", types2[0])
	}

	count2 := 0
	for row := range node2.All() {
		count2++
		if row.Value[0].(string) != "nested_value" {
			t.Errorf("expected 'nested_value', got %v", row.Value[0])
		}
	}
	if count2 != 100 {
		t.Errorf("expected 100 rows, got %d", count2)
	}
}

func TestOrderBySupport(t *testing.T) {
	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "order_test.jsonl")

	// 5 rows:
	// row 0: a=2, b="banana", c="2026-06-10"
	// row 1: a=1, b="apple", c="2026-06-01"
	// row 2: a=nil, b="cherry", c="2026-06-05"
	// row 3: a=1, b=nil, c="2026-06-08"
	// row 4: a=3, b="date", c=nil
	jsonlData := `{"a": 2, "b": "banana", "c": "2026-06-10"}
{"a": 1, "b": "apple", "c": "2026-06-01"}
{"b": "cherry", "c": "2026-06-05"}
{"a": 1, "c": "2026-06-08"}
{"a": 3, "b": "date"}
`
	if err := os.WriteFile(jsonlPath, []byte(jsonlData), 0644); err != nil {
		t.Fatalf("failed to write temp JSONL file: %v", err)
	}

	// Helper to run query and return the selected values
	runQuery := func(query string) [][]any {
		stmt := sql.Parse(query)
		node := nodes.NodeFromStatement(stmt)
		var rows [][]any
		for row := range node.All() {
			rows = append(rows, row.Value)
		}
		return rows
	}

	// 1. Basic ASC sort on 'a' (NULLs first)
	// Expected order of 'a': nil, 1 (apple), 1 (nil), 2, 3
	// Note: since it's a stable sort, the two 1s should preserve their input order (apple first, then nil)
	rows1 := runQuery(fmt.Sprintf("SELECT a, b FROM jsonl(%q) ORDER BY a ASC", jsonlPath))
	if len(rows1) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows1))
	}
	// Row 0: a=nil, b="cherry"
	if rows1[0][0] != nil || rows1[0][1].(string) != "cherry" {
		t.Errorf("expected cherry first, got %+v", rows1[0])
	}
	// Row 1: a=1, b="apple"
	if rows1[1][0].(int) != 1 || rows1[1][1].(string) != "apple" {
		t.Errorf("expected apple second, got %+v", rows1[1])
	}
	// Row 2: a=1, b=nil
	if rows1[2][0].(int) != 1 || rows1[2][1] != nil {
		t.Errorf("expected a=1 b=nil third, got %+v", rows1[2])
	}
	// Row 3: a=2, b="banana"
	if rows1[3][0].(int) != 2 || rows1[3][1].(string) != "banana" {
		t.Errorf("expected banana fourth, got %+v", rows1[3])
	}
	// Row 4: a=3, b="date"
	if rows1[4][0].(int) != 3 || rows1[4][1].(string) != "date" {
		t.Errorf("expected date fifth, got %+v", rows1[4])
	}

	// 2. DESC sort on 'a' (NULLs last)
	// Expected order: 3, 2, 1 (apple), 1 (nil), nil
	rows2 := runQuery(fmt.Sprintf("SELECT a, b FROM jsonl(%q) ORDER BY a DESC", jsonlPath))
	if len(rows2) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows2))
	}
	// Row 0: a=3, b="date"
	if rows2[0][0].(int) != 3 || rows2[0][1].(string) != "date" {
		t.Errorf("expected date first, got %+v", rows2[0])
	}
	// Row 1: a=2, b="banana"
	if rows2[1][0].(int) != 2 || rows2[1][1].(string) != "banana" {
		t.Errorf("expected banana second, got %+v", rows2[1])
	}
	// Row 2: a=1, b="apple"
	if rows2[2][0].(int) != 1 || rows2[2][1].(string) != "apple" {
		t.Errorf("expected apple third, got %+v", rows2[2])
	}
	// Row 3: a=1, b=nil
	if rows2[3][0].(int) != 1 || rows2[3][1] != nil {
		t.Errorf("expected a=1 b=nil fourth, got %+v", rows2[3])
	}
	// Row 4: a=nil, b="cherry"
	if rows2[4][0] != nil || rows2[4][1].(string) != "cherry" {
		t.Errorf("expected cherry last, got %+v", rows2[4])
	}

	// 3. Multi-column sort: a ASC, b DESC
	// Expected order:
	// a=nil (cherry)
	// a=1, b="apple" vs a=1, b=nil. Since b=nil is last in DESC (because default for DESC is nulls last),
	// "apple" > nil, so "apple" should come first, then nil.
	// a=2 (banana)
	// a=3 (date)
	rows3 := runQuery(fmt.Sprintf("SELECT a, b FROM jsonl(%q) ORDER BY a ASC, b DESC", jsonlPath))
	if rows3[1][1].(string) != "apple" || rows3[2][1] != nil {
		t.Errorf("expected stable multi-column sorting to place apple before nil, got %+v then %+v", rows3[1], rows3[2])
	}

	// 4. NULLS LAST with ASC
	// Expected: 1 (apple), 1 (nil), 2, 3, nil
	rows4 := runQuery(fmt.Sprintf("SELECT a FROM jsonl(%q) ORDER BY a ASC NULLS LAST", jsonlPath))
	if rows4[4][0] != nil {
		t.Errorf("expected last row to be nil, got %+v", rows4[4])
	}

	// 5. NULLS FIRST with DESC
	// Expected: nil, 3, 2, 1 (apple), 1 (nil)
	rows5 := runQuery(fmt.Sprintf("SELECT a FROM jsonl(%q) ORDER BY a DESC NULLS FIRST", jsonlPath))
	if rows5[0][0] != nil {
		t.Errorf("expected first row to be nil, got %+v", rows5[0])
	}

	// 6. 1-based index ordering: ORDER BY 2 DESC, 1 ASC
	// SELECT a, b ... ORDER BY 2 DESC
	// Ordering by 2nd column (b) DESC.
	// b values: banana, apple, cherry, nil, date.
	// Sorted b DESC (NULLs last): date, cherry, banana, apple, nil.
	rows6 := runQuery(fmt.Sprintf("SELECT a, b FROM jsonl(%q) ORDER BY 2 DESC, 1 ASC", jsonlPath))
	if rows6[0][1].(string) != "date" || rows6[1][1].(string) != "cherry" || rows6[2][1].(string) != "banana" || rows6[3][1].(string) != "apple" || rows6[4][1] != nil {
		t.Errorf("unexpected ordering by index: %+v", rows6)
	}

	// 7. Aliases and expressions: ORDER BY alias_a DESC, expr_a ASC
	// SELECT a AS alias_a, a + 10 AS expr_a FROM jsonl(...)
	rows7 := runQuery(fmt.Sprintf("SELECT a AS alias_a, a + 10 AS expr_a FROM jsonl(%q) ORDER BY alias_a DESC", jsonlPath))
	if rows7[0][0].(int) != 3 || rows7[1][0].(int) != 2 {
		t.Errorf("unexpected ordering by alias: %+v", rows7)
	}
}

func TestArrayTypeSupport(t *testing.T) {
	stmt := sql.Parse("SELECT CAST('[1, \"two\", 3.0]' AS ARRAY) FROM x")
	node := nodes.NodeFromStatement(stmt)

	types := node.Types()
	if len(types) != 1 {
		t.Fatalf("expected 1 column, got %d", len(types))
	}
	if types[0].Type != nodes.ColumnType_ARRAY {
		t.Errorf("expected type ColumnType_ARRAY, got %v", types[0].Type)
	}

	count := 0
	for row := range node.All() {
		count++
		val := row.Value[0]
		arr, ok := val.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T (%v)", val, val)
		}
		if len(arr) != 3 {
			t.Fatalf("expected array length 3, got %d", len(arr))
		}
		if arr[0].(float64) != 1 || arr[1].(string) != "two" || arr[2].(float64) != 3.0 {
			t.Errorf("unexpected array contents: %v", arr)
		}
	}
	if count != 100 {
		t.Errorf("expected 100 rows, got %d", count)
	}
}





