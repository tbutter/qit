package sources

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"strconv"
	"strings"

	"github.com/tbutter/qit/nodes"
)

// NewCSVNode creates a new Node that reads data from a CSV file.
func NewCSVNode(filename string) (*nodes.Node, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %w", err)
	}

	firstRow, err := reader.Read()
	colTypes := make([]nodes.ColumnType, len(header))
	for i := range colTypes {
		colTypes[i] = nodes.ColumnType_STRING
	}

	if err == nil {
		for i, val := range firstRow {
			if i < len(colTypes) {
				if _, err := strconv.Atoi(val); err == nil {
					colTypes[i] = nodes.ColumnType_INT
				} else if _, err := strconv.ParseFloat(val, 64); err == nil {
					colTypes[i] = nodes.ColumnType_FLOAT
				} else if _, err := nodes.ParseDate(val); err == nil {
					colTypes[i] = nodes.ColumnType_DATE
				}
			}
		}
	}

	defs := make([]nodes.ColumnDef, len(header))
	for i, name := range header {
		defs[i] = nodes.ColumnDef{
			Name: strings.TrimSpace(name),
			Type: colTypes[i],
		}
	}

	return nodes.NewNode(&CSVNode{
		filename: filename,
		headers:  header,
		types:    defs,
	}), nil
}

// CSVNode reads and parses rows from a CSV file.
type CSVNode struct {
	filename string
	headers  []string
	types    []nodes.ColumnDef
}

func (c *CSVNode) Types() []nodes.ColumnDef {
	return c.types
}

func (c *CSVNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		file, err := os.Open(c.filename)
		if err != nil {
			return
		}
		defer file.Close()

		reader := csv.NewReader(file)
		// Skip header
		if _, err := reader.Read(); err != nil {
			return
		}

		for {
			record, err := reader.Read()
			if err != nil {
				break
			}

			rowVals := make([]any, len(c.headers))
			for i, val := range record {
				if i >= len(rowVals) {
					break
				}
				if c.types[i].Type == nodes.ColumnType_INT {
					if intVal, err := strconv.Atoi(val); err == nil {
						rowVals[i] = intVal
						continue
					}
				} else if c.types[i].Type == nodes.ColumnType_FLOAT {
					if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
						rowVals[i] = floatVal
						continue
					}
				} else if c.types[i].Type == nodes.ColumnType_DATE {
					if dateVal, err := nodes.ParseDate(val); err == nil {
						rowVals[i] = dateVal
						continue
					}
				} else if c.types[i].Type == nodes.ColumnType_COMPLEX {
					var m map[string]any
					if err := json.Unmarshal([]byte(val), &m); err == nil {
						rowVals[i] = m
						continue
					}
				} else if c.types[i].Type == nodes.ColumnType_ARRAY {
					var arr []any
					if err := json.Unmarshal([]byte(val), &arr); err == nil {
						rowVals[i] = arr
						continue
					}
				}
				rowVals[i] = val
			}

			if !yield(nodes.Row{Value: rowVals}) {
				return
			}
		}
	}
}
