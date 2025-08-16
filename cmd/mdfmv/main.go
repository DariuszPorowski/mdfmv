package mdfmv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/dariuszporowski/mdfmv/internal/scan"
)

// Filtering handled inside formatResults for single-pass counting.
// If quiet mode is enabled, results will be filtered in formatResults.

// cliFlags holds all CLI flag values.
type cliFlags struct {
	schemaPath string
	recursive  bool
	exts       []string
	quiet      bool
	outputFmt  string
	force      bool
	baseDir    string
	workers    int
}

//nolint:gochecknoglobals // cobra flags are bound to a global command instance
var cfg = &cliFlags{}

// formatOutcome represents the structured result of formatting for testability.
type formatOutcome struct {
	Output string
	Failed int
	Warned int
}

// formatResults builds output string for the chosen format and computes failed/warned counts.
// It applies quiet filtering logic internally (filtering out valid entries when quiet is set).
func formatResults(results []scan.Result, c *cliFlags) (formatOutcome, error) {
	fmtOpt := strings.ToLower(c.outputFmt)

	// Prepare filtered slice only if quiet; otherwise reuse original slice.
	filtered := results
	if c.quiet {
		filtered = make([]scan.Result, 0, len(results)) // pre-size capacity for potential worst-case (all fail)
	}

	failed, warned := 0, 0

	for _, r := range results { // single pass: count + optionally build quiet filtered list
		if !r.Valid {
			failed++

			if c.quiet { // only collect failed results in quiet mode
				filtered = append(filtered, r)
			}
		} else if len(r.Issues) > 0 { // warnings only on valid files
			warned++
		}
	}
	var sb strings.Builder

	switch fmtOpt {
	case "json":
		items := buildItems(filtered)
		enc := json.NewEncoder(&sb)
		enc.SetIndent("", "  ")

		err := enc.Encode(items)
		if err != nil {
			return formatOutcome{}, err
		}
	case "yaml", "yml":
		items := buildItems(filtered)

		b, err := yaml.Marshal(items)
		if err != nil {
			return formatOutcome{}, err
		}

		_, err = sb.Write(b)
		if err != nil {
			return formatOutcome{}, err
		}
	case "", "tsv":
		for _, r := range filtered {
			printTSVTo(&sb, r)
		}
	default:
		return formatOutcome{}, fmt.Errorf("unknown output format: %s (valid: json, tsv, yaml)", fmtOpt)
	}

	return formatOutcome{Output: sb.String(), Failed: failed, Warned: warned}, nil
}

