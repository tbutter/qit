package sources

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tbutter/qit/nodes"
)

func TestParseSpreadsheetURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
		hasError bool
	}{
		{
			url:      "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKv1HB3K1w19Zz30OD1gUXU27vI/edit#gid=0",
			expected: "1BxiMVs0XRA5nFMdKv1HB3K1w19Zz30OD1gUXU27vI",
			hasError: false,
		},
		{
			url:      "1BxiMVs0XRA5nFMdKv1HB3K1w19Zz30OD1gUXU27vI",
			expected: "1BxiMVs0XRA5nFMdKv1HB3K1w19Zz30OD1gUXU27vI",
			hasError: false,
		},
		{
			url:      "https://docs.google.com/spreadsheets/invalid",
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range tests {
		res, err := ParseSpreadsheetURL(tc.url)
		if tc.hasError {
			if err == nil {
				t.Errorf("expected error for url %q, got nil", tc.url)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for url %q: %v", tc.url, err)
			}
			if res != tc.expected {
				t.Errorf("expected ID %q, got %q", tc.expected, res)
			}
		}
	}
}

func TestGSheetNode(t *testing.T) {
	// 1. Start a mock server for Google APIs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Token exchange or refresh
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.FormValue("client_secret") != "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" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "invalid_client", "error_description": "missing or incorrect client secret"}`))
				return
			}
			if r.FormValue("grant_type") == "authorization_code" {
				if r.FormValue("code_verifier") == "" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "code_verifier required"}`))
					return
				}
				if r.FormValue("code") != "mock_auth_code" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
					return
				}
				_, _ = w.Write([]byte(`{
					"access_token": "mock_access_token",
					"refresh_token": "mock_refresh_token",
					"expires_in": 3600
				}`))
			} else if r.FormValue("grant_type") == "refresh_token" {
				if r.FormValue("refresh_token") != "mock_refresh_token" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
					return
				}
				_, _ = w.Write([]byte(`{
					"access_token": "mock_new_access_token",
					"expires_in": 3600
				}`))
			}
			return
		}

		// Spreadsheet metadata (fetch first sheet name)
		if r.URL.Path == "/v4/spreadsheets/my_sheet_id" && r.URL.RawQuery == "fields=sheets.properties.title" {
			if r.Header.Get("Authorization") != "Bearer mock_access_token" && r.Header.Get("Authorization") != "Bearer mock_new_access_token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{
				"sheets": [
					{
						"properties": {
							"title": "FirstSheet"
						}
					}
				]
			}`))
			return
		}

		// Spreadsheet values fetch
		if r.URL.Path == "/v4/spreadsheets/my_sheet_id/values/FirstSheet" {
			if r.Header.Get("Authorization") != "Bearer mock_access_token" && r.Header.Get("Authorization") != "Bearer mock_new_access_token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{
				"values": [
					["id", "name", "price", "joined_on"],
					[1, "Widget", 19.99, "2026-06-01"],
					[2, "Gizmo", 5.5, "2026-06-02"]
				]
			}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 2. Override real Google endpoints
	oldTokenURL := googleTokenURL
	oldSheetsAPIHost := GoogleSheetsAPIHost
	googleTokenURL = server.URL + "/token"
	GoogleSheetsAPIHost = server.URL
	defer func() {
		googleTokenURL = oldTokenURL
		GoogleSheetsAPIHost = oldSheetsAPIHost
	}()

	// 3. Setup client credentials in environment to skip client prompt
	os.Setenv("GOOGLE_CLIENT_ID", "mock_client_id")
	defer func() {
		os.Unsetenv("GOOGLE_CLIENT_ID")
	}()

	// Setup TokenFilePathOverride using a temp dir
	tokenFile := filepath.Join(t.TempDir(), "gsheet_token.json")
	TokenFilePathOverride = tokenFile
	defer func() { TokenFilePathOverride = "" }()

	// 4. Mock user entering authorization code via stdin
	oldStdin := os.Stdin
	rStdin, wStdin, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = rStdin
	defer func() { os.Stdin = oldStdin }()

	// Write the code to stdin pipe
	_, _ = wStdin.WriteString("mock_auth_code\n")
	_ = wStdin.Close()

	// 5. Test GSheet source node creation
	node, err := NewGSheetNode("my_sheet_id", "")
	if err != nil {
		t.Fatalf("unexpected error creating GSheet node: %v", err)
	}

	types := node.Types()
	if len(types) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(types))
	}

	expectedTypes := map[string]nodes.ColumnType{
		"id":        nodes.ColumnType_INT,
		"name":      nodes.ColumnType_STRING,
		"price":     nodes.ColumnType_FLOAT,
		"joined_on": nodes.ColumnType_DATE,
	}

	for _, col := range types {
		expType, exists := expectedTypes[col.Name]
		if !exists {
			t.Errorf("unexpected column name: %s", col.Name)
		} else if col.Type != expType {
			t.Errorf("expected column %s to be of type %v, got %v", col.Name, expType, col.Type)
		}
	}

	// Read rows
	var rows [][]any
	for row := range node.All() {
		rows = append(rows, row.Value)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0][0].(int) != 1 || rows[0][1].(string) != "Widget" || rows[0][2].(float64) != 19.99 {
		t.Errorf("unexpected row 0: %+v", rows[0])
	}
	if rows[1][0].(int) != 2 || rows[1][1].(string) != "Gizmo" || rows[1][2].(float64) != 5.5 {
		t.Errorf("unexpected row 1: %+v", rows[1])
	}

	// 6. Test Token Refresh Flow
	// Manually corrupt cached token expiry to force a refresh
	var cachedToken struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("token file was not cached: %v", err)
	}
	_ = json.Unmarshal(data, &cachedToken)
	cachedToken.Expiry = time.Now().Add(-1 * time.Hour) // expired
	corruptedData, _ := json.Marshal(cachedToken)
	_ = os.WriteFile(tokenFile, corruptedData, 0600)

	// Since token is expired, creating a new node should trigger refresh, which uses mock_new_access_token.
	// No stdin input is needed for refresh!
	node2, err := NewGSheetNode("my_sheet_id", "FirstSheet")
	if err != nil {
		t.Fatalf("unexpected error creating GSheet node with expired token: %v", err)
	}

	rows2 := 0
	for range node2.All() {
		rows2++
	}
	if rows2 != 2 {
		t.Errorf("expected 2 rows on refreshed token run, got %d", rows2)
	}
}
