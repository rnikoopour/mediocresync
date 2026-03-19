package sync

import (
	"path"
	"strings"
)

// applyFilters reports whether a remote file should be included in a sync run.
//
// Supported keywords (same syntax for both include and exclude lists):
//
//	path: <subdir>   Match files under <jobRemotePath>/<subdir>/ at any depth.
//	name: <pattern>  Match files whose base name matches the glob pattern.
//	                 Uses standard single-segment globbing: * matches any
//	                 sequence of non-/ characters, ? matches one non-/ character.
//
// Rules:
//   - If includeFilters is non-empty, the file must match at least one entry.
//   - If excludeFilters is non-empty, the file must not match any entry.
//   - Both conditions must be satisfied for the file to be included.
//   - Unrecognised keywords are silently ignored.
func applyFilters(filePath, jobRemotePath string, includeFilters, excludeFilters []string) bool {
	base := strings.TrimSuffix(jobRemotePath, "/")

	if len(includeFilters) > 0 && !matchesAny(filePath, base, includeFilters) {
		return false
	}
	if len(excludeFilters) > 0 && matchesAny(filePath, base, excludeFilters) {
		return false
	}
	return true
}

// matchesAny reports whether filePath matches at least one filter in the list.
func matchesAny(filePath, base string, filters []string) bool {
	for _, f := range filters {
		f = strings.TrimSpace(f)

		if subdir, ok := parsePathFilter(f); ok {
			prefix := base + "/" + strings.Trim(subdir, "/")
			if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
				return true
			}
			continue
		}

		if pattern, ok := parseNameFilter(f); ok {
			if matched, _ := path.Match(pattern, path.Base(filePath)); matched {
				return true
			}
			continue
		}
	}
	return false
}

// parsePathFilter extracts the subdir from a "path: <subdir>" filter.
func parsePathFilter(filter string) (subdir string, ok bool) {
	rest, found := strings.CutPrefix(filter, "path:")
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// parseNameFilter extracts the glob pattern from a "name: <pattern>" filter.
func parseNameFilter(filter string) (pattern string, ok bool) {
	rest, found := strings.CutPrefix(filter, "name:")
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}