// printTSVTo writes a single result in TSV form into a builder (testable helper).
func printTSVTo(w *strings.Builder, r scan.Result) {
	status := "OK"
	if !r.Valid {
		status = "FAIL"
	}

	if len(r.Issues) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\t\n", status, r.Path)

		return
	}

	_, _ = fmt.Fprintf(w, "%s\t%s\t\n", status, r.Path)

	for _, iss := range r.Issues {
		_, _ = fmt.Fprintf(w, "\t\t%s\n", iss)
	}
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// rootCmd represents the base command when called without any subcommands.
//
//nolint:gochecknoglobals // cobra requires a package-level command variable
var rootCmd = &cobra.Command{
	Use:           "mdfmv [path]",
	Short:         "Markdown Front Matter Validator",
	Long:          "mdfmv scans Markdown files and validates their front matter against a JSON Schema provided via flag or embedded in the front matter. The path argument can be either a single file or a directory.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       build.Version,
	Args:          cobra.MaximumNArgs(1),
	PreRunE:       func(_ *cobra.Command, _ []string) error { return validateAndNormalize(cfg) },
	RunE: func(_ *cobra.Command, args []string) error {
		// Context that cancels on Ctrl+C / SIGTERM to propagate to workers.
		base := context.Background()
		ctx, stop := signal.NotifyContext(base, os.Interrupt, syscall.SIGTERM)
		defer stop()

		var results []scan.Result

		// Case 1: no args provided. Allow only when input is piped via stdin.
		if len(args) == 0 {
			st, _ := os.Stdin.Stat()
			if (st.Mode() & os.ModeCharDevice) != 0 {
				return errors.New("path is required when not reading from stdin")
			}
			b, rerr := io.ReadAll(os.Stdin)
			if rerr != nil {
				return rerr
			}
			r := scan.ValidateContent(ctx, string(b), cfg.baseDir, cfg.schemaPath, cfg.force)
			results = append(results, r)
		} else {
			// Case 2: path provided, check if it's a file or directory
			root := args[0]
			root, err := filepath.Abs(root)
			if err != nil {
				return err
			}

			// Check if the path is a file or directory
			info, err := os.Stat(root)
			if err != nil {
				return err
			}

			if info.IsDir() {
				// Directory: use full scan
				opts := scan.Options{
					Root:       root,
					Recursive:  cfg.recursive,
					Extensions: cfg.exts,
					SchemaPath: cfg.schemaPath,
					Force:      cfg.force,
					Workers:    cfg.workers,
				}
				results, err = scan.Scan(ctx, opts)
				if err != nil {
					return err
				}
			} else {
				// Single file: read and validate directly
				content, err := os.ReadFile(root)
				if err != nil {
					return err
				}

				// Use the file's directory as baseDir for relative schema resolution
				baseDir := filepath.Dir(root)
				r := scan.ValidateContent(ctx, string(content), baseDir, cfg.schemaPath, cfg.force)
				r.Path = root // Set the actual file path instead of "-"
				results = append(results, r)
			}
		}

		// If cancellation already requested, abort before heavy work.
		if ctx.Err() != nil {
			return &exitError{code: 130, msg: "operation cancelled"}
		}

		// Determine output format
		outcome, err := formatResults(results, cfg)
		if err != nil {
			return err
		}
		// write generated output
		_, _ = io.WriteString(os.Stdout, outcome.Output)

		failed := outcome.Failed
		warned := outcome.Warned
		if ctx.Err() != nil { // cancelled during output generation / counting
			return &exitError{code: 130, msg: "operation cancelled"}
		}
		if failed > 0 {
			return &exitError{code: 1, msg: fmt.Sprintf("%d file(s) failed validation", failed)}
		}
		if warned > 0 {
			// Only warnings present
			return &exitError{code: 2, msg: ""}
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() int {
	setupFlags()

	rootCmd.SetVersionTemplate(build.String("mdfmv"))

	err := rootCmd.Execute()
	if err == nil {
		return 0
	}

	var ee *exitError
	if errors.As(err, &ee) {
		if ee.msg != "" {
			_, _ = fmt.Fprintln(os.Stderr, ee.msg)
		}

		return ee.code
	}

	// Unexpected error: print and use code 1
	_, _ = fmt.Fprintln(os.Stderr, err)

	return 1
}

// setupFlags configures persistent flags for the root command.
func setupFlags() {
	rootCmd.PersistentFlags().StringVarP(&cfg.schemaPath, "schema", "s", "", "Path or URL to a JSON Schema file to validate front matter against (applies to all files)")
	rootCmd.PersistentFlags().BoolVarP(&cfg.recursive, "recursive", "r", false, "Scan directories recursively (default is false)")
	rootCmd.PersistentFlags().StringSliceVarP(&cfg.exts, "ext", "e", []string{".md", ".markdown", ".mdown", ".mdx"}, "File extensions to include")
	rootCmd.PersistentFlags().BoolVarP(&cfg.quiet, "quiet", "q", false, "Quiet mode; only print errors")
	rootCmd.PersistentFlags().StringVarP(&cfg.outputFmt, "output", "o", "tsv", "Output format: json, tsv, yaml")
	rootCmd.PersistentFlags().BoolVar(&cfg.force, "force", false, "Fail files that do not contain front matter")
	rootCmd.PersistentFlags().StringVar(&cfg.baseDir, "base-dir", "", "Base directory for resolving relative $schema when reading from stdin (defaults to current directory)")
	rootCmd.PersistentFlags().IntVar(&cfg.workers, "workers", 0, "Number of concurrent workers (0 = auto)")
}

// earlyValidateAndNormalize performs upfront validation on flags to fail fast
// and normalizes extensions list (leading dots, de-dup, stable order).
func validateAndNormalize(c *cliFlags) error {
	// Normalize output format now (store lower)
	c.outputFmt = strings.ToLower(strings.TrimSpace(c.outputFmt))
	switch c.outputFmt {
	case "", "tsv", "json", "yaml", "yml":
		// ok (treat yml same as yaml later)
	default:
		return fmt.Errorf("invalid output format %q (allowed: json, tsv, yaml)", c.outputFmt)
	}

	// If schema path given and not URL, verify it exists (file only)
	if c.schemaPath != "" && !isURLLike(c.schemaPath) {
		_, err := os.Stat(c.schemaPath)
		if err != nil {
			return fmt.Errorf("schema path: %w", err)
		}
	}

	// Normalize extensions
	seen := make(map[string]struct{}, len(c.exts))

	norm := make([]string, 0, len(c.exts))
	for _, e := range c.exts {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}

		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}

		if _, exists := seen[e]; exists {
			continue
		}

		seen[e] = struct{}{}
		norm = append(norm, strings.ToLower(e))
	}

	sort.Strings(norm)

	if len(norm) == 0 { // ensure we always have at least one extension
		norm = []string{".md"}
	}

	c.exts = norm

	// If reading stdin later and baseDir empty, set to cwd early
	if c.baseDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			c.baseDir = cwd
		}
	}

	return nil
}

// isURLLike performs a light heuristic to detect URL schema paths.
func isURLLike(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// outItem is the serialized representation for JSON/YAML outputs.
type outItem struct {
	Path   string   `json:"path"   yaml:"path"`
	Valid  bool     `json:"valid"  yaml:"valid"`
	Issues []string `json:"issues" yaml:"issues"`
}

func buildItems(rs []scan.Result) []outItem {
	items := make([]outItem, len(rs))
	for i, r := range rs {
		issues := r.Issues
		if issues == nil {
			issues = []string{}
		}

		items[i] = outItem{Path: r.Path, Valid: r.Valid, Issues: issues}
	}

	return items
}
