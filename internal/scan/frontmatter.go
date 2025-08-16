package scan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dariuszporowski/mdfmv/internal/utils"
)

// ErrUnclosedFrontMatter is returned when an opening front matter fence has no closing fence.
var ErrUnclosedFrontMatter = errors.New("front matter fence not closed with ---")

// FrontMatter contains parsed front matter data.
// We keep it generic as map[string]any so schemas can be flexible.
// Content is the remaining markdown without the front matter fence.
// SchemaSource indicates from where schema came (flag or inline or key reference)
// SchemaRef holds the path/URL when schema is referenced via key
// InlineSchema holds parsed inline schema object if present
// If neither is set and no global schema, no validation occurs.
type FrontMatter struct {
	Data           map[string]any
	Content        string
	HasFrontMatter bool
	SchemaSource   string         // "flag", "inline", "key", or "none"
	SchemaRef      string         // when SchemaSource == "key"
	InlineSchema   map[string]any // when SchemaSource == "inline"
}

// DetermineSchemaType analyzes the front matter data to determine the schema type.
func (fm *FrontMatter) DetermineSchemaType() SchemaType {
	if fm.Data == nil {
		return SchemaTypeNone
	}

	if v, ok := fm.Data["$schema"]; ok {
		switch v.(type) {
		case string:
			return SchemaTypeKey
		case map[string]any:
			return SchemaTypeInline
		}
	}

	return SchemaTypeNone
}

// ParseFrontMatter extracts YAML front matter from markdown content. For now,
// we only support YAML front matter (between --- fences at the beginning of the file),
// as it's the most common; JSON can be added later.
func ParseFrontMatter(content string) (*FrontMatter, error) {
	content = strings.TrimLeft(content, "\ufeff") // strip BOM if present
	// support both LF and CRLF after opening fence
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		// no YAML front matter
		return &FrontMatter{Data: map[string]any{}, Content: content, HasFrontMatter: false, SchemaSource: string(SchemaTypeNone)}, nil
	}
	// find closing fence for both LF and CRLF
	search := content[4:]
	// Handle empty front matter block: second fence appears immediately
	if strings.HasPrefix(search, "---\n") { // LF
		rest := search[len("---\n"):]

		return &FrontMatter{Data: map[string]any{}, Content: rest, HasFrontMatter: false, SchemaSource: string(SchemaTypeNone)}, nil
	}

	if strings.HasPrefix(search, "---\r\n") { // CRLF variant
		rest := search[len("---\r\n"):]

		return &FrontMatter{Data: map[string]any{}, Content: rest, HasFrontMatter: false, SchemaSource: string(SchemaTypeNone)}, nil
	}

	endIdx := strings.Index(search, "\n---\n")
	fenceLen := 5

	if endIdx == -1 {
		endIdx = strings.Index(search, "\r\n---\r\n")
		fenceLen = 8
	}

	if endIdx == -1 {
		return nil, ErrUnclosedFrontMatter
	}

	end := 4 + endIdx
	head := content[4:end]
	rest := content[end+fenceLen:]

	// Parse YAML front matter into a map
	data, err := utils.UnmarshalYAMLToMap(head)
	if err != nil {
		return nil, fmt.Errorf("parse front matter yaml: %w", err)
	}

	fm := &FrontMatter{Data: data, Content: rest, HasFrontMatter: true}
	// If the front matter parses to an empty map, treat it as if no front matter existed.
	if len(data) == 0 {
		fm.HasFrontMatter = false
		fm.SchemaSource = string(SchemaTypeNone)

		return fm, nil
	}

	// Determine schema type and set appropriate fields
	schemaType := fm.DetermineSchemaType()
	fm.SchemaSource = string(schemaType)

	switch schemaType {
	case SchemaTypeKey:
		if v, ok := data["$schema"]; ok {
			if s, ok := v.(string); ok {
				fm.SchemaRef = s
			}
		}
	case SchemaTypeInline:
		if v, ok := data["$schema"]; ok {
			if schema, ok := v.(map[string]any); ok {
				fm.InlineSchema = schema
			}
		}
	default:
		// SchemaTypeNone or SchemaTypeFlag - no additional setup needed
	}

	return fm, nil
}
