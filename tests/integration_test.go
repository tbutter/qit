package tests

import (
	"reflect"
	"testing"
	"time"

	"github.com/tbutter/qit/nodes"
	_ "github.com/tbutter/qit/sources"
	"github.com/tbutter/qit/sql"
)

type testCase struct {
	name          string
	query         string
	expectedCols  []string
	expectedTypes []nodes.ColumnType
	expectedRows  [][]any
}

func parseTestDate(s string) time.Time {
	t, err := nodes.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestQuerySuite(t *testing.T) {
	testCases := []testCase{
		{
			name:          "Filter and Project",
			query:         `SELECT name, age FROM csv("testdata/users.csv") WHERE age > 25 ORDER BY age ASC`,
			expectedCols:  []string{"name", "age"},
			expectedTypes: []nodes.ColumnType{nodes.ColumnType_STRING, nodes.ColumnType_INT},
			expectedRows: [][]any{
				{"David", 28},
				{"Alice", 30},
				{"Charlie", 35},
			},
		},
		{
			name:          "Group By and Aggregate",
			query:         `SELECT city, count(*) AS user_count, sum(age) AS total_age FROM csv("testdata/users.csv") GROUP BY city ORDER BY city ASC`,
			expectedCols:  []string{"city", "user_count", "total_age"},
			expectedTypes: []nodes.ColumnType{nodes.ColumnType_STRING, nodes.ColumnType_INT, nodes.ColumnType_INT},
			expectedRows: [][]any{
				{"New York", 2, 58},
				{"San Francisco", 2, 47},
				{"Seattle", 1, 35},
			},
		},
		{
			name:          "Inner Join",
			query:         `SELECT u.name, o.amount FROM csv("testdata/users.csv") AS u JOIN csv("testdata/orders.csv") AS o ON u.id = o.user_id ORDER BY o.amount DESC`,
			expectedCols:  []string{"name", "amount"},
			expectedTypes: []nodes.ColumnType{nodes.ColumnType_STRING, nodes.ColumnType_FLOAT},
			expectedRows: [][]any{
				{"Bob", 150.0},
				{"David", 120.0},
				{"Alice", 99.99},
				{"Alice", 49.50},
				{"Charlie", 20.0},
			},
		},
		{
			name:          "Date Filter",
			query:         `SELECT id, amount, order_date FROM csv("testdata/orders.csv") WHERE order_date >= '2026-02-01' ORDER BY order_date ASC`,
			expectedCols:  []string{"id", "amount", "order_date"},
			expectedTypes: []nodes.ColumnType{nodes.ColumnType_INT, nodes.ColumnType_FLOAT, nodes.ColumnType_DATE},
			expectedRows: [][]any{
				{103, 150.0, parseTestDate("2026-02-01")},
				{104, 20.0, parseTestDate("2026-02-15")},
				{105, 120.0, parseTestDate("2026-03-01")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stmt := sql.Parse(tc.query)
			node := nodes.NodeFromStatement(stmt)

			// 1. Verify schema
			cols := node.Types()
			if len(cols) != len(tc.expectedCols) {
				t.Fatalf("expected %d columns, got %d", len(tc.expectedCols), len(cols))
			}

			for i, col := range cols {
				if col.Name != tc.expectedCols[i] {
					t.Errorf("expected column at index %d to be %q, got %q", i, tc.expectedCols[i], col.Name)
				}
				if col.Type != tc.expectedTypes[i] {
					t.Errorf("expected column at index %d to be type %v, got %v", i, tc.expectedTypes[i], col.Type)
				}
			}

			// 2. Verify rows
			var actualRows [][]any
			for row := range node.All() {
				actualRows = append(actualRows, row.Value)
			}

			if len(actualRows) != len(tc.expectedRows) {
				t.Fatalf("expected %d rows, got %d", len(tc.expectedRows), len(actualRows))
			}

			for rowIndex, actualRow := range actualRows {
				expectedRow := tc.expectedRows[rowIndex]
				if len(actualRow) != len(expectedRow) {
					t.Fatalf("row %d has %d fields, expected %d", rowIndex, len(actualRow), len(expectedRow))
				}

				for colIndex, actualVal := range actualRow {
					expectedVal := expectedRow[colIndex]

					// Custom check for dates/times
					if actTime, ok := actualVal.(time.Time); ok {
						expTime, ok := expectedVal.(time.Time)
						if !ok {
							t.Errorf("row %d column %d: expected value is not time.Time", rowIndex, colIndex)
						} else if !actTime.Equal(expTime) {
							t.Errorf("row %d column %d: expected date %v, got %v", rowIndex, colIndex, expTime, actTime)
						}
						continue
					}

					// Standard check for other types
					if !reflect.DeepEqual(actualVal, expectedVal) {
						t.Errorf("row %d column %d: expected %+v (%T), got %+v (%T)", rowIndex, colIndex, expectedVal, expectedVal, actualVal, actualVal)
					}
				}
			}
		})
	}
}
