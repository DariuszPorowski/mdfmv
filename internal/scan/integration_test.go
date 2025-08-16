package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test directory-level behaviors including:
// - empty front matter block treated as absent
// - unclosed front matter fence warning vs failure
// - file without front matter but global schema path present.
func TestIntegration_DirectoryScan_WarningsAndFailures(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	// Files:
	// a.md : empty front matter block (--- then ---) should be OK + WarnNoFrontMatter
	// b.md : unclosed fence, no force -> OK + ErrUnclosedFrontMatter
	// c.md : normal front matter referencing schema, minimal data passes
	// d.md : no front matter at all, schema flag provided -> OK + WarnNoFrontMatter
	// schema.json: used by c.md

	schema := `{"type":"object","required":["title"],"properties":{"title":{"type":"string"}}}`
	write := func(name, content string) {
		p := filepath.Join(root, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	write("schema.json", schema)
	write("a.md", "---\n---\n# A\n")
	write("b.md", "---\nkey: value\n") // unclosed
	write("c.md", "---\n$schema: ./schema.json\ntitle: ok\n---\nBody\n")
	write("d.md", "# No front matter here\n")

	opts := Options{Root: root, Recursive: false, Extensions: []string{".md"}, SchemaPath: "", Force: false}
	results, err := Scan(ctx, opts)
	require.NoError(t, err)

	// Map results by filename
	resMap := map[string]Result{}
	for _, r := range results {
		resMap[filepath.Base(r.Path)] = r
	}

	if r, ok := resMap["a.md"]; assert.True(t, ok) {
		assert.True(t, r.Valid)
		assert.Contains(t, r.Issues, WarnNoFrontMatter)
	}

	if r, ok := resMap["b.md"]; assert.True(t, ok) {
		assert.True(t, r.Valid)
		assert.Contains(t, r.Issues, ErrUnclosedFrontMatter.Error())
	}

	if r, ok := resMap["c.md"]; assert.True(t, ok) {
		assert.True(t, r.Valid)
		assert.Empty(t, r.Issues)
	}

	if r, ok := resMap["d.md"]; assert.True(t, ok) {
		assert.True(t, r.Valid)
		assert.Contains(t, r.Issues, WarnNoFrontMatter)
	}
}
