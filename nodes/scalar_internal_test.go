package nodes

import (
	"testing"
)

func TestConcatFunc(t *testing.T) {
	fn := concatFunc{}

	// 1. Test Type()
	if fn.Type(nil) != ColumnType_STRING {
		t.Errorf("expected Type(nil) to be STRING")
	}
	if fn.Type([]ColumnType{ColumnType_INT, ColumnType_FLOAT}) != ColumnType_STRING {
		t.Errorf("expected Type([INT, FLOAT]) to be STRING")
	}

	// 2. Test Eval() with multiple strings
	res, err := fn.Eval([]any{"hello", " ", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "hello world" {
		t.Errorf("expected 'hello world', got %v", res)
	}

	// 3. Test Eval() with mixed types (string, int, float)
	res, err = fn.Eval([]any{"count: ", 10, ", ratio: ", 0.75})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "count: 10, ratio: 0.75" {
		t.Errorf("expected 'count: 10, ratio: 0.75', got %v", res)
	}

	// 4. Test Eval() with nil/NULL values (should be skipped/ignored)
	res, err = fn.Eval([]any{"apple", nil, "-", nil, "pie"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "apple-pie" {
		t.Errorf("expected 'apple-pie', got %v", res)
	}

	// 5. Test Eval() with empty arguments
	res, err = fn.Eval(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "" {
		t.Errorf("expected empty string, got %v", res)
	}
}
