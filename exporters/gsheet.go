package exporters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tbutter/qit/nodes"
	"github.com/tbutter/qit/sources"
)

// ExportGSheet writes the result rows of a Node to a Google Sheet.
// It will overwrite the contents of the specified sheet.
func ExportGSheet(spreadsheetID string, sheetName string, node *nodes.Node) error {
	id, err := sources.ParseSpreadsheetURL(spreadsheetID)
	if err != nil {
		return err
	}
	spreadsheetID = id

	clientID, err := sources.GetClientID()
	if err != nil {
		return err
	}

	accessToken, err := sources.GetAccessToken(clientID)
	if err != nil {
		return err
	}

	// 1. Convert node results to values
	types := node.Types()
	var values [][]any

	// Header row
	header := make([]any, len(types))
	for i, col := range types {
		header[i] = col.Name
	}
	values = append(values, header)

	// Data rows
	for row := range node.All() {
		rowVals := make([]any, len(row.Value))
		for i, val := range row.Value {
			if val == nil {
				rowVals[i] = ""
			} else {
				switch v := val.(type) {
				case map[string]any, []any:
					// Serialize complex/array types to json string for sheet
					if bytes, err := json.Marshal(v); err == nil {
						rowVals[i] = string(bytes)
					} else {
						rowVals[i] = fmt.Sprintf("%v", v)
					}
				default:
					rowVals[i] = v
				}
			}
		}
		values = append(values, rowVals)
	}

	// 2. Resolve sheetName
	writeRange := sheetName
	if writeRange == "" {
		var err error
		writeRange, err = sources.GetFirstSheetName(spreadsheetID, accessToken)
		if err != nil {
			return fmt.Errorf("failed to get first sheet name: %w", err)
		}
	}

	// 3. Clear the sheet first
	clearURL := fmt.Sprintf("%s/v4/spreadsheets/%s/values/%s:clear", sources.GoogleSheetsAPIHost, spreadsheetID, url.QueryEscape(writeRange))
	clearReq, err := http.NewRequest("POST", clearURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create clear request: %w", err)
	}
	clearReq.Header.Set("Authorization", "Bearer "+accessToken)

	clearResp, err := http.DefaultClient.Do(clearReq)
	if err == nil {
		clearResp.Body.Close()
	}

	// 4. Update request
	updateData := struct {
		Values [][]any `json:"values"`
	}{
		Values: values,
	}
	bodyBytes, err := json.Marshal(updateData)
	if err != nil {
		return fmt.Errorf("failed to marshal sheet values: %w", err)
	}

	updateURL := fmt.Sprintf("%s/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED", sources.GoogleSheetsAPIHost, spreadsheetID, url.QueryEscape(writeRange))
	req, err := http.NewRequest("PUT", updateURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update sheet (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
