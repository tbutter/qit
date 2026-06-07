package main

import (
	dbsql "database/sql"
	"log"
	"os"

	"github.com/tbutter/qit/nodes"
	_ "github.com/tbutter/qit/sources"
	"github.com/tbutter/qit/sql"
	_ "modernc.org/sqlite"
)

func main() {
	// Create a demo file with duplicate grouping keys
	csvData := `a,b,c
1,100,apple
1,150,apricot
2,200,banana
2,250,blueberry
3,300,cherry
`
	filename := "demo.csv"
	if err := os.WriteFile(filename, []byte(csvData), 0644); err != nil {
		log.Fatalf("failed to write demo csv: %v", err)
	}
	defer os.Remove(filename)

	// Query 1: GROUP BY a with aggregates including sum(b)
	stmt1 := sql.Parse("SELECT a, min(b), max(b), count(b), sum(b), count(*) FROM csv(\"demo.csv\") GROUP BY a")
	node1 := nodes.NodeFromStatement(stmt1)

	log.Println("--- Query 1: Aggregates ---")
	log.Println("Schema:")
	for _, col := range node1.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node1.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 2: SELECT with addition and subtraction expressions
	stmt2 := sql.Parse("SELECT a-b, a+b, a-b+10 FROM csv(\"demo.csv\")")
	node2 := nodes.NodeFromStatement(stmt2)

	log.Println("--- Query 2: Arithmetic ---")
	log.Println("Schema:")
	for _, col := range node2.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node2.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 3: SELECT with concat function call
	stmt3 := sql.Parse("SELECT concat(c, '-', a) FROM csv(\"demo.csv\")")
	node3 := nodes.NodeFromStatement(stmt3)

	log.Println("--- Query 3: Concat ---")
	log.Println("Schema:")
	for _, col := range node3.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node3.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Create a demo file with date columns
	csvData2 := `created_at,event
2026-06-01,signup
2026-06-06,login
2026-06-10,logout
`
	filename2 := "demo_dates.csv"
	if err := os.WriteFile(filename2, []byte(csvData2), 0644); err != nil {
		log.Fatalf("failed to write demo csv: %v", err)
	}
	defer os.Remove(filename2)

	// Query 4: SELECT with date comparison
	stmt4 := sql.Parse("SELECT created_at, event FROM csv(\"demo_dates.csv\") WHERE created_at > '2026-06-05'")
	node4 := nodes.NodeFromStatement(stmt4)

	log.Println("--- Query 4: Date Comparison ---")
	log.Println("Schema:")
	for _, col := range node4.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node4.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 5: SELECT with CAST expressions
	stmt5 := sql.Parse("SELECT CAST(a AS FLOAT), CAST(b AS VARCHAR) FROM csv(\"demo.csv\")")
	node5 := nodes.NodeFromStatement(stmt5)

	log.Println("--- Query 5: Cast ---")
	log.Println("Schema:")
	for _, col := range node5.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node5.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Create a demo SQLite database
	sqlitePath := "demo.sqlite"
	db, err := dbsql.Open("sqlite", sqlitePath)
	if err != nil {
		log.Fatalf("failed to open demo sqlite database: %v", err)
	}
	// Create table and insert some data
	_, _ = db.Exec("DROP TABLE IF EXISTS users")
	_, err = db.Exec("CREATE TABLE users (id INTEGER, name TEXT, balance REAL, joined_at DATE)")
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, name, balance, joined_at) VALUES
		(1, 'Alice', 100.50, '2026-01-01'),
		(2, 'Bob', 250.75, '2026-02-15'),
		(3, 'Charlie', 50.00, '2026-06-06')`)
	if err != nil {
		log.Fatalf("failed to insert demo data: %v", err)
	}
	db.Close()
	defer os.Remove(sqlitePath)

	// Query 6: SELECT from sqlite table
	stmt6 := sql.Parse("SELECT name, balance, joined_at FROM sqlite(\"demo.sqlite\", \"users\") WHERE id > 1")
	node6 := nodes.NodeFromStatement(stmt6)

	log.Println("--- Query 6: SQLite Source ---")
	log.Println("Schema:")
	for _, col := range node6.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node6.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 7: SELECT with JOIN (CSV and SQLite joined together)
	stmt7 := sql.Parse("SELECT c.a, c.c, u.name, u.balance FROM csv(\"demo.csv\") AS c JOIN sqlite(\"demo.sqlite\", \"users\") AS u ON c.a = u.id")
	node7 := nodes.NodeFromStatement(stmt7)

	log.Println("--- Query 7: CSV and SQLite JOIN ---")
	log.Println("Schema:")
	for _, col := range node7.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node7.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 8: SELECT with Subselect
	stmt8 := sql.Parse("SELECT m FROM (SELECT a, max(b) AS m FROM csv(\"demo.csv\") GROUP BY a) t")
	node8 := nodes.NodeFromStatement(stmt8)

	log.Println("--- Query 8: Subselect ---")
	log.Println("Schema:")
	for _, col := range node8.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node8.All() {
		log.Printf("%+v\n", row.Value)
	}

	// Query 9: SELECT with COMPLEX dot-notation access
	stmt9 := sql.Parse("SELECT a, d.k1, d.k2 FROM x WHERE a = 5")
	node9 := nodes.NodeFromStatement(stmt9)

	log.Println("--- Query 9: COMPLEX Dot Access ---")
	log.Println("Schema:")
	for _, col := range node9.Types() {
		log.Printf("- %s (Type: %d)\n", col.Name, col.Type)
	}
	log.Println("Rows:")
	for row := range node9.All() {
		log.Printf("%+v\n", row.Value)
	}
}
