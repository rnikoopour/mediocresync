package sync

import (
	"regexp"
	"strings"
)

// applyFilters reports whether a remote file path should be included in a sync
// run given the job's include and exclude glob patterns.
//
// Rules:
//  1. If include is non-empty the file must match at least one pattern.
//  2. If exclude is non-empty the file is dropped if it matches any pattern.
//
// Patterns are matched against the full remote path. Glob syntax:
//   - "*"  matches anything, including "/" (i.e. crosses directory boundaries)
//   - "?"  matches any single character except "/"
//
// Examples:
//   - "*.txt"    matches any .txt file at any depth
//   - "*/foo/*"  matches any file inside a directory named foo at any depth
//   - "backup_?" matches backup_a, backup_b, etc. at any depth
func applyFilters(remotePath string, include, exclude []string) bool {
	if len(include) == 0 && len(exclude) == 0 {
		return true
	}

	matchAny := func(patterns []string) bool {
		for _, pattern := range patterns {
			if globMatch(pattern, remotePath) {
				return true
			}
		}
		return false
	}

	if len(include) > 0 && !matchAny(include) {
		return false
	}
	if len(exclude) > 0 && matchAny(exclude) {
		return false
	}
	return true
}

// globMatch converts a glob pattern to a regexp and tests it against s.
// "*" matches any sequence of characters including "/".
// "?" matches any single character except "/".
func globMatch(pattern, s string) bool {
	re := globToRegexp(pattern)
	return re.MatchString(s)
}

// globToRegexp compiles a glob pattern into a regexp.
// The result is cached in a sync.Map would be an optimisation, but for the
// small number of patterns expected here a fresh compile per call is fine.
func globToRegexp(pattern string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteByte('^')
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString("[^/]")
		// Escape all regexp metacharacters.
		case '.', '+', '^', '$', '{', '}', '[', ']', '(', ')', '|', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteByte('$')
	// Pattern syntax is controlled by us so Compile should never fail.
	re, err := regexp.Compile(sb.String())
	if err != nil {
		// Return a regexp that matches nothing.
		return regexp.MustCompile(`^\z`)
	}
	return re
}
