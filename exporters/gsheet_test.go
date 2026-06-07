package exporters_test

import (
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tbutter/qit/exporters"
	"github.com/tbutter/qit/nodes"
	"github.com/tbutter/qit/sources"
)

type mockMemoryNode struct {
	types []nodes.ColumnDef
	rows  []nodes.Row
}

func (m *mockMemoryNode) Types() []nodes.ColumnDef {
	return m.types
}

func (m *mockMemoryNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		for _, row := range m.rows {
			if !yield(row) {
				return
			}
		}
	}
}

func TestExportGSheet(t *testing.T) {
	var cleared, updated bool
	var receivedValues [][]any

	// 1. Mock Google Sheets API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Verify Auth header
		if r.Header.Get("Authorization") != "Bearer mock_access_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.Method == "POST" && r.URL.Path == "/v4/spreadsheets/mock_spreadsheet_id/values/mock_sheet_name:clear" {
			cleared = true
			_, _ = w.Write([]byte(`{}`))
			return
		}

		if r.Method == "PUT" && r.URL.Path == "/v4/spreadsheets/mock_spreadsheet_id/values/mock_sheet_name" {
			if r.URL.Query().Get("valueInputOption") != "USER_ENTERED" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "invalid valueInputOption"}`))
				return
			}
			updated = true

			var body struct {
				Values [][]any `json:"values"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			receivedValues = body.Values
			_, _ = w.Write([]byte(`{}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 2. Override real Google Sheets API Host
	oldSheetsAPIHost := sources.GoogleSheetsAPIHost
	sources.GoogleSheetsAPIHost = server.URL
	defer func() {
		sources.GoogleSheetsAPIHost = oldSheetsAPIHost
	}()

	// 3. Setup mock environment for client credentials
	os.Setenv("GOOGLE_CLIENT_ID", "mock_client_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "mock_client_secret")
	defer func() {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
	}()

	// 4. Override token path with valid cached token
	tempTokenFile := filepath.Join(t.TempDir(), "gsheet_token.json")
	// Write dummy cached token that is not expired
	tokenData := `{
		"access_token": "mock_access_token",
		"refresh_token": "mock_refresh_token",
		"expiry": "2099-06-07T13:00:00Z"
	}`
	if err := os.WriteFile(tempTokenFile, []byte(tokenData), 0600); err != nil {
		t.Fatalf("failed to write temp token file: %v", err)
	}

	sources.TokenFilePathOverride = tempTokenFile
	defer func() {
		sources.TokenFilePathOverride = ""
	}()

	// 5. Construct a mock node to export
	nodeTypes := []nodes.ColumnDef{
		{Name: "id", Type: nodes.ColumnType_INT},
		{Name: "name", Type: nodes.ColumnType_STRING},
	}
	nodeRows := []nodes.Row{
		{Value: []any{1, "Widget"}},
		{Value: []any{2, "Gizmo"}},
	}
	memNode := &mockMemoryNode{
		types: nodeTypes,
		rows:  nodeRows,
	}
	node := nodes.NewNode(memNode)

	// 6. Execute export
	err := exporters.ExportGSheet("mock_spreadsheet_id", "mock_sheet_name", node)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	if !cleared {
		t.Error("expected clear endpoint to be called")
	}
	if !updated {
		t.Error("expected update endpoint to be called")
	}

	// Verify headers and rows in received values
	if len(receivedValues) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 data), got %d", len(receivedValues))
	}

	// Header checks
	if receivedValues[0][0] != "id" || receivedValues[0][1] != "name" {
		t.Errorf("unexpected headers: %v", receivedValues[0])
	}

	// Data checks
	if receivedValues[1][0].(float64) != 1 || receivedValues[1][1] != "Widget" {
		t.Errorf("unexpected first data row: %v", receivedValues[1])
	}
	if receivedValues[2][0].(float64) != 2 || receivedValues[2][1] != "Gizmo" {
		t.Errorf("unexpected second data row: %v", receivedValues[2])
	}
}
