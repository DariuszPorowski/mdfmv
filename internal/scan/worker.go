package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/dariuszporowski/mdfmv/internal/utils"
)

// scanWorker encapsulates the worker logic for parallel file processing.
type scanWorker struct {
	opts      Options
	sc        *schemaCache
	resultsCh chan<- Result
}

// processFile handles validation of a single file.
func (w *scanWorker) processFile(ctx context.Context, path string) {
	content, ok := w.readFile(path)
	if !ok {
		return
	}

	fm, ok := w.parseFrontMatter(path, content)
	if !ok {
		return
	}

	if !fm.HasFrontMatter {
		w.handleNoFrontMatter(path)

		return
	}

	schemaBytes, ok := w.fetchSchema(ctx, path, fm)
	if !ok {
		return
	}

	if schemaBytes == nil {
		w.addResult(Result{Path: path, Valid: true, Issues: []string{WarnNoSchema}})

		return
	}

	compiled, ok := w.obtainCompiledSchema(path, schemaBytes, fm)
	if !ok {
		return
	}

	w.validateAndReport(ctx, path, fm, compiled)
}

func (w *scanWorker) readFile(path string) ([]byte, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		w.addResult(Result{Path: path, Valid: false, Issues: []string{err.Error()}})

		return nil, false
	}

	return content, true
}

func (w *scanWorker) parseFrontMatter(path string, content []byte) (*FrontMatter, bool) {
	fm, err := ParseFrontMatter(string(content))
	if err != nil {
		if errors.Is(err, ErrUnclosedFrontMatter) {
			if w.opts.Force {
				w.addResult(Result{Path: path, Valid: false, Issues: []string{err.Error()}})
			} else {
				w.addResult(Result{Path: path, Valid: true, Issues: []string{err.Error()}})
			}
		} else {
			w.addResult(Result{Path: path, Valid: false, Issues: []string{fmt.Sprintf("parse front matter: %v", err)}})
		}

		return nil, false
	}

	return fm, true
}

func (w *scanWorker) handleNoFrontMatter(path string) {
	if w.opts.Force {
		w.addResult(Result{Path: path, Valid: false, Issues: []string{WarnMissingFront}})
	} else {
		w.addResult(Result{Path: path, Valid: true, Issues: []string{WarnNoFrontMatter}})
	}
}

func (w *scanWorker) fetchSchema(ctx context.Context, path string, fm *FrontMatter) ([]byte, bool) {
	schemaBytes, err := loadSchemaBytes(ctx, w.opts.SchemaPath, fm, filepath.Dir(path))
	if err != nil {
		w.addResult(Result{Path: path, Valid: false, Issues: []string{fmt.Sprintf("load schema: %v", err)}})

		return nil, false
	}

	return schemaBytes, true
}

func (w *scanWorker) deriveCacheKey(path string, fm *FrontMatter) string {
	switch {
	case w.opts.SchemaPath != "":
		return computeBaseURIFrom(w.opts.SchemaPath)
	case fm.SchemaRef != "":
		ref := fm.SchemaRef
		if !utils.IsURL(ref) && !filepath.IsAbs(ref) {
			ref = filepath.Join(filepath.Dir(path), ref)
		}

		return computeBaseURIFrom(ref)
	case fm.InlineSchema != nil:
		return inlineSchemaKey(fm.InlineSchema)
	default:
		return ""
	}
}

func (w *scanWorker) obtainCompiledSchema(path string, schemaBytes []byte, fm *FrontMatter) (*jsonschema.Schema, bool) {
	cacheKey := w.deriveCacheKey(path, fm)

	// Cached
	if cacheKey != "" {
		if c, ok := w.sc.get(cacheKey); ok {
			return c, true
		}

		cs, err := compileSchemaWithBase(schemaBytes, cacheKey)
		if err != nil {
			w.addResult(Result{Path: path, Valid: false, Issues: []string{fmt.Sprintf("compile schema: %v", err)}})

			return nil, false
		}

		w.sc.set(cacheKey, cs)

		return cs, true
	}

	// Inline / no key
	cs, err := compileSchemaWithBase(schemaBytes, computeInlineBaseURI())
	if err != nil {
		w.addResult(Result{Path: path, Valid: false, Issues: []string{fmt.Sprintf("compile schema: %v", err)}})

		return nil, false
	}

	return cs, true
}

func (w *scanWorker) validateAndReport(ctx context.Context, path string, fm *FrontMatter, compiled *jsonschema.Schema) {
	validationErrors, err := fm.validateWithSchema(ctx, nil, compiled)
	if err != nil {
		w.addResult(Result{Path: path, Valid: false, Issues: []string{err.Error()}})

		return
	}

	w.addResult(createValidationResult(path, nil, validationErrors))
}

// addResult safely adds a result to the results slice.
func (w *scanWorker) addResult(result Result) { w.resultsCh <- result }

// processFilesInParallel processes files using a worker pool for concurrency.
func processFilesInParallel(ctx context.Context, filePaths []string, opts Options, results *[]Result) {
	sc := &schemaCache{}

	// Pre-size results slice to avoid repeated reallocations during aggregation (only if empty slice provided).
	if results != nil && len(*results) == 0 {
		*results = make([]Result, 0, len(filePaths))
	}

	workerCount := opts.Workers
	if workerCount <= 0 {
		workerCount = max(runtime.NumCPU(), 1)
	}

	jobs := make(chan string, len(filePaths))
	resultsCh := make(chan Result, workerCount*2)
	var wg sync.WaitGroup
	var aggWg sync.WaitGroup

	// Aggregator: single writer to the results slice
	aggWg.Go(func() {
		for r := range resultsCh {
			*results = append(*results, r)
		}
	})

	// Start workers
	for range workerCount {
		wg.Go(func() {
			worker := &scanWorker{opts: opts, sc: sc, resultsCh: resultsCh}

			for path := range jobs {
				if ctx.Err() != nil { // stop early if cancelled
					return
				}

				worker.processFile(ctx, path)
			}
		})
	}

	// Send jobs
	for _, path := range filePaths {
		if ctx.Err() != nil { // stop enqueueing if cancelled
			break
		}

		jobs <- path
	}

	close(jobs)

	// Wait for completion
	wg.Wait()
	// Close results and wait for aggregator to finish
	close(resultsCh)
	aggWg.Wait()
}
