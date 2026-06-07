package sources

import (
	"path/filepath"
	"strings"

	"github.com/tbutter/qit/nodes"
	sqlp "github.com/rqlite/sql"
)

func init() {
	nodes.RegisterSourceFactory(createSource, NewFakeNode)
}

// createSource handles the dispatch of table function names to source node constructors.
func createSource(name string, args []sqlp.Expr) (*nodes.Node, string, error) {
	if strings.EqualFold(name, "csv") && len(args) > 0 {
		var filename string
		switch arg := args[0].(type) {
		case *sqlp.StringLit:
			filename = arg.Value
		case *sqlp.Ident:
			filename = arg.Name
		}
		if filename != "" {
			node, err := NewCSVNode(filename)
			if err != nil {
				return nil, "", err
			}
			base := filepath.Base(filename)
			if ext := filepath.Ext(base); ext != "" {
				base = strings.TrimSuffix(base, ext)
			}
			return node, base, nil
		}
	} else if strings.EqualFold(name, "jsonl") && len(args) > 0 {
		var filename string
		switch arg := args[0].(type) {
		case *sqlp.StringLit:
			filename = arg.Value
		case *sqlp.Ident:
			filename = arg.Name
		}
		if filename != "" {
			node, err := NewJSONLNode(filename)
			if err != nil {
				return nil, "", err
			}
			base := filepath.Base(filename)
			if ext := filepath.Ext(base); ext != "" {
				base = strings.TrimSuffix(base, ext)
			}
			return node, base, nil
		}
	} else if strings.EqualFold(name, "sqlite") && len(args) > 1 {
		var dbPath string
		switch arg := args[0].(type) {
		case *sqlp.StringLit:
			dbPath = arg.Value
		case *sqlp.Ident:
			dbPath = arg.Name
		}
		var tableName string
		switch arg := args[1].(type) {
		case *sqlp.StringLit:
			tableName = arg.Value
		case *sqlp.Ident:
			tableName = arg.Name
		}
		if dbPath != "" && tableName != "" {
			node, err := NewSQLiteNode(dbPath, tableName)
			if err != nil {
				return nil, "", err
			}
			return node, tableName, nil
		}
	} else if strings.EqualFold(name, "gsheet") && len(args) > 0 {
		var sheetURL string
		switch arg := args[0].(type) {
		case *sqlp.StringLit:
			sheetURL = arg.Value
		case *sqlp.Ident:
			sheetURL = arg.Name
		}
		var sheetName string
		if len(args) > 1 {
			switch arg := args[1].(type) {
			case *sqlp.StringLit:
				sheetName = arg.Value
			case *sqlp.Ident:
				sheetName = arg.Name
			}
		}
		if sheetURL != "" {
			node, err := NewGSheetNode(sheetURL, sheetName)
			if err != nil {
				return nil, "", err
			}
			return node, "gsheet", nil
		}
	} else {
		// Unknown table function — use fake node
		node := NewFakeNode()
		return node, name, nil
	}

	return nil, "", nil
}
