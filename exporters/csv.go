package exporters

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/tbutter/qit/nodes"
)

// ExportCSV writes the result rows of a Node as CSV data to the given writer.
func ExportCSV(w io.Writer, node *nodes.Node) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// 1. Write header row
	types := node.Types()
	header := make([]string, len(types))
	for i, col := range types {
		header[i] = col.Name
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// 2. Write data rows
	for row := range node.All() {
		csvRow := make([]string, len(row.Value))
		for i, val := range row.Value {
			if val == nil {
				csvRow[i] = ""
				continue
			}

			switch v := val.(type) {
			case string:
				csvRow[i] = v
			case int:
				csvRow[i] = strconv.Itoa(v)
			case float64:
				csvRow[i] = strconv.FormatFloat(v, 'f', -1, 64)
			case bool:
				csvRow[i] = strconv.FormatBool(v)
			case time.Time:
				// If time portion is zero, format as date-only. Otherwise format with time.
				if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
					csvRow[i] = v.Format("2006-01-02")
				} else {
					csvRow[i] = v.Format("2006-01-02 15:04:05")
				}
			case map[string]any, []any:
				bytes, err := json.Marshal(v)
				if err != nil {
					csvRow[i] = fmt.Sprintf("%v", v)
				} else {
					csvRow[i] = string(bytes)
				}
			default:
				// Try JSON marshal, fallback to fmt.Sprintf
				if bytes, err := json.Marshal(v); err == nil {
					csvRow[i] = string(bytes)
				} else {
					csvRow[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		if err := writer.Write(csvRow); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return writer.Error()
}
