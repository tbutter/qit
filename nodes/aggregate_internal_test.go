package nodes

import (
	"testing"
)

func TestMinFunc(t *testing.T) {
	fn := minFunc{}

	// Test Type()
	if fn.Type(nil) != ColumnType_STRING {
		t.Errorf("expected Type(nil) to be STRING")
	}
	if fn.Type([]ColumnType{ColumnType_INT}) != ColumnType_INT {
		t.Errorf("expected Type([INT]) to be INT")
	}

	// Test EvalGroup() with integers
	res, err := fn.EvalGroup([][]any{{10}, {5}, {20}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 5 {
		t.Errorf("expected min to be 5, got %v", res)
	}

	// Test EvalGroup() with strings
	res, err = fn.EvalGroup([][]any{{"cherry"}, {"apple"}, {"banana"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "apple" {
		t.Errorf("expected min to be apple, got %v", res)
	}

	// Test EvalGroup() with empty/nil rows
	res, err = fn.EvalGroup([][]any{{}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected min to be nil, got %v", res)
	}
}

func TestMaxFunc(t *testing.T) {
	fn := maxFunc{}

	// Test Type()
	if fn.Type(nil) != ColumnType_STRING {
		t.Errorf("expected Type(nil) to be STRING")
	}
	if fn.Type([]ColumnType{ColumnType_FLOAT}) != ColumnType_FLOAT {
		t.Errorf("expected Type([FLOAT]) to be FLOAT")
	}

	// Test EvalGroup() with integers
	res, err := fn.EvalGroup([][]any{{10}, {5}, {20}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 20 {
		t.Errorf("expected max to be 20, got %v", res)
	}

	// Test EvalGroup() with strings
	res, err = fn.EvalGroup([][]any{{"cherry"}, {"apple"}, {"banana"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "cherry" {
		t.Errorf("expected max to be cherry, got %v", res)
	}

	// Test EvalGroup() with empty/nil rows
	res, err = fn.EvalGroup([][]any{{}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected max to be nil, got %v", res)
	}
}

func TestCountFunc(t *testing.T) {
	fn := countFunc{}

	// Test Type()
	if fn.Type(nil) != ColumnType_INT {
		t.Errorf("expected Type(nil) to be INT")
	}

	// Test EvalGroup() count(*) (empty args list inner element)
	res, err := fn.EvalGroup([][]any{{}, {}, {}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 3 {
		t.Errorf("expected count(*) to be 3, got %v", res)
	}

	// Test EvalGroup() count(col) with nils
	res, err = fn.EvalGroup([][]any{{10}, {nil}, {20}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 2 {
		t.Errorf("expected count(col) to be 2, got %v", res)
	}

	// Test EvalGroup() with empty input slice
	res, err = fn.EvalGroup(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 0 {
		t.Errorf("expected count(nil) to be 0, got %v", res)
	}
}

func TestSumFunc(t *testing.T) {
	fn := sumFunc{}

	// Test Type()
	if fn.Type(nil) != ColumnType_INT {
		t.Errorf("expected Type(nil) to be INT")
	}
	if fn.Type([]ColumnType{ColumnType_FLOAT}) != ColumnType_FLOAT {
		t.Errorf("expected Type([FLOAT]) to be FLOAT")
	}

	// Test EvalGroup() with only integers
	res, err := fn.EvalGroup([][]any{{10}, {5}, {20}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 35 {
		t.Errorf("expected sum to be 35, got %v", res)
	}

	// Test EvalGroup() with mixed types (int and float64)
	res, err = fn.EvalGroup([][]any{{10}, {5.5}, {20}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 35.5 {
		t.Errorf("expected sum to be 35.5, got %v", res)
	}

	// Test EvalGroup() with float first
	res, err = fn.EvalGroup([][]any{{5.5}, {10}, {20}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 35.5 {
		t.Errorf("expected sum to be 35.5, got %v", res)
	}

	// Test EvalGroup() with no non-nil values
	res, err = fn.EvalGroup([][]any{{}, {nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected sum to be nil, got %v", res)
	}
}
