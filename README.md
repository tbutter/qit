# qit

`qit` is a lightweight, single-binary SQL query execution engine written in Go. It enables you to run standard SQL queries directly against diverse datasets—including local **CSV** files, newline-delimited **JSON (JSONL)**, **SQLite** databases, and **Google Sheets**—without needing to spin up or import data into a heavy database server.

---

## Features

* **Diverse Table Sources**: Query different data sources seamlessly:
  * `csv("path/to/file.csv")`
  * `jsonl("path/to/file.jsonl")`
  * `sqlite("path/to/db.sqlite", "table_name")`
  * `gsheet("url_or_spreadsheet_id", "sheet_name")`
* **SQL Query Capabilities**: Supporting `SELECT`, `WHERE`, `JOIN` (multi-source joins), `GROUP BY` with aggregate functions (`min`, `max`, `sum`, `count`, `count(*)`), `ORDER BY` (including `NULLS FIRST`/`NULLS LAST`), and subselects.
* **Complex Nested JSON**: Dot-notation access for nested maps (`COMPLEX` type) within JSONL/SQLite sources (e.g., `SELECT user.profile.name FROM ...`).
* **Array Support**: Support for `ARRAY` column types representing list structures (`[]any`).
* **Flexible Exporters**: Output query results as ASCII tables (`table`), CSV files (`csv`), raw logs (`log`), or directly back into a Google Sheet (`gsheet`).

---

## Installation & Setup

Ensure you have [Go](https://go.dev/) (v1.26+) installed on your machine.

Clone the repository and compile or run directly:

```bash
# Build the qit command-line utility
go build -o qit main.go

# Or run directly using Go
go run main.go --help
```

### Nix Flake & Nix Shell Usage

If you use Nix, you can build, run, or develop `qit` easily:

* **Run Directly (Without Cloning)**: Run `qit` immediately using the remote flake:
  ```bash
  nix run github:tbutter/qit -- -q "SELECT 1, 2, 3"
  ```
* **Temporary Shell**: Open a shell with `qit` added to your `$PATH` without cloning the repository:
  ```bash
  nix shell github:tbutter/qit
  # You can now run the tool directly:
  qit -q "SELECT 1, 2, 3"
  ```

---

## CLI Usage

`qit` can be run either interactively or as a single command-line execution by passing the query flag.

```bash
# General CLI arguments
qit -q "<SQL_QUERY>" -f <FORMAT> -o <OUTPUT_FILE>
```

### Options
* `-q`, `--query` (string): The SQL query to run.
* `-f`, `--format` (string): The output format (`table` [default], `csv`, `log`, `gsheet`).
* `-o`, `--output` (string): Output file path (defaults to `stdout`), or spreadsheet target for `gsheet` format (specified as `<spreadsheet_id_or_url>` or `<spreadsheet_id_or_url>:<sheet_name>`).

---

## Quick Examples

### 1. Querying CSV Files with Aggregations
Query columns from a CSV file, group by keys, and run aggregate calculations:
```bash
go run main.go -q "SELECT department, count(*), sum(salary), max(salary) FROM csv(\"employees.csv\") GROUP BY department"
```

### 2. Querying JSONL Files with Arrays and Dot-Notation
Retrieve nested JSON properties and lists:
```bash
go run main.go -q "SELECT id, user.name, CAST(tags AS ARRAY) FROM jsonl(\"users.jsonl\") WHERE user.active = 1"
```

### 3. Multi-Source JOIN (CSV and SQLite)
Join a local CSV file with an existing SQLite database table in a single query:
```bash
go run main.go -q "SELECT c.category, u.name, u.balance FROM csv(\"transactions.csv\") AS c JOIN sqlite(\"users.db\", \"accounts\") AS u ON c.user_id = u.id"
```

### 4. Querying a Google Sheet
Query data directly from a Google Sheet (initial run will trigger a secure browser OAuth2 authorization flow):
```bash
go run main.go -q "SELECT name, email, joined_on FROM gsheet(\"https://docs.google.com/spreadsheets/d/1BxiMVs0XRA.../edit\", \"Sheet1\") WHERE joined_on > '2026-01-01'"
```

---

## Programmatic Exporters

You can also use `qit` programmatically in Go to export nodes to files or remote destinations:

```go
import (
	"github.com/tbutter/qit/exporters"
	"github.com/tbutter/qit/nodes"
)

// Export a query node to an ASCII table writer
err := exporters.ExportTable(os.Stdout, node)

// Export to a CSV file writer
err := exporters.ExportCSV(csvFileWriter, node)

// Export query results directly back into a Google Sheet
err := exporters.ExportGSheet("spreadsheet_id", "SheetName", node)
```

---

## Reference Documentation
For in-depth details on syntax and behavior, see:
* [SOURCES.md](SOURCES.md) - Details on table functions and type inference rules.
* [FUNCTIONS.md](FUNCTIONS.md) - Details on SQL functions, type casting, and schema definitions.
