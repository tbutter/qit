package sql

import (
	"log"
	"strings"

	sqlp "github.com/rqlite/sql"
)

func Parse(sql string) sqlp.Statement {
	stmt, err := sqlp.NewParser(strings.NewReader(sql)).ParseStatement()
	if err != nil {
		log.Fatal(err)
	}
	return stmt
}
