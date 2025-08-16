package scan

// Options controls the behavior of the scanner.
type Options struct {
	Root       string
	Recursive  bool
	Extensions []string
	SchemaPath string // optional global schema path/URL
	Force      bool   // fail files without front matter when true
	// Workers allows overriding the number of concurrent workers used during scanning.
	// When 0 or negative, a default based on CPU count is used.
	Workers int
}

// Result represents a validation result for a single file.
type Result struct {
	Path   string
	Valid  bool
	Issues []string
}

// Standardized warning messages (keep stable for tests/CLI filtering).
const (
	WarnNoFrontMatter = "no front matter"
	WarnNoSchema      = "no schema provided"
	WarnMissingFront  = "missing front matter" // failure when --force
)

// SchemaType represents the source of schema configuration.
type SchemaType string

const (
	SchemaTypeFlag   SchemaType = "flag"
	SchemaTypeInline SchemaType = "inline"
	SchemaTypeKey    SchemaType = "key"
	SchemaTypeNone   SchemaType = "none"
)
