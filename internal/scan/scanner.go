package scan

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
)

// Scan walks files under root according to options, parses front matter, and validates.
func Scan(ctx context.Context, opts Options) ([]Result, error) {
	var (
		results   []Result
		filePaths []string
	)

	// First pass: collect files to process
	filePaths, walkResults, err := collectFilesToProcess(opts)
	if err != nil {
		return walkResults, err
	}

	// Add any walk errors to results
	results = append(results, walkResults...)

	// Process files in parallel using worker pool
	processFilesInParallel(ctx, filePaths, opts, &results)

	return results, nil
}

// collectFilesToProcess walks the directory tree and collects files to process.
func collectFilesToProcess(opts Options) ([]string, []Result, error) {
	var (
		filePaths []string
		results   []Result
		ignored   = buildIgnoreMatcher(opts.Root)
		// Build a set of extensions. We assume CLI already normalized (leading dot, lowercase, deduped),
		// but still lowercase defensively. Avoid re-normalizing logic here to prevent duplication.
		extSet = func(exts []string) map[string]struct{} {
			if len(exts) == 0 {
				return nil
			}

			m := make(map[string]struct{}, len(exts))
			for _, e := range exts {
				m[strings.ToLower(e)] = struct{}{}
			}

			return m
		}(opts.Extensions)
		matchExt = func(name string) bool {
			if len(extSet) == 0 {
				return true
			}

			ext := strings.ToLower(filepath.Ext(name))
			_, ok := extSet[ext]

			return ok
		}
	)

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			results = append(results, Result{
				Path:   path,
				Valid:  false,
				Issues: []string{err.Error()},
			})

			//nolint:nilerr // Continue walking and collect all errors in results
			return nil
		}

		if d.IsDir() {
			if d.Name() == ".git" || ignored(path, true) {
				if path != opts.Root {
					return fs.SkipDir
				}
			}

			if !opts.Recursive && path != opts.Root {
				return fs.SkipDir
			}

			return nil
		}

		if matchExt(d.Name()) && !ignored(path, false) {
			filePaths = append(filePaths, path)
		}

		return nil
	}

	err := filepath.WalkDir(opts.Root, walkFn)

	return filePaths, results, err
}
