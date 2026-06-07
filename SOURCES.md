# Supported Table Sources

This document describes the data sources available in the query execution engine.

---

## 1. Fake Dataset Source (`FakeNode`)
The fake data source generates a synthetic, in-memory dataset of exactly 100 rows containing randomized values.

* **Usage**: Acts as the default fallback data source if no specific table function is named in the `FROM` clause.
* **Schema**:
  | Column Name | Type | Description |
  |---|---|---|
  | `a` | `INT` | A random integer between `1` and `10` inclusive. |
  | `b` | `INT` | A random integer between `100` and `999` inclusive. |
  | `c` | `STRING` | A random word chosen from: *apple*, *banana*, *cherry*, *date*, *elderberry*, *fig*, *grape*, *honeydew*, *kiwi*, *lemon*. |
* **Example**:
  ```sql
  SELECT a, b, c FROM x WHERE a = 5
  ```

---

## 2. CSV File Source (`CSVNode`)
The CSV source reads a delimited text file on the local filesystem and streams its contents.

* **Usage**: Specified using the `csv("path/to/file.csv")` table function in the `FROM` clause.
* **Header Handling**: The first line of the CSV file is parsed and used as the column names (headers). Spaces are trimmed.
* **Schema & Type Inference**:
  The schema is resolved dynamically. The first data row (the second line of the CSV) is examined to infer column types:
  * If a cell value parses as an integer (`strconv.Atoi`), the column type is set to `ColumnType_INT`.
  * If it fails to parse as integer but parses as a float (`strconv.ParseFloat`), the column type is set to `ColumnType_FLOAT`.
  * If it parses as a date (`time.Time` matching layouts like `"2006-01-02"`, `"2006-01-02 15:04:05"`, or RFC 3339), the column type is set to `ColumnType_DATE`.
  * Otherwise, the column type defaults to `ColumnType_STRING`.
* **Value Casting**:
  During iteration, values in each row are parsed into their respective Go types (`int`, `float64`, `time.Time`, `string`) based on the inferred column type. If parsing fails for a specific cell, it falls back to the original string value.
* **Example**:
  ```sql
  SELECT name, score, created_at FROM csv("data.csv") WHERE score > 85.5
  ```

---

## 3. SQLite Source (`SQLiteNode`)
The SQLite source opens a local SQLite database and streams rows from a specified table.

* **Usage**: Specified using the `sqlite("path/to/db.sqlite", "table_name")` table function in the `FROM` clause.
* **Schema & Type Reflection**:
  The schema is resolved dynamically at instantiation using a `PRAGMA table_info` query. SQLite column types are matched to the query engine's internal `ColumnType` constants:
  * Types containing `INT` are mapped to `ColumnType_INT`.
  * Types containing `REAL`, `FLOA`, or `DOUB` are mapped to `ColumnType_FLOAT`.
  * Types containing `DATE` or `TIME` are mapped to `ColumnType_DATE`.
  * Types containing `CHAR`, `TEXT`, or `CLOB` (or any other types) default to `ColumnType_STRING`.
* **Value Coercion**:
  During iteration, database values are scanned and coerced to match the inferred `ColumnType`:
  * **INT**: Integers (`int`/`int64`/`float64`) and numeric strings are converted/coerced to Go `int`.
  * **FLOAT**: Float and integer values are converted to Go `float64`.
  * **DATE**: Date/time strings, epoch timestamps, or `time.Time` values are coerced to Go `time.Time`.
  * **STRING**: Values are formatted or parsed to Go `string`.
* **Example**:
  ```sql
  SELECT a, b FROM sqlite("file.sqlite", "tablename")
  ```

---

## 4. JSON Lines File Source (`JSONLNode`)
The JSONL source reads a newline-delimited JSON (JSON Lines) file on the local filesystem and streams its contents.

* **Usage**: Specified using the `jsonl("path/to/file.jsonl")` table function in the `FROM` clause.
* **Schema & Type Inference**:
  The schema is resolved dynamically from the keys of the first non-empty JSON object parsed in the file. Column names are sorted alphabetically for deterministic order. Column types are inferred from their JSON values:
  * **Boolean**: Mapped to `ColumnType_INT` (evaluates to `1` for `true`, `0` for `false`).
  * **Number**: Mapped to `ColumnType_INT` if it can be represented as an integer without loss of precision, otherwise `ColumnType_FLOAT`.
  * **String**: Evaluated as `ColumnType_DATE` if it parses as a date, otherwise `ColumnType_STRING`.
  * **Object**: Mapped to `ColumnType_COMPLEX`.
  * **Array / Null / Other**: Mapped to `ColumnType_STRING`.
* **Value Parsing**:
  During iteration, each line is parsed as a JSON object. Values for the keys identified in the schema are coerced into the column's Go type. If a key is missing from a line, `nil` is returned.
* **Example**:
  ```sql
  SELECT id, user.name, created_at FROM jsonl("data.jsonl")
  ```

---

## 5. Google Sheets Source (`GSheetNode`)
The Google Sheets source fetches a sheet from a Google Spreadsheet using OAuth2 authentication.

* **Usage**: Specified using the `gsheet("url_or_id", "sheet_name")` table function in the `FROM` clause. The sheet name argument is optional; if omitted, the first sheet in the spreadsheet is automatically fetched.
* **Authentication**:
  The engine uses OAuth2 with PKCE (Proof Key for Code Exchange) to authenticate. This avoids the need for storing a client secret in the code or prompting for it. It uses a Google OAuth Client ID:
  1. Checks the environment variable `GOOGLE_CLIENT_ID`.
  2. Reads from `credentials.json` (extracting `client_id`) in the current directory if it exists.
  3. Falls back to a built-in default Google Client ID.
  
  During the initial authorization, it generates a cryptographically random code verifier, builds an authorization URL with the challenge, and prompts the user to input the authorization code from their browser. The code is exchanged for access/refresh tokens which are cached in `.qit_token.json` for subsequent runs.
* **Schema & Type Inference**:
  Like the CSV source, the first row is used as column headers. The first data row (second row of the sheet) is parsed to infer column types (`INT`, `FLOAT`, `DATE`, `STRING`).
* **Example**:
  ```sql
  SELECT name, score FROM gsheet("https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKv1HB3K1w19Zz30OD1gUXU27vI/edit", "Sheet1")
  ```
