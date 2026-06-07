package sources

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tbutter/qit/nodes"
)

var (
	googleTokenURL         = "https://oauth2.googleapis.com/token"
	GoogleSheetsAPIHost    = "https://sheets.googleapis.com"
	googleAuthURLBase      = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenFilePathOverride string
)

// GSheetNode reads and parses rows from a Google Sheet.
type GSheetNode struct {
	headers []string
	types   []nodes.ColumnDef
	rows    [][]string
}

func (g *GSheetNode) Types() []nodes.ColumnDef {
	return g.types
}

func (g *GSheetNode) All() iter.Seq[nodes.Row] {
	return func(yield func(nodes.Row) bool) {
		for _, record := range g.rows {
			rowVals := make([]any, len(g.headers))
			for i, val := range record {
				if i >= len(rowVals) {
					break
				}
				if g.types[i].Type == nodes.ColumnType_INT {
					if v, err := strconv.Atoi(val); err == nil {
						rowVals[i] = v
					} else {
						rowVals[i] = val
					}
				} else if g.types[i].Type == nodes.ColumnType_FLOAT {
					if v, err := strconv.ParseFloat(val, 64); err == nil {
						rowVals[i] = v
					} else {
						rowVals[i] = val
					}
				} else if g.types[i].Type == nodes.ColumnType_DATE {
					if v, err := nodes.ParseDate(val); err == nil {
						rowVals[i] = v
					} else {
						rowVals[i] = val
					}
				} else if g.types[i].Type == nodes.ColumnType_COMPLEX {
					var m map[string]any
					if err := json.Unmarshal([]byte(val), &m); err == nil {
						rowVals[i] = m
					} else {
						rowVals[i] = val
					}
				} else if g.types[i].Type == nodes.ColumnType_ARRAY {
					var arr []any
					if err := json.Unmarshal([]byte(val), &arr); err == nil {
						rowVals[i] = arr
					} else {
						rowVals[i] = val
					}
				} else {
					rowVals[i] = val
				}
			}
			if !yield(nodes.Row{Value: rowVals}) {
				return
			}
		}
	}
}

// NewGSheetNode creates a new Node that reads data from a Google Sheet.
func NewGSheetNode(urlStr string, sheetName string) (*nodes.Node, error) {
	spreadsheetID, err := ParseSpreadsheetURL(urlStr)
	if err != nil {
		return nil, err
	}

	clientID, err := GetClientID()
	if err != nil {
		return nil, err
	}

	accessToken, err := GetAccessToken(clientID)
	if err != nil {
		return nil, err
	}

	if sheetName == "" {
		sheetName, err = GetFirstSheetName(spreadsheetID, accessToken)
		if err != nil {
			return nil, err
		}
	}

	records, err := fetchSheetValues(spreadsheetID, sheetName, accessToken)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("sheet is empty")
	}

	header := records[0]
	var dataRows [][]string
	if len(records) > 1 {
		dataRows = records[1:]
	}

	colTypes := make([]nodes.ColumnType, len(header))
	for i := range colTypes {
		colTypes[i] = nodes.ColumnType_STRING
	}

	if len(dataRows) > 0 {
		firstRow := dataRows[0]
		for i, val := range firstRow {
			if i < len(colTypes) {
				if _, err := strconv.Atoi(val); err == nil {
					colTypes[i] = nodes.ColumnType_INT
				} else if _, err := strconv.ParseFloat(val, 64); err == nil {
					colTypes[i] = nodes.ColumnType_FLOAT
				} else if _, err := nodes.ParseDate(val); err == nil {
					colTypes[i] = nodes.ColumnType_DATE
				}
			}
		}
	}

	defs := make([]nodes.ColumnDef, len(header))
	for i, name := range header {
		defs[i] = nodes.ColumnDef{
			Name: strings.TrimSpace(name),
			Type: colTypes[i],
		}
	}

	return nodes.NewNode(&GSheetNode{
		headers: header,
		types:   defs,
		rows:    dataRows,
	}), nil
}

func ParseSpreadsheetURL(urlStr string) (string, error) {
	// Support both raw spreadsheet ID and full Google Sheets URLs
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		// Treat as direct ID
		return urlStr, nil
	}

	parts := strings.Split(urlStr, "/d/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid Google Sheets URL format: /d/ section not found")
	}
	idPart := strings.Split(parts[1], "/")[0]
	if idPart == "" {
		return "", fmt.Errorf("could not extract spreadsheet ID from URL")
	}
	return idPart, nil
}

func generatePKCEPair() (string, string, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}

const defaultClientID = "575544034189-qp9tqe857rij6ee4mcdsggne3hjesbch.apps.googleusercontent.com"

func GetClientID() (string, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID != "" {
		return clientID, nil
	}

	// Try reading from credentials.json in current workspace directory
	if data, err := os.ReadFile("credentials.json"); err == nil {
		var creds struct {
			Installed struct {
				ClientID string `json:"client_id"`
			} `json:"installed"`
			Web struct {
				ClientID string `json:"client_id"`
			} `json:"web"`
		}
		if err := json.Unmarshal(data, &creds); err == nil {
			if creds.Installed.ClientID != "" {
				return creds.Installed.ClientID, nil
			}
			if creds.Web.ClientID != "" {
				return creds.Web.ClientID, nil
			}
		}
	}

	return defaultClientID, nil
}

