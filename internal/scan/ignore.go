package scan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// buildIgnoreMatcher returns a function that checks if a path should be ignored
// using gitignore patterns from .mdfmvignore and .gitignore at the scan root.
func buildIgnoreMatcher(root string) func(path string, isDir bool) bool {
	var matchers []gitignore.Matcher
	load := func(name string) {
		p := filepath.Join(root, name)

		b, err := os.ReadFile(p)
		if err == nil {
			ps := parseGitignoreLines(string(b))
			if len(ps) > 0 {
				m := gitignore.NewMatcher(ps)
				matchers = append(matchers, m)
			}
		}
	}
	load(".mdfmvignore")
	load(".gitignore")

	return func(path string, isDir bool) bool {
		// Always ignore .git
		base := filepath.Base(path)
		if base == ".git" {
			return true
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}

		rel = filepath.ToSlash(rel)
		// aggregate matchers: if any says ignore, ignore unless negated later
		// go-git matcher returns true if path is matched by ignore patterns
		for _, m := range matchers {
			if m.Match(strings.Split(rel, "/"), isDir) {
				return true
			}
		}

		return false
	}
}

// parseGitignoreLines parses gitignore content into patterns.
func parseGitignoreLines(s string) []gitignore.Pattern {
	s = strings.TrimPrefix(s, "\uFEFF")     // strip UTF-8 BOM if present
	s = strings.ReplaceAll(s, "\r\n", "\n") // normalize Windows CRLF to LF
	s = strings.ReplaceAll(s, "\r", "\n")   // normalize old Mac CR to LF
	lines := strings.Split(s, "\n")

	ps := make([]gitignore.Pattern, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ps = append(ps, gitignore.ParsePattern(line, nil))
	}

	return ps
}
