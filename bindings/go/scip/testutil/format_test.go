package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scip-code/scip/bindings/go/scip"
)

func rangeAt(startLine, startChar, endLine, endChar int32) scip.Range {
	return scip.Range{
		Start: scip.Position{Line: startLine, Character: startChar},
		End:   scip.Position{Line: endLine, Character: endChar},
	}
}

// TestFormatSnapshotFeatures exercises enclosing_symbol, multiline occurrences
// and synthetic_definition rendering, plus the zero-width range edge case.
func TestFormatSnapshotFeatures(t *testing.T) {
	source := "package foo\nfunc bar() {\n  return\n}\n"
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0644))

	document := &scip.Document{
		RelativePath: "test.go",
		Occurrences: []*scip.Occurrence{
			{
				Symbol:      "local foo",
				SymbolRoles: int32(scip.SymbolRole_Definition),
				TypedRange:  rangeAt(0, 8, 0, 11).AsTypedRange(),
			},
			{
				// Multiline reference spanning lines 1..3.
				Symbol:     "local bar",
				TypedRange: rangeAt(1, 5, 3, 1).AsTypedRange(),
			},
			{
				// Zero-width definition: still renders a single marker.
				Symbol:      "local zero",
				SymbolRoles: int32(scip.SymbolRole_Definition),
				TypedRange:  rangeAt(2, 2, 2, 2).AsTypedRange(),
			},
		},
		Symbols: []*scip.SymbolInformation{
			{
				Symbol:          "local foo",
				Kind:            scip.SymbolInformation_Namespace,
				DisplayName:     "foo",
				EnclosingSymbol: "local root",
			},
			{
				// Defined at "local foo"'s definition site; rendered as a
				// synthetic_definition under it.
				Symbol:        "local child",
				Kind:          scip.SymbolInformation_Field,
				DisplayName:   "child",
				Relationships: []*scip.Relationship{{Symbol: "local foo", IsDefinition: true}},
			},
		},
	}

	snapshot, err := FormatSnapshot(document, "//", scip.DescriptorOnlyFormatter, sourcePath)
	require.NoError(t, err)

	// enclosing_symbol is rendered through the symbol formatter.
	require.Contains(t, snapshot, "//        ^^^ definition local foo")
	require.Contains(t, snapshot, "enclosing_symbol local root")

	// Synthetic definition: distinct child symbol, underscore markers, and the
	// is_definition relationship rendered as "definition".
	require.Contains(t, snapshot, "//        ___ synthetic_definition local child")
	require.Contains(t, snapshot, "relationship local foo definition")

	// Multiline occurrence: markers extend to end of the start line and a
	// "<lineDelta>:<endCharacter>" suffix reports the end position.
	require.Contains(t, snapshot, "//     ^^^^^^^ reference local bar 2:1")

	// Zero-width range still renders exactly one marker.
	require.Contains(t, snapshot, "//  ^ definition local zero")
}

// TestFormatSnapshotTestRole checks that occurrences with SymbolRole.Test get
// a `test` modifier after their role, and that other role bits (ReadAccess
// here) and synthetic definitions are rendered as before.
func TestFormatSnapshotTestRole(t *testing.T) {
	source := "func add() {}\nfunc TestAdd() { add() }\nvar _ = add\nfunc helper()\n"
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "add_test.go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0644))

	document := &scip.Document{
		RelativePath: "add_test.go",
		Occurrences: []*scip.Occurrence{
			{
				Symbol:      "local add",
				SymbolRoles: int32(scip.SymbolRole_Definition),
				TypedRange:  rangeAt(0, 5, 0, 8).AsTypedRange(),
			},
			{
				Symbol:      "local TestAdd",
				SymbolRoles: int32(scip.SymbolRole_Definition | scip.SymbolRole_Test),
				TypedRange:  rangeAt(1, 5, 1, 12).AsTypedRange(),
			},
			{
				Symbol:      "local add",
				SymbolRoles: int32(scip.SymbolRole_ReadAccess | scip.SymbolRole_Test),
				TypedRange:  rangeAt(1, 17, 1, 20).AsTypedRange(),
			},
			{
				Symbol:      "local add",
				SymbolRoles: int32(scip.SymbolRole_ReadAccess),
				TypedRange:  rangeAt(2, 8, 2, 11).AsTypedRange(),
			},
			{
				Symbol:      "local helper",
				SymbolRoles: int32(scip.SymbolRole_ForwardDefinition | scip.SymbolRole_Test),
				TypedRange:  rangeAt(3, 5, 3, 11).AsTypedRange(),
			},
		},
		Symbols: []*scip.SymbolInformation{
			{
				// Synthetic definitions do not inherit the Test role of the
				// definition they are rendered under.
				Symbol:        "local child",
				Relationships: []*scip.Relationship{{Symbol: "local TestAdd", IsDefinition: true}},
			},
		},
	}

	snapshot, err := FormatSnapshot(document, "//", scip.DescriptorOnlyFormatter, sourcePath)
	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"  func add() {}",
		"//     ^^^ definition local add",
		"  func TestAdd() { add() }",
		"//     ^^^^^^^ definition test local TestAdd",
		"//     _______ synthetic_definition local child",
		"//             relationship local TestAdd definition",
		"//                 ^^^ reference test local add",
		"  var _ = add",
		"//        ^^^ reference local add",
		"  func helper()",
		"//     ^^^^^^ forward_definition test local helper",
		"  ",
		"",
	}, "\n"), snapshot)
}
