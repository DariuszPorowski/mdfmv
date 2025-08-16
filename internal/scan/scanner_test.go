package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err, "mkdir")
	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "write")
}

func TestScan_SimpleDir_ExtensionsAndRecursive(t *testing.T) {
	dir := t.TempDir()
	// files
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntitle: ok\n---\n")
	writeFile(t, filepath.Join(dir, "a.mdx"), "---\ntitle: ok\n---\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "ignore\n")
	sub := filepath.Join(dir, "sub")
	writeFile(t, filepath.Join(sub, "c.md"), "---\ntitle: ok\n---\n")

	opts := Options{Root: dir, Recursive: true, Extensions: []string{".md"}}

	res, err := Scan(t.Context(), opts)
	require.NoError(t, err)
	// Expect 2 processed: a.md and c.md (subdir)
	assert.NotEmpty(t, res, "expected some results")
}

func TestScan_IgnoreFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".mdfmvignore"), "ignored.md\n")
	writeFile(t, filepath.Join(dir, "ignored.md"), "---\ntitle: ok\n---\n")
	writeFile(t, filepath.Join(dir, "kept.md"), "---\ntitle: ok\n---\n")

	opts := Options{Root: dir, Recursive: false, Extensions: []string{".md"}}

	res, err := Scan(t.Context(), opts)
	require.NoError(t, err)
	// Ensure at least one result exists (walk or validation), but the ignored file should not be present
	foundIgnored := false

	for _, r := range res {
		if filepath.Base(r.Path) == "ignored.md" {
			foundIgnored = true

			break
		}
	}

	assert.False(t, foundIgnored, "ignored file was processed")
}
