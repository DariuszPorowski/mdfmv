package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeBaseURIHelpers(t *testing.T) {
	assert.True(t, strings.HasPrefix(computeInlineBaseURI(), "inline://"))
	assert.True(t, strings.HasPrefix(computeBaseURIFrom(""), "inline://"))
	assert.Equal(t, "https://example.com/s", computeBaseURIFrom("https://example.com/s"))
	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.json")
	_ = os.WriteFile(f, []byte("{}"), 0o600)
	got := computeBaseURIFrom(f)
	assert.True(t, strings.HasPrefix(got, "file://"))
}

func TestValidateContent_NoSchema_NoForce_Warn(t *testing.T) {
	ctx := t.Context()
	content := "---\ntitle: ok\n---\nBody\n"
	r := ValidateContent(ctx, content, "", "", false)
	assert.True(t, r.Valid, "expected valid with warning")
	assert.Contains(t, r.Issues, WarnNoSchema)
}

func TestValidateContent_NoFrontMatter_Force_Fail(t *testing.T) {
	ctx := t.Context()
	content := "# No front matter\n"
	r := ValidateContent(ctx, content, "", "", true)
	assert.False(t, r.Valid)
	assert.Contains(t, r.Issues, WarnMissingFront)
}

func TestValidateContent_NoFrontMatter_NoForce_Warn(t *testing.T) {
	ctx := t.Context()
	content := "# No front matter\n"
	r := ValidateContent(ctx, content, "", "", false)
	assert.True(t, r.Valid)
	assert.Contains(t, r.Issues, WarnNoFrontMatter)
}

func TestValidateContent_UnclosedFence_NoForce_Warn(t *testing.T) {
	ctx := t.Context()
	content := "---\ntitle: x\n" // missing closing fence
	r := ValidateContent(ctx, content, "", "", false)
	assert.True(t, r.Valid, "expected valid (warning) when fence unclosed without force")
	assert.Contains(t, r.Issues, ErrUnclosedFrontMatter.Error())
}

func TestValidateContent_UnclosedFence_Force_Fail(t *testing.T) {
	ctx := t.Context()
	content := "---\ntitle: x\n" // missing closing fence
	r := ValidateContent(ctx, content, "", "", true)
	assert.False(t, r.Valid, "expected failure when fence unclosed with force")
	assert.Contains(t, r.Issues, ErrUnclosedFrontMatter.Error())
}
