package scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dariuszporowski/mdfmv/internal/utils"
)

// loadSchema loads schema bytes from local files or http(s) URLs.
func loadSchema(ctx context.Context, pathOrURL string) ([]byte, error) {
	if pathOrURL == "" {
		return nil, errors.New("empty schema path")
	}

	if utils.IsURL(pathOrURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pathOrURL, nil)
		if err != nil {
			return nil, err
		}

		// Use a client with reasonable defaults; could be made configurable later.
		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch schema: %s", resp.Status)
		}

		return io.ReadAll(resp.Body)
	}

	return os.ReadFile(pathOrURL)
}

// FileDir is the directory of the markdown file (or provided baseDir for stdin/single content usage)
// used to resolve relative $schema references.
func loadSchemaBytes(ctx context.Context, schemaPath string, fm *FrontMatter, fileDir string) ([]byte, error) {
	switch {
	case schemaPath != "":
		fm.SchemaSource = string(SchemaTypeFlag)

		return loadSchema(ctx, schemaPath)
	case fm.SchemaSource == string(SchemaTypeKey) && fm.SchemaRef != "":
		ref := fm.SchemaRef
		if !utils.IsURL(ref) && !filepath.IsAbs(ref) {
			// Resolve relative to provided directory (already absolute or cwd-resolved by caller)
			ref = filepath.Join(fileDir, ref)
		}

		return loadSchema(ctx, ref)
	case fm.SchemaSource == string(SchemaTypeInline) && fm.InlineSchema != nil:
		// Marshal inline schema map (unique per file, not cached at bytes level)
		return json.Marshal(fm.InlineSchema)
	default:
		return nil, nil
	}
}
