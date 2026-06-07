package sources

import (
	"iter"
	"math/rand"
	"time"

	"github.com/tbutter/qit/nodes"
)

// NewFakeNode creates a fake data node returning 100 rows of random data.
func NewFakeNode() *nodes.Node {
	return nodes.NewNode(&FakeNode{})
}

// FakeNode implements the fake data source.
type FakeNode struct{}

func (f *FakeNode) Types() []nodes.ColumnDef {
	return []nodes.ColumnDef{
		{Name: "a", Type: nodes.ColumnType_INT},
		{Name: "b", Type: nodes.ColumnType_INT},
		{Name: "c", Type: nodes.ColumnType_STRING},
		{Name: "d", Type: nodes.ColumnType_COMPLEX},
	}
}

func (f *FakeNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		cVals := []string{"apple", "banana", "cherry", "date", "elderberry", "fig", "grape", "honeydew", "kiwi", "lemon"}

		for i := 0; i < 100; i++ {
			// a INT: random between 1 and 10 (approx. 10% match rate for WHERE a = 5)
			aVal := r.Intn(10) + 1
			// b INT: random between 100 and 999
			bVal := r.Intn(900) + 100
			// c STRING: random string from cVals
			cVal := cVals[r.Intn(len(cVals))]
			dVal := map[string]any{
				"k1": "nested_value",
				"k2": aVal,
				"nested": map[string]any{
					"sub": "deep",
				},
			}

			row := nodes.Row{
				Value: []any{aVal, bVal, cVal, dVal},
			}
			if !yield(row) {
				return
			}
		}
	}
}
