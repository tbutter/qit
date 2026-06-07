# Supported SQL Functions and Expressions

This document lists all built-in scalar, aggregate, table functions, and expressions supported by the query execution engine.

---

## 1. Scalar Functions
Scalar functions take column values from a single row and compute a value for that row.

### `concat(expr1, expr2, ...)`
* **Description**: Concatenates the string representations of all non-null argument values.
* **Return Type**: `STRING`
* **Example**:
  ```sql
  SELECT concat(c, '-', a) FROM csv("file.csv")
  ```

---

## 2. Aggregate (Grouping) Functions
Aggregate functions compute a single value over a group of rows (either grouped via `GROUP BY` or aggregated over the entire dataset).

### `count(expr)` / `count(*)`
* **Description**: Returns the count of non-null values of `expr` within the group. When used with the `*` wildcard (i.e. `count(*)`), it returns the total number of rows in the group (including nulls).
* **Return Type**: `INT`
* **Example**:
  ```sql
  SELECT count(b), count(*) FROM csv("file.csv") GROUP BY a
  ```

### `sum(expr)`
* **Description**: Computes the sum of all numeric values of `expr` within the group. Returns `NULL` if the group is empty or contains no non-null numeric values.
* **Return Type**: `INT` (if all values are integers) or `FLOAT` (if any value is float).
* **Example**:
  ```sql
  SELECT sum(b) FROM csv("file.csv") GROUP BY a
  ```

### `min(expr)`
* **Description**: Finds the minimum evaluated value of `expr` within the group. Supports numeric and string comparison.
* **Return Type**: Matches the type of the argument `expr`.
* **Example**:
  ```sql
  SELECT min(b) FROM csv("file.csv") GROUP BY a
  ```

### `max(expr)`
* **Description**: Finds the maximum evaluated value of `expr` within the group. Supports numeric and string comparison.
* **Return Type**: Matches the type of the argument `expr`.
* **Example**:
  ```sql
  SELECT max(b) FROM csv("file.csv") GROUP BY a
  ```

---

## 3. Table Functions
Table functions act as data sources in the `FROM` clause of a query.

### `csv("filename.csv")`
* **Description**: Reads rows from a CSV file using the first line as column headers. It automatically infers column types (`INT`, `FLOAT`, `DATE`, `STRING`) based on the values in the first data row.
* **Example**:
  ```sql
  SELECT a, b, c FROM csv("data.csv")
  ```

### `jsonl("filename.jsonl")`
* **Description**: Reads rows from a JSON Lines (JSONL) file where each line is a JSON object. Keys in the first object define the columns (sorted alphabetically for deterministic order) and their types are inferred based on the first object's values (with nested objects mapped to `COMPLEX`). Missing keys in subsequent lines resolve to `NULL`.
* **Example**:
  ```sql
  SELECT a, b, e.nested FROM jsonl("data.jsonl")
  ```

---

## 4. Casting Expressions
Casting expressions convert values from one type to another.

### `CAST(expr AS type)`
* **Description**: Converts the evaluated value of `expr` to the target `type`. Supported target types include:
  * `INT` / `INTEGER`
  * `FLOAT` / `REAL` / `DOUBLE`
  * `STRING` / `VARCHAR` / `TEXT`
  * `DATE` / `DATETIME` / `TIMESTAMP`
* **Casting Semantics**:
  * **To `INT`**: Truncates floats to integer, parses strings containing numbers, or converts dates to Unix timestamp integers.
  * **To `FLOAT`**: Promotes integers to float, parses strings containing decimals, or converts dates to Unix timestamp floats.
  * **To `STRING`**: Formats integers/floats as decimal strings, or formats dates to ISO-8601 string representation (`"2006-01-02 15:04:05"`).
  * **To `DATE`**: Converts Unix timestamps (int or float) to UTC dates, or parses strings using standard ISO layout.
  * **To `COMPLEX`**: Parses a JSON string into a `map[string]any`, or passes through an existing map.
* **Example**:
  ```sql
  SELECT CAST(a AS FLOAT), CAST(b AS VARCHAR), CAST(c AS DATE) FROM csv("data.csv")
  ```

---

## 5. Column Types

The engine supports the following column types:

| Type | ID | Go Representation | Description |
|---|---|---|---|
| `INT` | 1 | `int` | Integer values |
| `STRING` | 2 | `string` | Text values |
| `FLOAT` | 3 | `float64` | Floating-point values |
| `DATE` | 4 | `time.Time` | Date/time values |
| `COMPLEX` | 5 | `map[string]any` | Nested key-value maps accessed via dot notation |

### COMPLEX Type

The `COMPLEX` type represents a nested map (`map[string]any`) where each key is a string and values can be of any type — including another `COMPLEX` map for nested structures.

* **Dot-Notation Access**: Fields inside a `COMPLEX` column are accessed using SQL dot notation:
  * **`f.k`** — accesses key `k` from `COMPLEX` column `f`
  * **`t.f.k`** — accesses key `k` from `COMPLEX` column `f` on table alias `t`
* **Type Inference**: Accessing a key inside a `COMPLEX` column always returns `STRING` type at the schema level (since the actual runtime type of each map value is dynamic).
* **Sources**: `COMPLEX` values can come from:
  * **CSV**: JSON-encoded strings in a column are automatically deserialized into maps.
  * **JSONL**: Fields containing nested JSON objects are parsed into `COMPLEX` columns.
  * **SQLite**: JSON text columns typed as `COMPLEX` are deserialized during scanning.
  * **FakeNode**: The built-in fake data source includes a `d` column of type `COMPLEX`.
* **Example**:
  ```sql
  -- Access nested map keys from the fake data source
  SELECT a, d.k1, d.k2 FROM x WHERE a = 5

  -- With table alias
  SELECT t.d.k1 FROM x AS t
  ```
