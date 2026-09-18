package backups_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Backup code writes and removes stored files through the file store, which is what
// keeps a failed backup from leaving one behind. A provider's own methods stay
// reachable from here, so a guard keeps them out.
func Test_BackupCode_DoesNotCallProviderSaveOrDeleteDirectly(t *testing.T) {
	var offenders []string

	require.NoError(t, filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			selector, isSelector := call.Fun.(*ast.SelectorExpr)
			if !isSelector {
				return true
			}

			if selector.Sel.Name == "SaveFile" || selector.Sel.Name == "DeleteFile" {
				offenders = append(offenders, path+": "+selector.Sel.Name)
			}

			return true
		})

		return nil
	}))

	assert.Empty(t, offenders,
		"backup code must reach storage through the file store, not through a provider")
}