func getObscuredClientSecret() string {
	obfuscated := []byte{27, 19, 31, 15, 12, 4, 113, 23, 110, 36, 31, 40, 17, 21, 4, 25, 25, 6, 61, 37, 12, 16, 15, 59, 15, 8, 5, 105, 12, 9, 42, 31, 8, 58, 27}
	key := byte(0x5C)
	decoded := make([]byte, len(obfuscated))
	for i, b := range obfuscated {
		decoded[i] = b ^ key
	}
	return string(decoded)
}

func getClientSecret() (string, error) {
	secret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if secret != "" {
		return secret, nil
	}

	// Try reading from credentials.json in current workspace directory
	if data, err := os.ReadFile("credentials.json"); err == nil {
		var creds struct {
			Installed struct {
				ClientSecret string `json:"client_secret"`
			} `json:"installed"`
			Web struct {
				ClientSecret string `json:"client_secret"`
			} `json:"web"`
		}
		if err := json.Unmarshal(data, &creds); err == nil {
			if creds.Installed.ClientSecret != "" {
				return creds.Installed.ClientSecret, nil
			}
			if creds.Web.ClientSecret != "" {
				return creds.Web.ClientSecret, nil
			}
		}
	}

	return getObscuredClientSecret(), nil
}

func getTokenFilePath() (string, error) {
	if TokenFilePathOverride != "" {
		return TokenFilePathOverride, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configDir, "qit", "gsheet_token.json"), nil
}

func GetAccessToken(clientID string) (string, error) {
	tokenFile, err := getTokenFilePath()
	if err != nil {
		return "", err
	}
	var cachedToken struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}

	if data, err := os.ReadFile(tokenFile); err == nil {
		if err := json.Unmarshal(data, &cachedToken); err == nil {
			if cachedToken.Expiry.After(time.Now().Add(1 * time.Minute)) {
				return cachedToken.AccessToken, nil
			}
			if cachedToken.RefreshToken != "" {
				newToken, err := refreshAccessToken(clientID, cachedToken.RefreshToken)
				if err == nil {
					return newToken, nil
				}
			}
		}
	}

	// Generate PKCE code verifier and code challenge
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE pair: %w", err)
	}

	// Interactive OAuth2 flow
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=urn:ietf:wg:oauth:2.0:oob&response_type=code&scope=https://www.googleapis.com/auth/spreadsheets.readonly&code_challenge=%s&code_challenge_method=S256", googleAuthURLBase, clientID, challenge)
	fmt.Println("\nAuthorize Google Sheets access by opening this URL in your browser:")
	fmt.Println(authURL)
	fmt.Println()
	fmt.Print("Enter the authorization code: ")
	var authCode string
	if _, err := fmt.Scanln(&authCode); err != nil {
		return "", fmt.Errorf("failed to read authorization code: %w", err)
	}
	authCode = strings.TrimSpace(authCode)

	clientSecret, err := getClientSecret()
	if err != nil {
		return "", err
	}

	resp, err := http.PostForm(googleTokenURL, url.Values{
		"code":          {authCode},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code_verifier": {verifier},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	cachedToken.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		cachedToken.RefreshToken = tokenResp.RefreshToken
	}
	cachedToken.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	if err := os.MkdirAll(filepath.Dir(tokenFile), 0700); err == nil {
		if tokenData, err := json.Marshal(cachedToken); err == nil {
			_ = os.WriteFile(tokenFile, tokenData, 0600)
		}
	}

	return cachedToken.AccessToken, nil
}

func refreshAccessToken(clientID, refreshToken string) (string, error) {
	clientSecret, err := getClientSecret()
	if err != nil {
		return "", err
	}

	resp, err := http.PostForm(googleTokenURL, url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	tokenFile, err := getTokenFilePath()
	if err != nil {
		return "", err
	}
	var cachedToken struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}
	if data, err := os.ReadFile(tokenFile); err == nil {
		_ = json.Unmarshal(data, &cachedToken)
	}
	cachedToken.AccessToken = tokenResp.AccessToken
	cachedToken.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	if err := os.MkdirAll(filepath.Dir(tokenFile), 0700); err == nil {
		if tokenData, err := json.Marshal(cachedToken); err == nil {
			_ = os.WriteFile(tokenFile, tokenData, 0600)
		}
	}

	return tokenResp.AccessToken, nil
}

func GetFirstSheetName(spreadsheetID, accessToken string) (string, error) {
	reqURL := fmt.Sprintf("%s/v4/spreadsheets/%s?fields=sheets.properties.title", GoogleSheetsAPIHost, spreadsheetID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get sheet metadata (status %d): %s", resp.StatusCode, string(body))
	}

	var metadata struct {
		Sheets []struct {
			Properties struct {
				Title string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", err
	}

	if len(metadata.Sheets) == 0 {
		return "", fmt.Errorf("no sheets found in spreadsheet")
	}

	return metadata.Sheets[0].Properties.Title, nil
}

func fetchSheetValues(spreadsheetID, sheetName, accessToken string) ([][]string, error) {
	escapedSheet := url.QueryEscape(sheetName)
	reqURL := fmt.Sprintf("%s/v4/spreadsheets/%s/values/%s", GoogleSheetsAPIHost, spreadsheetID, escapedSheet)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch sheet values (status %d): %s", resp.StatusCode, string(body))
	}

	var rawResp struct {
		Values [][]any `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, err
	}

	result := make([][]string, len(rawResp.Values))
	for r, row := range rawResp.Values {
		result[r] = make([]string, len(row))
		for c, cell := range row {
			if cell == nil {
				result[r][c] = ""
			} else {
				result[r][c] = fmt.Sprintf("%v", cell)
			}
		}
	}

	return result, nil
}
