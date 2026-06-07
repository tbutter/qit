package sources

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"

	"github.com/tbutter/qit/nodes"
	_ "modernc.org/sqlite"
)

// NewSQLiteNode creates a new Node that reads data from a SQLite table.
func NewSQLiteNode(dbPath string, tableName string) (*nodes.Node, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query table info: %w", err)
	}
	defer rows.Close()

	var colDefs []nodes.ColumnDef
	for rows.Next() {
		var cid int
		var name string
		var typeStr string
		var notnull int
		var dfltVal any
		var pk int
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltVal, &pk); err != nil {
			return nil, err
		}

		typeStr = strings.ToUpper(typeStr)
		var colType nodes.ColumnType = nodes.ColumnType_STRING
		if strings.Contains(typeStr, "INT") {
			colType = nodes.ColumnType_INT
		} else if strings.Contains(typeStr, "CHAR") || strings.Contains(typeStr, "TEXT") || strings.Contains(typeStr, "CLOB") {
			colType = nodes.ColumnType_STRING
		} else if strings.Contains(typeStr, "REAL") || strings.Contains(typeStr, "FLOA") || strings.Contains(typeStr, "DOUB") {
			colType = nodes.ColumnType_FLOAT
		} else if strings.Contains(typeStr, "DATE") || strings.Contains(typeStr, "TIME") {
			colType = nodes.ColumnType_DATE
		} else if strings.Contains(typeStr, "ARRAY") {
			colType = nodes.ColumnType_ARRAY
		}

		colDefs = append(colDefs, nodes.ColumnDef{
			Name: name,
			Type: colType,
		})
	}

	if len(colDefs) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", tableName)
	}

	return nodes.NewNode(&SQLiteNode{
		dbPath:    dbPath,
		tableName: tableName,
		types:     colDefs,
	}), nil
}

// SQLiteNode reads and parses rows from a SQLite table.
type SQLiteNode struct {
	dbPath    string
	tableName string
	types     []nodes.ColumnDef
}

func (s *SQLiteNode) Types() []nodes.ColumnDef {
	return s.types
}

func (s *SQLiteNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		db, err := sql.Open("sqlite", s.dbPath)
		if err != nil {
			return
		}
		defer db.Close()

		var colNames []string
		for _, col := range s.types {
			colNames = append(colNames, fmt.Sprintf("%q", col.Name))
		}
		query := fmt.Sprintf("SELECT %s FROM %q", strings.Join(colNames, ", "), s.tableName)

		rows, err := db.Query(query)
		if err != nil {
			return
		}
		defer rows.Close()

		scanArgs := make([]any, len(s.types))
		for i := range scanArgs {
			var val any
			scanArgs[i] = &val
		}

		for rows.Next() {
			if err := rows.Scan(scanArgs...); err != nil {
				break
			}

			rowVals := make([]any, len(s.types))
			for i, arg := range scanArgs {
				rawVal := *(arg.(*any))
				if rawVal == nil {
					rowVals[i] = nil
					continue
				}

				switch s.types[i].Type {
				case nodes.ColumnType_INT:
					switch v := rawVal.(type) {
					case int64:
						rowVals[i] = int(v)
					case int:
						rowVals[i] = v
					case float64:
						rowVals[i] = int(v)
					case string:
						if parsed, err := strconv.Atoi(v); err == nil {
							rowVals[i] = parsed
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}
				case nodes.ColumnType_FLOAT:
					switch v := rawVal.(type) {
					case float64:
						rowVals[i] = v
					case int64:
						rowVals[i] = float64(v)
					case int:
						rowVals[i] = float64(v)
					case string:
						if parsed, err := strconv.ParseFloat(v, 64); err == nil {
							rowVals[i] = parsed
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}
				case nodes.ColumnType_DATE:
					switch v := rawVal.(type) {
					case time.Time:
						rowVals[i] = v
					case string:
						if parsed, err := nodes.ParseDate(v); err == nil {
							rowVals[i] = parsed
						} else {
							rowVals[i] = v
						}
					case int64:
						rowVals[i] = time.Unix(v, 0).UTC()
					default:
						rowVals[i] = v
					}
				case nodes.ColumnType_COMPLEX:
					switch v := rawVal.(type) {
					case map[string]any:
						rowVals[i] = v
					case string:
						var m map[string]any
						if err := json.Unmarshal([]byte(v), &m); err == nil {
							rowVals[i] = m
						} else {
							rowVals[i] = v
						}
					case []byte:
						var m map[string]any
						if err := json.Unmarshal(v, &m); err == nil {
							rowVals[i] = m
						} else {
							rowVals[i] = string(v)
						}
					default:
						rowVals[i] = v
					}

				case nodes.ColumnType_ARRAY:
					switch v := rawVal.(type) {
					case []any:
						rowVals[i] = v
					case string:
						var arr []any
						if err := json.Unmarshal([]byte(v), &arr); err == nil {
							rowVals[i] = arr
						} else {
							rowVals[i] = v
						}
					case []byte:
						var arr []any
						if err := json.Unmarshal(v, &arr); err == nil {
							rowVals[i] = arr
						} else {
							rowVals[i] = string(v)
						}
					default:
						rowVals[i] = v
					}
				case nodes.ColumnType_STRING:
					switch v := rawVal.(type) {
					case []byte:
						rowVals[i] = string(v)
					case string:
						rowVals[i] = v
					default:
						rowVals[i] = fmt.Sprintf("%v", v)
					}
				default:
					rowVals[i] = rawVal
				}
			}

			if !yield(nodes.Row{Value: rowVals}) {
				return
			}
		}
	}
}
