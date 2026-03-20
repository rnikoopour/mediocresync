package sync

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// applyFilters reports whether a remote file at filePath should be included.
//
// Include groups are ANDed: a non-empty includePathFilters requires the file
// to be under at least one listed subdirectory; a non-empty includeNameFilters
// requires the basename to match at least one glob pattern.
//
// Exclude groups are ORed: the file is excluded if it is under any
// excludePathFilter subdirectory OR if its basename matches any excludeNameFilter.
func applyFilters(
	filePath, jobRemotePath string,
	includePathFilters, includeNameFilters,
	excludePathFilters, excludeNameFilters []string,
) bool {
	base := strings.TrimSuffix(jobRemotePath, "/")

	if len(includePathFilters) > 0 && !matchesAnyPath(filePath, base, includePathFilters) {
		return false
	}
	if len(includeNameFilters) > 0 && !matchesAnyName(filePath, includeNameFilters) {
		return false
	}
	if matchesAnyPath(filePath, base, excludePathFilters) {
		return false
	}
	if matchesAnyName(filePath, excludeNameFilters) {
		return false
	}
	return true
}

func matchesAnyPath(filePath, base string, subdirs []string) bool {
	for _, subdir := range subdirs {
		subdir = strings.Trim(subdir, "/")
		if isGlobPattern(subdir) {
			if matchesGlobPath(filePath, base, subdir) {
				return true
			}
		} else {
			prefix := base + "/" + subdir
			if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
				return true
			}
		}
	}
	return false
}

// isGlobPattern reports whether s contains any glob metacharacter.
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// matchesGlobPath reports whether filePath is under a directory (relative to
// base) that matches the doublestar glob pattern. Each directory-level prefix
// of the relative path is tested so that, e.g., pattern "**/*alpha*" matches
// a file nested at any depth under a directory whose name contains "alpha".
func matchesGlobPath(filePath, base, pattern string) bool {
	base = strings.TrimSuffix(base, "/")
	prefix := base + "/"
	if !strings.HasPrefix(filePath, prefix) {
		return false
	}
	rel := filePath[len(prefix):]
	parts := strings.Split(rel, "/")
	// Test each directory prefix (not the filename itself).
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if matched, _ := doublestar.Match(pattern, dir); matched {
			return true
		}
	}
	return false
}

func matchesAnyName(filePath string, patterns []string) bool {
	name := path.Base(filePath)
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}
	return false
}
