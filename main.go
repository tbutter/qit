package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tbutter/qit/exporters"
	"github.com/tbutter/qit/nodes"
	_ "github.com/tbutter/qit/sources"
	"github.com/tbutter/qit/sql"
	_ "modernc.org/sqlite"
)

func main() {
	var query string
	flag.StringVar(&query, "query", "", "SQL query to execute")
	flag.StringVar(&query, "q", "", "SQL query to execute (shorthand)")

	var format string
	flag.StringVar(&format, "format", "table", "Output format (table, csv, log, gsheet)")
	flag.StringVar(&format, "f", "table", "Output format (table, csv, log, gsheet) (shorthand)")

	var outputFile string
	flag.StringVar(&outputFile, "output", "", "Output file path (default stdout)")
	flag.StringVar(&outputFile, "o", "", "Output file path (default stdout) (shorthand)")

	flag.Parse()

	// If no flag was explicitly passed, check positional arguments
	if query == "" && flag.NArg() > 0 {
		query = flag.Arg(0)
	}

	if query != "" {
		stmt := sql.Parse(query)
		node := nodes.NodeFromStatement(stmt)

		// Determine destination writer
		var w io.Writer = os.Stdout
		if outputFile != "" {
			f, err := os.Create(outputFile)
			if err != nil {
				log.Fatalf("failed to create output file: %v", err)
			}
			defer f.Close()
			w = f
		}

		if format == "csv" {
			if err := exporters.ExportCSV(w, node); err != nil {
				log.Fatalf("failed to export CSV: %v", err)
			}
		} else if format == "table" {
			if err := exporters.ExportTable(w, node); err != nil {
				log.Fatalf("failed to export table: %v", err)
			}
		} else if format == "gsheet" {
			if outputFile == "" {
				log.Fatal("output spreadsheet ID or URL is required for gsheet format (e.g. -o <spreadsheet_id> or -o <spreadsheet_id>:<sheet_name>)")
			}
			spreadsheetID := outputFile
			sheetName := ""
			// If it has a colon that isn't part of http/https protocol prefix, split it.
			if idx := strings.LastIndex(outputFile, ":"); idx != -1 && idx > 5 {
				spreadsheetID = outputFile[:idx]
				sheetName = outputFile[idx+1:]
			}
			if err := exporters.ExportGSheet(spreadsheetID, sheetName, node); err != nil {
				log.Fatalf("failed to export to Google Sheets: %v", err)
			}
		} else {
			fmt.Fprintln(w, "Schema:")
			for _, col := range node.Types() {
				fmt.Fprintf(w, "- %s (Type: %d)\n", col.Name, col.Type)
			}
			fmt.Fprintln(w, "Rows:")
			for row := range node.All() {
				fmt.Fprintf(w, "%+v\n", row.Value)
			}
		}
		return
	}

	flag.Usage()
	os.Exit(1)
}
