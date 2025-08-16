package scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/dariuszporowski/mdfmv/internal/utils"
)

// validateWithJSONSchema validates data against a compiled JSON schema.
func validateWithJSONSchema(_ context.Context, compiled *jsonschema.Schema, data map[string]any) ([]string, error) {
	verr := compiled.Validate(data)
	if verr == nil {
		return nil, nil
	}

	var ve *jsonschema.ValidationError
	if errors.As(verr, &ve) {
		return flattenValidationErrors(ve), nil
	}

	return []string{verr.Error()}, nil
}

// flattenValidationErrors turns a tree of ValidationError into human-readable messages.
// Only reports leaf errors (those without causes) to avoid duplication.
func flattenValidationErrors(ve *jsonschema.ValidationError) []string {
	// If this error has no causes, it's a leaf - report it
	if len(ve.Causes) == 0 {
		return []string{fmt.Sprintf("'%s': %s", ve.InstanceLocation, ve.Error())}
	}

	// Otherwise, recurse into causes and collect their leaf errors
	var msgs []string
	for _, c := range ve.Causes {
		msgs = append(msgs, flattenValidationErrors(c)...)
	}

	return msgs
}

// compileSchema compiles schemaBytes with the given base URI to support relative $ref.
func compileSchemaWithBase(schemaBytes []byte, baseURI string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()

	var schemaVal any

	err := json.Unmarshal(schemaBytes, &schemaVal)
	if err != nil {
		// fallback to YAML for schemas written in YAML
		m, yerr := utils.UnmarshalYAMLToMap(string(schemaBytes))
		if yerr != nil {
			return nil, fmt.Errorf("parse schema: %w", err)
		}

		schemaVal = m
	}

	// Use the baseURI as the resource name so relative $ref resolve properly.
	err = c.AddResource(baseURI, schemaVal)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	s, err := c.Compile(baseURI)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	return s, nil
}

// computeBaseURI determines a base URI for schema refs.
const inlineBaseURI = "inline://schema"

func computeInlineBaseURI() string { return inlineBaseURI }

func computeBaseURIFrom(pathOrURL string) string {
	if pathOrURL == "" {
		return inlineBaseURI
	}

	if utils.IsURL(pathOrURL) {
		return pathOrURL
	}

	abs, err := filepath.Abs(pathOrURL)
	if err != nil {
		return pathOrURL
	}

	p := filepath.ToSlash(abs)
	// On Windows, ensure leading slash so url.URL renders file:///C:/...
	if runtime.GOOS == "windows" {
		if len(p) >= 2 && p[1] == ':' { // drive letter path like C:/...
			p = "/" + p
		}
	}

	u := &url.URL{Scheme: "file", Path: p}

	return u.String()
}

// validateWithSchema performs the actual validation using the provided schema bytes and base.
func (fm *FrontMatter) validateWithSchema(ctx context.Context, schemaBytes []byte, compiled *jsonschema.Schema) ([]string, error) {
	// If compiled is provided, use it; else compile with inline base.
	var s *jsonschema.Schema
	if compiled != nil {
		s = compiled
	} else {
		cs, err := compileSchemaWithBase(schemaBytes, computeInlineBaseURI())
		if err != nil {
			return nil, fmt.Errorf("validate error: %w", err)
		}

		s = cs
	}

	return validateWithJSONSchema(ctx, s, fm.Data)
}

// createValidationResult creates a Result based on validation outcomes.
func createValidationResult(path string, warnings, validationErrs []string) Result {
	valid := len(validationErrs) == 0
	issues := make([]string, 0, len(warnings)+len(validationErrs))
	issues = append(issues, warnings...)
	issues = append(issues, validationErrs...)

	return Result{
		Path:   path,
		Valid:  valid,
		Issues: issues,
	}
}

// ValidateContent validates markdown content directly without file system operations.
// SchemaPath, when provided, takes precedence over any $schema in the front matter.
// When no schema is available: if force is true and there's no front matter, the result fails;
// otherwise the result is OK with a "no schema provided" warning.
//
//nolint:revive // 'force' is a deliberate behavior toggle for CLI UX
func ValidateContent(ctx context.Context, content, baseDir, schemaPath string, force bool) Result {
	fm, err := ParseFrontMatter(content)
	if err != nil {
		if errors.Is(err, ErrUnclosedFrontMatter) {
			if force {
				return Result{Path: "-", Valid: false, Issues: []string{err.Error()}}
			}

			return Result{Path: "-", Valid: true, Issues: []string{err.Error()}}
		}

		return Result{Path: "-", Valid: false, Issues: []string{fmt.Sprintf("parse front matter: %v", err)}}
	}

	// If no front matter: fail only when force, otherwise OK with warning (and skip schema validation entirely)
	if !fm.HasFrontMatter {
		if force {
			return Result{Path: "-", Valid: false, Issues: []string{WarnMissingFront}}
		}

		return Result{Path: "-", Valid: true, Issues: []string{WarnNoFrontMatter}}
	}

	// Ensure baseDir is set (for relative resolution) if empty.
	if baseDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			baseDir = cwd
		}
	}

	// Load schema (only when front matter exists) via unified helper
	schemaBytes, err := loadSchemaBytes(ctx, schemaPath, fm, baseDir)
	if err != nil {
		return Result{Path: "-", Valid: false, Issues: []string{fmt.Sprintf("load schema: %v", err)}}
	}

	if schemaBytes == nil { // front matter present but no schema
		return Result{Path: "-", Valid: true, Issues: []string{WarnNoSchema}}
	}

	baseForCompile := schemaPath
	if baseForCompile == "" && fm.SchemaRef != "" {
		baseForCompile = fm.SchemaRef
	}

	compiled, cerr := compileSchemaWithBase(schemaBytes, computeBaseURIFrom(baseForCompile))
	if cerr != nil {
		return Result{Path: "-", Valid: false, Issues: []string{cerr.Error()}}
	}

	validationErrs, err := fm.validateWithSchema(ctx, nil, compiled)
	if err != nil {
		return Result{Path: "-", Valid: false, Issues: []string{err.Error()}}
	}

	return createValidationResult("-", nil, validationErrs)
}
