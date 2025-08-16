package scan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontMatter_NoFrontMatter(t *testing.T) {
	content := "# Hello\nBody"
	fm, err := ParseFrontMatter(content)
	require.NoError(t, err)
	assert.False(t, fm.HasFrontMatter, "expected no front matter")
	assert.Equal(t, SchemaTypeNone, fm.DetermineSchemaType())
}

func TestParseFrontMatter_ValidYAML_KeySchema(t *testing.T) {
	content := "---\n$schema: ./schema.json\ntitle: Hello\n---\nBody\n"
	fm, err := ParseFrontMatter(content)
	require.NoError(t, err)
	assert.True(t, fm.HasFrontMatter, "expected front matter present")
	assert.Equal(t, "./schema.json", fm.SchemaRef)
}

func TestParseFrontMatter_InlineSchema(t *testing.T) {
	content := "---\n$schema:\n  $schema: 'https://json-schema.org/draft/2020-12/schema'\n  type: object\n  required: [title]\n  properties:\n    title: { type: string }\n---\nBody\n"
	fm, err := ParseFrontMatter(content)
	require.NoError(t, err)
	require.NotNil(t, fm.InlineSchema)
}

func TestParseFrontMatter_UnclosedFence(t *testing.T) {
	content := "---\ntitle: x\n"
	_, err := ParseFrontMatter(content)
	require.ErrorIs(t, err, ErrUnclosedFrontMatter)
}

func TestParseFrontMatter_EmptyBlockTreatedAsAbsent(t *testing.T) {
	content := "---\n---\n# Body\n"
	fm, err := ParseFrontMatter(content)
	require.NoError(t, err)
	assert.False(t, fm.HasFrontMatter, "empty block should be treated as no front matter")
}
