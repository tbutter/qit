package exporters

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tbutter/qit/nodes"
)

// ExportTable writes the result rows of a Node as a formatted ASCII table to the given writer.
func ExportTable(w io.Writer, node *nodes.Node) error {
	types := node.Types()
	numCols := len(types)
	if numCols == 0 {
		return nil
	}

	// 1. Gather headers and compute initial column widths based on header name lengths
	headers := make([]string, numCols)
	widths := make([]int, numCols)
	for i, col := range types {
		headers[i] = col.Name
		widths[i] = len(col.Name)
	}

	// 2. Read all rows into memory, formatting values and adjusting column widths
	var rows [][]string
	for row := range node.All() {
		strRow := make([]string, numCols)
		for i, val := range row.Value {
			if val == nil {
				strRow[i] = "NULL"
			} else {
				switch v := val.(type) {
				case string:
					strRow[i] = v
				case int:
					strRow[i] = strconv.Itoa(v)
				case float64:
					strRow[i] = strconv.FormatFloat(v, 'f', -1, 64)
				case bool:
					strRow[i] = strconv.FormatBool(v)
				case time.Time:
					if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
						strRow[i] = v.Format("2006-01-02")
					} else {
						strRow[i] = v.Format("2006-01-02 15:04:05")
					}
				case map[string]any, []any:
					bytes, err := json.Marshal(v)
					if err != nil {
						strRow[i] = fmt.Sprintf("%v", v)
					} else {
						strRow[i] = string(bytes)
					}
				default:
					if bytes, err := json.Marshal(v); err == nil {
						strRow[i] = string(bytes)
					} else {
						strRow[i] = fmt.Sprintf("%v", v)
					}
				}
			}
			if len(strRow[i]) > widths[i] {
				widths[i] = len(strRow[i])
			}
		}
		rows = append(rows, strRow)
	}

	// 3. Build top/middle/bottom separators
	var sepBuilder strings.Builder
	sepBuilder.WriteByte('+')
	for _, w := range widths {
		sepBuilder.WriteString(strings.Repeat("-", w+2))
		sepBuilder.WriteByte('+')
	}
	separator := sepBuilder.String()

	// 4. Write ASCII table to the destination writer
	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}

	// Header row
	var headerBuilder strings.Builder
	headerBuilder.WriteByte('|')
	for i, name := range headers {
		headerBuilder.WriteString(" ")
		headerBuilder.WriteString(padString(name, widths[i], types[i].Type))
		headerBuilder.WriteString(" |")
	}
	if _, err := fmt.Fprintln(w, headerBuilder.String()); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}

	// Data rows
	for _, row := range rows {
		var rowBuilder strings.Builder
		rowBuilder.WriteByte('|')
		for i, val := range row {
			rowBuilder.WriteString(" ")
			rowBuilder.WriteString(padString(val, widths[i], types[i].Type))
			rowBuilder.WriteString(" |")
		}
		if _, err := fmt.Fprintln(w, rowBuilder.String()); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}

	return nil
}

// padString pads a value to match the column width.
// Numeric fields (INT, FLOAT) are right-aligned, other types are left-aligned.
func padString(s string, width int, colType nodes.ColumnType) string {
	extra := width - len(s)
	if extra <= 0 {
		return s
	}

	switch colType {
	case nodes.ColumnType_INT, nodes.ColumnType_FLOAT:
		return strings.Repeat(" ", extra) + s
	default:
		return s + strings.Repeat(" ", extra)
	}
}
