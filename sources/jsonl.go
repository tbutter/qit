package sources

import (
	"bufio"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tbutter/qit/nodes"
)

// NewJSONLNode creates a new Node that reads data from a JSON Lines (JSONL) file.
func NewJSONLNode(filename string) (*nodes.Node, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var firstLine string
	for {
		lineBytes, err := reader.ReadBytes('\n')
		if err != nil && len(lineBytes) == 0 {
			break
		}
		line := strings.TrimSpace(string(lineBytes))
		if line != "" {
			firstLine = line
			break
		}
		if err != nil {
			break
		}
	}

	if firstLine == "" {
		return nil, fmt.Errorf("JSONL file is empty: %s", filename)
	}

	var firstObj map[string]any
	if err := json.Unmarshal([]byte(firstLine), &firstObj); err != nil {
		return nil, fmt.Errorf("error parsing first JSONL line: %w", err)
	}

	// Extract keys and sort them for deterministic order
	var keys []string
	for k := range firstObj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	defs := make([]nodes.ColumnDef, len(keys))
	for i, key := range keys {
		val := firstObj[key]
		var colType nodes.ColumnType = nodes.ColumnType_STRING

		if val != nil {
			switch v := val.(type) {
			case bool:
				colType = nodes.ColumnType_INT
			case float64:
				// JSON numbers are parsed as float64 by default.
				// If it's an integer, map to INT, otherwise FLOAT.
				if v == float64(int(v)) {
					colType = nodes.ColumnType_INT
				} else {
					colType = nodes.ColumnType_FLOAT
				}
			case string:
				if _, err := nodes.ParseDate(v); err == nil {
					colType = nodes.ColumnType_DATE
				} else {
					colType = nodes.ColumnType_STRING
				}
			case map[string]any:
				colType = nodes.ColumnType_COMPLEX
			case []any:
				colType = nodes.ColumnType_ARRAY
			default:
				colType = nodes.ColumnType_STRING
			}
		}

		defs[i] = nodes.ColumnDef{
			Name: key,
			Type: colType,
		}
	}

	return nodes.NewNode(&JSONLNode{
		filename: filename,
		keys:     keys,
		types:    defs,
	}), nil
}

// JSONLNode reads and parses rows from a JSONL file.
type JSONLNode struct {
	filename string
	keys     []string
	types    []nodes.ColumnDef
}

func (j *JSONLNode) Types() []nodes.ColumnDef {
	return j.types
}

func (j *JSONLNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		file, err := os.Open(j.filename)
		if err != nil {
			return
		}
		defer file.Close()

		reader := bufio.NewReader(file)
		for {
			lineBytes, err := reader.ReadBytes('\n')
			if err != nil && len(lineBytes) == 0 {
				break
			}
			line := strings.TrimSpace(string(lineBytes))
			if line == "" {
				if err != nil {
					break
				}
				continue
			}

			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				break
			}

			rowVals := make([]any, len(j.keys))
			for i, key := range j.keys {
				val, exists := obj[key]
				if !exists || val == nil {
					rowVals[i] = nil
					continue
				}

				switch j.types[i].Type {
				case nodes.ColumnType_INT:
					switch v := val.(type) {
					case bool:
						if v {
							rowVals[i] = 1
						} else {
							rowVals[i] = 0
						}
					case float64:
						rowVals[i] = int(v)
					case string:
						if parsed, err := strconv.Atoi(v); err == nil {
							rowVals[i] = parsed
						} else if parsedF, err := strconv.ParseFloat(v, 64); err == nil {
							rowVals[i] = int(parsedF)
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}

				case nodes.ColumnType_FLOAT:
					switch v := val.(type) {
					case float64:
						rowVals[i] = v
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
					switch v := val.(type) {
					case string:
						if parsed, err := nodes.ParseDate(v); err == nil {
							rowVals[i] = parsed
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}

				case nodes.ColumnType_COMPLEX:
					switch v := val.(type) {
					case map[string]any:
						rowVals[i] = v
					case string:
						var m map[string]any
						if err := json.Unmarshal([]byte(v), &m); err == nil {
							rowVals[i] = m
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}

				case nodes.ColumnType_ARRAY:
					switch v := val.(type) {
					case []any:
						rowVals[i] = v
					case string:
						var arr []any
						if err := json.Unmarshal([]byte(v), &arr); err == nil {
							rowVals[i] = arr
						} else {
							rowVals[i] = v
						}
					default:
						rowVals[i] = v
					}

				case nodes.ColumnType_STRING:
					switch v := val.(type) {
					case string:
						rowVals[i] = v
					case map[string]any, []any:
						if bytes, err := json.Marshal(v); err == nil {
							rowVals[i] = string(bytes)
						} else {
							rowVals[i] = fmt.Sprintf("%v", v)
						}
					default:
						rowVals[i] = fmt.Sprintf("%v", v)
					}
				default:
					rowVals[i] = val
				}
			}

			if !yield(nodes.Row{Value: rowVals}) {
				return
			}

			if err != nil {
				break
			}
		}
	}
}
